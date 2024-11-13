package server

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/WuKongIM/WuKongIM/pkg/client"
	"golang.org/x/exp/rand"
	"golang.org/x/sync/errgroup"
)

type onlineTask struct {
	cfg               *taskCfg
	s                 *server
	uids              []string
	userClientMap     map[string]*testClient
	userClientMapLock sync.RWMutex
	isRunning         atomic.Bool
	currentOnline     int  // 当前在线用户数
	onlineFinished    bool // 是否已经上线完成
	stopC             chan struct{}
}

func newOnlineTask(s *server) task {
	return &onlineTask{
		cfg:           s.taskCfg,
		s:             s,
		userClientMap: make(map[string]*testClient),
		stopC:         make(chan struct{}),
	}
}

func (t *onlineTask) taskType() taskType {
	return taskOnline
}

func (t *onlineTask) start() {
	t.isRunning.Store(true)
	go t.loop()
	go t.run()
}

func (t *onlineTask) stop() {
	t.isRunning.Store(false)
	close(t.stopC)
}

func (t *onlineTask) running() bool {
	return t.isRunning.Load()
}

func (t *onlineTask) run() {
	// 生成用户uid
	for i := 0; i < t.cfg.Online; i++ {
		t.uids = append(t.uids, t.s.genUid(i))
	}

	// 分配上线用户，如果需要上线用户数量超过1万，则需要分批上线，每批5000个用户
	if t.cfg.Online > t.s.opts.OnlinePerBatch {
		// 分批上线
		// 1. 计算分批次数
		batchCount := t.cfg.Online/t.s.opts.OnlinePerBatch + 1

		// 2. 按批次上线
		for i := 0; i < batchCount; i++ {
			if !t.isRunning.Load() {
				break
			}
			startIndex := i * t.s.opts.OnlinePerBatch
			endIndex := startIndex + t.s.opts.OnlinePerBatch
			if startIndex >= t.cfg.Online {
				break
			}
			t.online(startIndex, endIndex)
		}
	} else {
		// 一次性上线
		t.online(0, t.cfg.Online)
	}

	t.onlineFinished = true

}

// 上线用户 (startIndex, endIndex]
func (t *onlineTask) online(startIndex, endIndex int) {
	if startIndex >= endIndex {
		return
	}

	if startIndex >= len(t.uids) {
		return
	}

	// 获取用户tcp地址
	onlineUids := t.uids[startIndex:endIndex]
	userTcpAddrMap, err := t.s.api.route(onlineUids)
	if err != nil {
		log.Printf("get user tcp addr error: %s", err)
		return
	}

	// 连接im
	timeoutCtx, cancel := context.WithTimeout(t.s.serverCtx, 2*time.Minute)
	defer cancel()
	g, _ := errgroup.WithContext(timeoutCtx)
	g.SetLimit(20)

	for _, uid := range onlineUids {
		uid := uid
		g.Go(func() error {
			if !t.isRunning.Load() {
				return nil
			}
			t.userClientMapLock.Lock()
			tcpAddr := userTcpAddrMap[uid]
			t.userClientMapLock.Unlock()
			cli := client.New(tcpAddr, client.WithUID(uid), client.WithAutoReconn(false))

			testCli := newTestClient(cli)
			err := testCli.connect()
			if err != nil {
				log.Printf("connect error: %s", err)
				return nil
			}
			t.userClientMapLock.Lock()
			t.userClientMap[uid] = testCli
			t.userClientMapLock.Unlock()
			return nil
		})
	}
	_ = g.Wait()
}

func (t *onlineTask) loop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.countOnline()
			if !t.isRunning.Load() {
				return
			}
			t.reconnectIfNeed()
		case <-t.stopC:
			return
		}
	}
}

func (t *onlineTask) reconnectIfNeed() {
	if !t.onlineFinished {
		return
	}

	needCreateUids := make([]string, 0)
	for _, uid := range t.uids {
		t.userClientMapLock.RLock()
		cli := t.userClientMap[uid]
		t.userClientMapLock.RUnlock()
		if cli != nil {
			continue
		}
		needCreateUids = append(needCreateUids, uid)
	}

	if len(needCreateUids) > 0 {
		var err error
		tpcAddrMap, err := t.s.api.route(needCreateUids)
		if err != nil {
			log.Printf("get user tcp addr error: %s", err)
			return
		}

		for _, uid := range needCreateUids {

			tcpAddr := tpcAddrMap[uid]
			cli := client.New(tcpAddr, client.WithUID(uid), client.WithAutoReconn(false))
			testCli := newTestClient(cli)
			err := testCli.connect()
			if err != nil {
				log.Printf("reconnect error: %s", err)
				continue
			}
			t.userClientMapLock.Lock()
			t.userClientMap[uid] = testCli
			t.userClientMapLock.Unlock()
		}
	}

	for _, uid := range t.uids {
		t.userClientMapLock.RLock()
		cli := t.userClientMap[uid]
		t.userClientMapLock.RUnlock()

		if cli == nil {
			continue
		}
		if cli.isConnected() {
			continue
		}
		err := cli.connect()
		if err != nil {
			log.Printf("reconnect error: %s", err)
			continue
		}
	}
}

// 统计在线用户数
func (t *onlineTask) countOnline() {

	ct := 0

	for _, uid := range t.uids {
		t.userClientMapLock.RLock()
		cli := t.userClientMap[uid]
		t.userClientMapLock.RUnlock()
		if cli == nil {
			continue
		}
		if cli.isConnected() {
			ct++
		}
	}
	t.currentOnline = ct
}

// 获取一个随机在线用户
func (t *onlineTask) randomOnlineUser() string {
	if len(t.uids) == 0 {
		return ""
	}
	rd := rand.Intn(len(t.uids))
	return t.uids[rd]
}

// 获取一个随机在线用户的client
func (t *onlineTask) randomOnlineClient() *testClient {
	uid := t.randomOnlineUser()
	t.userClientMapLock.Lock()
	defer t.userClientMapLock.Unlock()
	cli := t.userClientMap[uid]

	// 如果是空的，重新获取一个
	if cli == nil {
		uid = t.randomOnlineUser()
		cli = t.userClientMap[uid]
	}

	// 再来一次
	if cli == nil {
		uid = t.randomOnlineUser()
		cli = t.userClientMap[uid]
	}

	// 如果还是空则随便返回一个
	if cli == nil {
		for _, c := range t.userClientMap {
			cli = c
			break
		}
	}

	return cli
}

type testClient struct {
	cli *client.Client
}

func newTestClient(cli *client.Client) *testClient {
	return &testClient{
		cli: cli,
	}
}

func (t *testClient) connect() error {
	return t.cli.Connect()
}

func (t *testClient) isConnected() bool {
	return t.cli.IsConnected()
}

func (t *testClient) send(ch *client.Channel, payload []byte) error {
	return t.cli.SendMessage(ch, payload)
}
