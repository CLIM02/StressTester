package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/WuKongIM/StressTester/pkg/client"
	wkproto "github.com/WuKongIM/WuKongIMGoProto"
	"go.uber.org/atomic"
)

type testClient struct {
	cli *client.Client
	s   *server

	preSendCount atomic.Int64 // 上一次发送消息数（统计发送消息速率用）
	sendCount    atomic.Int64 // 发送消息数
	sendBytes    atomic.Int64 // 发送字节数
	preSendBytes atomic.Int64 // 上次发送字节数（统计发送消息速率用）

	preRecv      atomic.Int64 // 上次接受消息数（统计接受消息速率用）
	recvCount    atomic.Int64 // 接收消息数
	recvBytes    atomic.Int64 // 接收字节数
	preRecvBytes atomic.Int64 // 上次接受消息字节

	sendSuccess atomic.Int64 // 发送成功数
	sendErr     atomic.Int64 // 发送错误数

	sendMapLock sync.RWMutex
	sendMap     map[uint64]int64 // 发送消息的记录,key为clientSeq，值为发送时间戳

	sendSuccessMapLock sync.RWMutex
	sendSuccessMap     map[int64]int64 // 发送成功消息记录,key为messageId，值为发送时间戳

	recvMap map[int64]int64 // 接收消息的记录,key为messageId，值为接收时间戳
	uid     string
	stopC   chan struct{}
}

func newTestClient(tcpAddr, uid string, s *server) *testClient {
	cli := client.New(tcpAddr, client.WithUID(uid), client.WithAutoReconn(false), client.WithDefaultBufSize(1024*100))
	t := &testClient{
		cli:            cli,
		s:              s,
		sendMap:        make(map[uint64]int64),
		sendSuccessMap: make(map[int64]int64),
		recvMap:        make(map[int64]int64),
		uid:            uid,
		stopC:          make(chan struct{}),
	}

	onlineTask := s.getOnlineTask()

	cli.SetOnRecv(func(recv *wkproto.RecvPacket) error {

		s.testCount.Add(1)

		// t.recvMapLock.Lock()
		// t.recvMap[recv.MessageID] = time.Now().UnixNano()
		// t.recvMapLock.Unlock()

		t.recvCount.Inc()
		t.recvBytes.Add(int64(recv.Size()))

		fromCli := onlineTask.getUserClient(recv.FromUID)
		if fromCli != nil {
			fromCli.sendSuccessMapLock.Lock()
			if start, ok := fromCli.sendSuccessMap[recv.MessageID]; ok {
				latency := time.Now().UnixNano() - start
				if t.s.recvMinLatency.Load() == 0 || t.s.recvMinLatency.Load() > latency {
					t.s.recvMinLatency.Store(latency)
				}

				if t.s.recvMaxLatency.Load() < latency {
					t.s.recvMaxLatency.Store(latency)
				}

				t.s.recvAvgLatency.Store((t.s.recvAvgLatency.Load()*t.recvCount.Load() + latency) / (t.recvCount.Load() + 1))
			}
			fromCli.sendSuccessMapLock.Unlock()

		}

		return nil
	})

	cli.SetOnSendack(func(sendackPacket *wkproto.SendackPacket) {

		// 发送延迟统计
		if start, ok := t.sendMap[sendackPacket.ClientSeq]; ok {

			t.sendSuccessMapLock.Lock()
			t.sendSuccessMap[sendackPacket.MessageID] = start
			t.sendSuccessMapLock.Unlock()

			latency := time.Now().UnixNano() - start
			t.sendMapLock.Lock()
			delete(t.sendMap, sendackPacket.ClientSeq)
			t.sendMapLock.Unlock()

			if t.s.sendMinLatency.Load() == 0 || t.s.sendMinLatency.Load() > latency {
				t.s.sendMinLatency.Store(latency)
			}

			if t.s.sendMaxLatency.Load() < latency {
				t.s.sendMaxLatency.Store(latency)
			}

			t.s.sendAvgLatency.Store((t.s.sendAvgLatency.Load()*t.sendSuccess.Load() + latency) / (t.sendSuccess.Load() + 1))
		}

		if sendackPacket.ReasonCode == wkproto.ReasonSuccess {
			t.sendSuccess.Inc()
		} else {
			t.sendErr.Inc()
			fmt.Println("send error:", sendackPacket.ReasonCode.String(), t.uid)
		}

	})

	go t.cleanLoop()

	return t
}

func (t *testClient) cleanLoop() {
	tk := time.NewTicker(time.Second * 10)
	for {
		select {
		case <-tk.C:
			t.sendSuccessMapLock.Lock()
			for k, v := range t.sendSuccessMap {
				milliseconds := (time.Now().UnixNano() - v) / 1e6
				if milliseconds > 40000 { // 40s
					delete(t.sendSuccessMap, k)
				}
			}
			t.sendSuccessMapLock.Unlock()
		case <-t.stopC:
			return

		}
	}
}

func (t *testClient) connect() error {
	return t.cli.Connect()
}

func (t *testClient) isConnected() bool {
	return t.cli.IsConnected()
}

func (t *testClient) send(ch *client.Channel, payload []byte) error {

	t.sendCount.Inc()
	packet, dataSize, err := t.cli.SendMessage(ch, payload)
	if err != nil {
		t.sendErr.Inc()
		return err
	}
	t.sendBytes.Add(dataSize)

	t.sendMapLock.Lock()
	t.sendMap[packet.ClientSeq] = time.Now().UnixNano()
	t.sendMapLock.Unlock()
	return nil
}

func (t *testClient) close() {
	close(t.stopC)
	t.cli.Close()
}
