package server

import (
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"github.com/WuKongIM/StressTester/pkg/client"
	"github.com/WuKongIM/WuKongIM/pkg/wklog"
	wkproto "github.com/WuKongIM/WuKongIMGoProto"
	"go.uber.org/zap"
	"golang.org/x/exp/rand"
)

type p2pTask struct {
	isRunning atomic.Bool
	cfg       *p2pCfg
	s         *server
	pairs     [][2]string // 聊天对
	wklog.Log
	stopC chan struct{}

	sendMsgCount atomic.Int64 // 总共发送消息数量
}

func newP2pTask(cfg *p2pCfg, s *server) task {
	return &p2pTask{
		cfg:   cfg,
		s:     s,
		Log:   wklog.NewWKLog("p2pTask"),
		stopC: make(chan struct{}),
	}
}

func (p *p2pTask) taskType() taskType {
	return taskP2p
}

func (p *p2pTask) start() {
	p.isRunning.Store(true)
	p.generateData()
	go p.sendLoop()
}

func (p *p2pTask) stop() {
	p.isRunning.Store(false)
	close(p.stopC)
}

func (p *p2pTask) generateData() {
	// 获取聊天对数
	onlineTask := p.getOnlineTask()

	users := onlineTask.uids // 当前有的用户数组

	// 获取指定对数需要的最少用户数量
	minUserCount, err := minUsersForPairs(p.cfg.Count)
	if err != nil {
		p.Error("获取最少用户数失败！", zap.Error(err))
		return
	}

	if len(users) < minUserCount {
		for i := len(users); i < minUserCount; i++ {
			users = append(users, p.s.genUid(i))
		}
	}

	p.pairs, err = generateRandomPairs(users, p.cfg.Count)
	if err != nil {
		p.Error("生成聊天对数失败！", zap.Error(err))
		return
	}
}

func (p *p2pTask) sendLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.willSendMsg()
		case <-p.stopC:
			return
		}
	}
}

func (p *p2pTask) willSendMsg() {
	onlineTask := p.getOnlineTask()
	// 等用户都上线后，再发消息
	if !onlineTask.onlineFinished {
		return
	}

	msgCount := p.cfg.MsgRate / 60 // 每秒发送消息数量
	var probability float64 = 0
	if p.cfg.MsgRate%60 > 0 {
		probability = float64(p.cfg.MsgRate%60) / float64(60)
	}

	p.sendMsg(msgCount, probability)
}

func (p *p2pTask) sendMsg(msgCount int, probability float64) {
	onlineTask := p.getOnlineTask()
	// 生成指定大小的随机byte数组
	msg := make([]byte, p.s.opts.MsgByteSize)
	_, _ = rand.Read(msg)

	randSend := func(pair [2]string) {
		k := rand.Intn(2)
		fromUid := pair[k]
		toUid := pair[0]
		if k == 0 {
			toUid = pair[1]
		}

		fromClient := onlineTask.getUserClient(fromUid)
		if fromClient == nil {
			p.Info("发送者的客户端没有找到 ---> %s", zap.String("fromUid", fromUid))
			return
		}
		if !fromClient.isConnected() {
			return
		}
		p.sendMsgCount.Add(1)
		err := fromClient.send(&client.Channel{
			ChannelID:   toUid,
			ChannelType: wkproto.ChannelTypePerson,
		}, msg)
		if err != nil {
			p.Error("send msg error", zap.Error(err), zap.String("fromUid", fromUid), zap.String("toUid", toUid))
		}
	}

	for i := 0; i < msgCount; i++ {
		if !p.isRunning.Load() {
			break
		}
		for _, pair := range p.pairs {
			randSend(pair)
		}
	}

	if probability > 0 {
		for _, pair := range p.pairs {
			if canSendMessage(probability) {
				randSend(pair)
			}
		}
	}

}

func (p *p2pTask) running() bool {

	return p.isRunning.Load()
}
func generateRandomPairs(users []string, numPairs int) ([][2]string, error) {
	n := len(users)
	maxPairs := n * (n - 1) / 2
	if numPairs > maxPairs {
		return nil, fmt.Errorf("指定的对数数量超出最大可能组合数量")
	}

	// 使用 Fisher-Yates 洗牌算法生成随机对数
	pairs := make([][2]string, 0, numPairs)
	used := make(map[[2]int]bool)

	for len(pairs) < numPairs {
		i := rand.Intn(n)
		j := rand.Intn(n)
		if i != j {
			if i > j {
				i, j = j, i
			}
			pair := [2]int{i, j}
			if !used[pair] {
				used[pair] = true
				pairs = append(pairs, [2]string{users[i], users[j]})
			}
		}
	}

	return pairs, nil
}

// 给定对数，返回最少需要用户数量
func minUsersForPairs(pairs int) (int, error) {
	if pairs < 0 {
		return 0, errors.New("对数数量必须是非负数")
	}

	// 解 n * (n - 1) / 2 >= pairs
	// 使用二次方程：n^2 - n - 2*pairs = 0
	a, b, c := 1.0, -1.0, float64(-2*pairs)
	discriminant := b*b - 4*a*c
	if discriminant < 0 {
		return 0, errors.New("无解")
	}

	// 解方程，取正根并向上取整
	n := (-b + math.Sqrt(discriminant)) / (2 * a)
	return int(math.Ceil(n)), nil
}

func (p *p2pTask) getOnlineTask() *onlineTask {
	task := p.s.getTask(taskOnline)
	if task == nil {
		return nil
	}
	return task.(*onlineTask)
}
