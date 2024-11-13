package server

type taskType string

const (
	taskOnline  taskType = "online"  // 用户上线任务
	taskChannel taskType = "channel" // 频道任务
)
