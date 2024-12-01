package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/WuKongIM/StressTester/pkg/wkhttp"
	"github.com/gin-contrib/pprof"
	"go.uber.org/atomic"
	"golang.org/x/exp/rand"
)

func init() {
	rand.Seed(uint64(time.Now().UnixNano()))
}

type server struct {
	stats // 统计信息

	testCount atomic.Int64 // 测试次数

	r    *wkhttp.WKHttp
	opts *Options
	api  *wuKongImApi

	serverCtx context.Context
	cancel    context.CancelFunc

	tasksLock sync.RWMutex
	tasks     []task
	taskCfg   *taskCfg

	taskStartTime time.Time // 任务开始时间
}

func New(opts *Options) *server {
	r := wkhttp.New()
	s := &server{
		opts: opts,
		r:    r,
	}

	s.serverCtx, s.cancel = context.WithCancel(context.Background())

	s.api = newWuKongImApi(s.opts.Server)

	return s
}

func (s *server) Run() error {
	s.setRoutes()
	return s.r.Run(s.opts.Addr)
}

func (s *server) setRoutes() {
	stressApi := newStressApi(s)
	stressApi.route(s.r)

	baseApi := newBaseApi(s)
	baseApi.route(s.r)

	pprof.Register(s.r.GetGinRoute()) // 注册pprof
}

func (s *server) addTask(t task) {
	s.tasksLock.Lock()
	defer s.tasksLock.Unlock()
	s.tasks = append(s.tasks, t)
}

func (s *server) removeAllTask() {
	s.tasksLock.Lock()
	defer s.tasksLock.Unlock()
	s.tasks = s.tasks[:0]
}

func (s *server) getTask(taskType taskType) task {
	s.tasksLock.RLock()
	defer s.tasksLock.RUnlock()
	for _, t := range s.tasks {
		if t.taskType() == taskType {
			return t
		}
	}
	return nil
}

func (s *server) setTaskConfig(cfg *taskCfg) {
	s.taskCfg = cfg
}

func (s *server) startTask() error {
	if s.taskCfg == nil {
		return fmt.Errorf("task config is nil")
	}

	s.taskStartTime = time.Now()
	s.stats = stats{}

	s.removeAllTask()

	// 在线任务
	t := newOnlineTask(s)
	s.addTask(t)
	t.start()

	// 频道任务
	for index, channelCfg := range s.taskCfg.Channels {
		if channelCfg.Count <= 0 {
			continue
		}
		t := newChannelTask(index, channelCfg, s)
		s.addTask(t)
		t.start()
	}

	// 单聊
	if s.taskCfg.P2p != nil && s.taskCfg.P2p.Count > 0 {
		p := newP2pTask(s.taskCfg.P2p, s)
		s.addTask(p)
		p.start()
	}

	// 统计任务
	stats := newStatsTask(s)
	s.addTask(stats)
	stats.start()

	return nil
}

func (s *server) stopTask() {
	s.tasksLock.Lock()
	defer s.tasksLock.Unlock()

	s.taskStartTime = time.Time{}

	for _, t := range s.tasks {
		t.stop()
	}
}

// 是否正在执行任务
func (s *server) isRunning() bool {
	s.tasksLock.RLock()
	defer s.tasksLock.RUnlock()
	for _, t := range s.tasks {
		if t.running() {
			return true
		}
	}
	return false
}

// 生成用户uid
func (s *server) genUid(index int) string {

	return fmt.Sprintf("%s%07d", s.taskCfg.UserPrefix, index)
}

// 生成频道id
func (s *server) genChannelId(taskIndex, index int) string {
	return fmt.Sprintf("%s%d:%07d", s.taskCfg.ChannelPrefix, taskIndex, index)
}

func (s *server) getOnlineTask() *onlineTask {
	task := s.getTask(taskOnline)
	if task == nil {
		return nil
	}
	return task.(*onlineTask)
}

func (s *server) getChannelTasks() []*channelTask {
	s.tasksLock.RLock()
	defer s.tasksLock.RUnlock()
	var tasks []*channelTask
	for _, t := range s.tasks {
		if t.taskType() == taskChannel {
			tasks = append(tasks, t.(*channelTask))
		}
	}
	return tasks
}

func (s *server) getP2pTask() *p2pTask {
	task := s.getTask(taskP2p)
	if task == nil {
		return nil
	}
	return task.(*p2pTask)
}

// 获取模拟的消息
func (s *server) getMockMsg() []byte {

	return []byte(`{"type":1,"content":"000000000000000000000000000000"}`)
}
