package server

type task interface {
	taskType() taskType // 任务类型
	start()             // 开始任务
	stop()              // 停止任务
	running() bool      // 是否正在执行任务
}
