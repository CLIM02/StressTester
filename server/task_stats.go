package server

import (
	"sync/atomic"
	"time"

	"github.com/WuKongIM/WuKongIM/pkg/wklog"
)

type statsTask struct {
	s *server
	wklog.Log
	isRunning      atomic.Bool
	stopC          chan struct{}
	intervalSecond int // 间隔获取统计数据的秒数
}

func newStatsTask(s *server) task {
	return &statsTask{
		s:              s,
		Log:            wklog.NewWKLog("statsTask"),
		stopC:          make(chan struct{}),
		intervalSecond: 2,
	}
}

func (s *statsTask) taskType() taskType {
	return taskStats
}

func (s *statsTask) start() {
	s.isRunning.Store(true)
	go s.loop()
}

func (s *statsTask) stop() {
	s.isRunning.Store(false)
	close(s.stopC)
}

func (s *statsTask) running() bool {

	return s.isRunning.Load()
}

func (s *statsTask) loop() {
	tk := time.NewTicker(time.Second * time.Duration(s.intervalSecond))
	defer tk.Stop()
	for {
		select {
		case <-tk.C:
			s.startCount()
		case <-s.stopC:
			return
		}
	}
}

func (s *statsTask) startCount() {

	// 在线数统计
	s.countOnline()

	// 消息统计
	s.countMsg()

}

// 统计在线用户数
func (s *statsTask) countOnline() {
	onlineTask := s.getOnlineTask()
	clients := onlineTask.allClient()
	online := 0
	for _, c := range clients {
		if c.isConnected() {
			online++
		}
	}
	s.s.online.Store(int64(online))
	s.s.offline.Store(int64(len(clients) - online))
}

// 统计消息
func (s *statsTask) countMsg() {
	onlineTask := s.getOnlineTask()
	clients := onlineTask.allClient()
	var (
		send           int64 // 发送消息数
		batchSend      int64
		sendSuccess    int64
		sendErr        int64
		sendBytes      int64
		batchSendBytes int64

		recv           int64
		recvBytes      int64
		batchRecv      int64
		batchRecvBytes int64
	)
	for _, c := range clients {
		send += c.sendCount.Load()
		batchSend += (c.sendCount.Load() - c.preSendCount.Load())
		c.preSendCount.Store(c.sendCount.Load())

		sendSuccess += c.sendSuccess.Load()
		sendErr += c.sendErr.Load()
		sendBytes += c.sendBytes.Load()
		batchSendBytes += (c.sendBytes.Load() - c.preSendBytes.Load())
		c.preSendBytes.Store(c.sendBytes.Load())

		recv += c.recvCount.Load()
		recvBytes += c.recvBytes.Load()
		batchRecv += (c.recvCount.Load() - c.preRecv.Load())
		batchRecvBytes += (c.recvBytes.Load() - c.preRecvBytes.Load())
		c.preRecv.Store(c.recvCount.Load())
		c.preRecvBytes.Store(c.recvBytes.Load())

	}
	s.s.send.Store(send)
	s.s.sendRate.Store(batchSend / int64(s.intervalSecond))
	s.s.sendSuccess.Store(sendSuccess)
	s.s.sendErr.Store(sendErr)
	s.s.sendBytes.Store(sendBytes)
	s.s.sendBytesRate.Store(batchSendBytes / int64(s.intervalSecond))

	s.s.recv.Store(recv)
	s.s.recvBytes.Store(recvBytes)
	s.s.recvRate.Store(batchRecv / (int64(s.intervalSecond)))
	s.s.recvBytesRate.Store(batchRecvBytes / int64(s.intervalSecond))
}

func (s *statsTask) getOnlineTask() *onlineTask {
	task := s.s.getTask(taskOnline)
	if task == nil {
		return nil
	}
	return task.(*onlineTask)
}
