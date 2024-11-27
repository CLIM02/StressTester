package server

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/WuKongIM/StressTester/pkg/client"
	"golang.org/x/exp/rand"
)

type channelTask struct {
	isRunning             atomic.Bool
	channelCfg            *channelCfg
	s                     *server
	channelIds            []string
	createChannelFinished bool // 是否已经创建频道完成
	channelStatsMap       map[string]*channelStats
	channelStatsMapLock   sync.RWMutex
	stopC                 chan struct{}

	tick int

	subscribers       []string // 订阅者
	onlineSubscribers []string // 在线订阅者

	taskIndex int // 任务下标，来区分频道id
}

func newChannelTask(taskIndex int, channelCfg *channelCfg, s *server) task {
	return &channelTask{
		channelCfg:      channelCfg,
		s:               s,
		channelStatsMap: make(map[string]*channelStats),
		stopC:           make(chan struct{}),
		taskIndex:       taskIndex,
	}
}

func (c *channelTask) taskType() taskType {
	return taskChannel
}

func (c *channelTask) start() {
	c.isRunning.Store(true)
	go c.sendLoop()
	go c.run()
}

func (c *channelTask) stop() {
	c.createChannelFinished = false
	c.isRunning.Store(false)
	close(c.stopC)

}

func (c *channelTask) running() bool {
	return c.isRunning.Load()
}

func (c *channelTask) run() {

	// 生成频道id
	for i := 0; i < c.channelCfg.Count; i++ {
		c.channelIds = append(c.channelIds, c.s.genChannelId(c.taskIndex, i))
	}

	// 分批创建频道
	if c.channelCfg.Count > c.s.opts.CreateChannelPerBatch {
		batchCount := c.channelCfg.Count/c.s.opts.CreateChannelPerBatch + 1
		for i := 0; i < batchCount; i++ {
			if !c.isRunning.Load() {
				break
			}
			startIndex := i * c.s.opts.CreateChannelPerBatch
			endIndex := startIndex + c.s.opts.CreateChannelPerBatch
			if startIndex >= c.channelCfg.Count {
				break
			}
			c.createChannel(startIndex, endIndex)
		}
	} else {
		c.createChannel(0, c.channelCfg.Count)
	}

	c.createChannelFinished = true

}

// 创建频道 (startIndex, endIndex]
func (c *channelTask) createChannel(startIndex, endIndex int) {
	if startIndex >= endIndex {
		return
	}

	if startIndex >= len(c.channelIds) {
		return
	}

	channelIds := c.channelIds[startIndex:endIndex]

	// 创建频道
	c.createChannelWithIds(channelIds)

}

func (c *channelTask) createSubscribers() {
	onlineTask := c.s.getOnlineTask()
	// 生成订阅者
	subscribers := make([]string, 0, c.channelCfg.Subscriber.Count)
	// 如果订阅者在线人数小于等于频道在线人数，则直接使用订阅者
	if c.channelCfg.Subscriber.Online <= onlineTask.cfg.Online {
		subscribers = append(subscribers, onlineTask.uids[:c.channelCfg.Subscriber.Online]...)
	} else {
		subscribers = append(subscribers, onlineTask.uids[:onlineTask.cfg.Online]...)
		log.Printf("warn: user online count less than subscriber online count")
	}

	// 在线订阅者
	c.onlineSubscribers = make([]string, len(subscribers))
	copy(c.onlineSubscribers, subscribers)

	// 填充离线订阅者
	if len(subscribers) < c.channelCfg.Subscriber.Count {
		for i := len(subscribers); i < c.channelCfg.Subscriber.Count; i++ {
			// 让离线订阅者的uid不要在在线用户中
			subscribers = append(subscribers, c.s.genUid(onlineTask.cfg.Online+i+10000))
		}
	}
	c.subscribers = subscribers
}

func (c *channelTask) createChannelWithIds(channelIds []string) {

	if len(c.subscribers) == 0 {
		c.createSubscribers()
	}
	// 创建频道
	for _, channelId := range channelIds {
		if !c.isRunning.Load() {
			break
		}

		err := c.s.api.createChannel(&channelCreateReq{
			channelInfoReq: channelInfoReq{
				ChannelId:   channelId,
				ChannelType: uint8(c.channelCfg.Type),
			},
			Subscribers: c.subscribers,
			Reset:       1,
		})
		if err != nil {
			panic(fmt.Sprintf("create channel error: %s", err))
		}
		c.channelStatsMapLock.Lock()
		c.channelStatsMap[channelId] = &channelStats{}
		c.channelStatsMapLock.Unlock()

	}
}

func (c *channelTask) sendLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.willSendMsg()
		case <-c.stopC:
			return
		}
	}
}

func (c *channelTask) willSendMsg() {

	if !c.createChannelFinished { // 频道创建成功后才能发送消息
		return
	}

	onlineTask := c.s.getOnlineTask()
	if !onlineTask.onlineFinished {
		return
	}

	c.tick++

	msgCount := c.channelCfg.MsgRate / 60 // 每秒发送消息数量
	var tickCount int
	if c.channelCfg.MsgRate%60 > 0 {
		tickCount = 60 / (c.channelCfg.MsgRate % 60) // 多少个tick发送一次消息
	}

	if tickCount > 0 && c.tick >= tickCount {
		c.tick = 0
		msgCount++
	}

	c.sendMsg(msgCount)
}

func (c *channelTask) sendMsg(msgCount int) {

	onlineTask := c.s.getOnlineTask()
	if onlineTask == nil {
		return
	}

	if !onlineTask.onlineFinished {
		return
	}

	// 生成指定大小的随机byte数组
	msg := make([]byte, c.s.opts.MsgByteSize)
	_, _ = rand.Read(msg)

	var err error
	for i := 0; i < msgCount; i++ {
		if !c.isRunning.Load() {
			break
		}
		for _, channelId := range c.channelIds {
			if !c.isRunning.Load() {
				break
			}

			cli := c.randomOnlineSubscriberClient()
			if cli == nil {
				continue
			}
			if !cli.isConnected() {
				continue
			}
			err = cli.send(&client.Channel{
				ChannelID:   channelId,
				ChannelType: uint8(c.channelCfg.Type),
			}, msg)
			if err != nil {
				log.Printf("send msg error: %s", err)
			} else {
				c.channelStatsMapLock.Lock()
				stats, ok := c.channelStatsMap[channelId]
				if ok {
					stats.sendMsgCount++
				}
				c.channelStatsMapLock.Unlock()
			}
		}
	}

}

// 获取一个随机在线订阅者客户端
func (c *channelTask) randomOnlineSubscriberClient() *testClient {
	if len(c.onlineSubscribers) == 0 {
		return nil
	}
	uid := c.onlineSubscribers[rand.Intn(len(c.onlineSubscribers))]

	onlineTask := c.s.getOnlineTask()

	return onlineTask.getUserClient(uid)
}

func (c *channelTask) reCreateChannelIfNeeded() {

	if !c.createChannelFinished {
		return
	}

	// 获取创建失败的频道
	failedChannelIds := make([]string, 0)
	for _, channelId := range c.channelIds {
		c.channelStatsMapLock.RLock()
		_, ok := c.channelStatsMap[channelId]
		c.channelStatsMapLock.RUnlock()
		if !ok {
			failedChannelIds = append(failedChannelIds, channelId)
		}
	}

	if len(failedChannelIds) == 0 {
		return
	}

	// 重新创建
	c.createChannelWithIds(failedChannelIds)

}

// 频道统计
type channelStats struct {
	sendMsgCount int64 // 发送消息数量
}
