package server

import (
	"context"
	"fmt"
	"sync"

	"github.com/WuKongIM/StressTester/pkg/wkhttp"
)

type server struct {
	r    *wkhttp.WKHttp
	opts *Options
	api  *wuKongImApi

	serverCtx context.Context
	cancel    context.CancelFunc

	tasksLock sync.RWMutex
	tasks     []task
	taskCfg   *taskCfg
}

func New(opts *Options) *server {
	r := wkhttp.New()

	fmt.Println("opts-->", opts.Id)

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
}

func (s *server) addTask(t task) {
	s.tasksLock.Lock()
	defer s.tasksLock.Unlock()
	s.tasks = append(s.tasks, t)
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
	// 在线任务
	t := newOnlineTask(s)
	s.addTask(t)
	t.start()

	// 频道任务
	for _, channelCfg := range s.taskCfg.Channels {
		t := newChannelTask(channelCfg, s)
		s.addTask(t)
		t.start()
	}

	return nil
}

func (s *server) stopTask() {
	s.tasksLock.Lock()
	defer s.tasksLock.Unlock()
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
func (s *server) genChannelId(index int) string {
	return fmt.Sprintf("%s%07d", s.taskCfg.ChannelPrefix, index)
}
