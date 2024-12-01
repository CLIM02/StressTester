package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/WuKongIM/StressTester/pkg/wkhttp"
	"github.com/WuKongIM/WuKongIM/pkg/wklog"
	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"go.uber.org/zap"
)

type baseApi struct {
	s *server
	wklog.Log
}

func newBaseApi(s *server) *baseApi {
	return &baseApi{
		s:   s,
		Log: wklog.NewWKLog("baseApi"),
	}
}

func (b *baseApi) route(r *wkhttp.WKHttp) {
	r.POST("/v1/exchange", b.exchange)    // 交换数据
	r.GET("/v1/health", b.health)         // 监控检测
	r.GET("/v1/systemInfo", b.systemInfo) // 系统信息

}

func (b *baseApi) exchange(c *wkhttp.Context) {
	var req struct {
		Server string `json:"server"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.ResponseError(err)
		return
	}
	if strings.TrimSpace(req.Server) == "" {
		c.ResponseError(errors.New("server is empty"))
		return
	}

	err := b.s.opts.writeServerAddr(req.Server)
	if err != nil {
		c.ResponseError(err)
		return
	}
	b.s.api.baseURL = req.Server

	c.JSON(http.StatusOK, gin.H{
		"id":     b.s.opts.Id,
		"status": 1,
	})

}

func (b *baseApi) health(c *wkhttp.Context) {

	c.JSON(200, gin.H{
		"status": 1,
	})
}

func (b *baseApi) systemInfo(c *wkhttp.Context) {
	cpuInfos, err := cpu.Info()
	if err != nil {
		b.Error("获取 CPU 信息出错", zap.Error(err))
		c.ResponseError(err)
		return
	}
	coreNum := 0
	for _, cpuInfo := range cpuInfos {
		coreNum += int(cpuInfo.Cores)
	}

	percentages, err := cpu.Percent(1*time.Second, false)
	if err != nil {
		b.Error("获取 CPU 使用率失败", zap.Error(err))
		c.ResponseError(err)
		return
	}

	// 获取内存信息
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		b.Error("获取内存信息出错", zap.Error(err))
		c.ResponseError(err)
		return
	}

	desc := fmt.Sprintf("%d核 %s", coreNum, formatBytes(vmStat.Total))

	isRunning := b.s.isRunning()

	status := testerStatusNormal
	taskDesc := "未运行"
	var taskDuration int64 = 0
	if isRunning {
		status = testerStatusRunning
		if !b.s.taskStartTime.IsZero() {
			taskDuration = time.Now().Unix() - b.s.taskStartTime.Unix()
			taskDesc = formatTime(taskDuration)
		}
	}

	systemInfo := systemInfo{
		No:               b.s.opts.Id,
		CpuPercent:       float32(percentages[0]),
		CpuCore:          coreNum,
		MemoryTotal:      int(vmStat.Total),
		MemoryFree:       int(vmStat.Free),
		MemoryUsed:       int(vmStat.Used),
		MemoryPercent:    vmStat.UsedPercent,
		Desc:             desc,
		Status:           status,
		TaskDuration:     taskDuration,
		TaskDurationDesc: taskDesc,
	}

	systemInfo.MemoryFreeDesc = formatBytes(uint64(systemInfo.MemoryTotal))
	systemInfo.MemoryPercentDesc = fmt.Sprintf("%0.2f%%", systemInfo.MemoryPercent)
	systemInfo.MemoryUsedDesc = formatBytes(uint64(systemInfo.MemoryUsed))
	systemInfo.MemoryTotalDesc = formatBytes(uint64(systemInfo.MemoryTotal))

	systemInfo.CpuPercentDesc = fmt.Sprintf("%0.2f%%", systemInfo.CpuPercent)
	if b.s.taskCfg != nil {
		systemInfo.Task = *b.s.taskCfg
	}

	c.JSON(http.StatusOK, systemInfo)

}

type systemInfo struct {
	No                string       `json:"no"`                  // 压测机编号
	CpuPercent        float32      `json:"cpu_percent"`         // cpu使用率
	CpuPercentDesc    string       `json:"cpu_percent_desc"`    // cpu使用率描述
	CpuCore           int          `json:"cpu_core"`            // cpu核心数
	MemoryTotal       int          `json:"memory_total"`        // 总内存数量(单位byte)
	MemoryTotalDesc   string       `json:"memory_total_desc"`   // 总内存描述
	MemoryFree        int          `json:"memory_free"`         // 空闲内存数量(单位byte)
	MemoryFreeDesc    string       `json:"memory_free_desc"`    // 空闲内存描述
	MemoryUsed        int          `json:"memory_used"`         // 已使用内存数量(单位byte)
	MemoryUsedDesc    string       `json:"memory_used_desc"`    // 已使用内存描述
	MemoryPercent     float64      `json:"memory_percent"`      // 内存使用率
	MemoryPercentDesc string       `json:"memory_percent_desc"` // 内存使用率描述
	Desc              string       `json:"desc"`
	Status            testerStatus `json:"status"`
	Task              taskCfg      `json:"task"`
	TaskDuration      int64        `json:"task_duration"`      // 任务时间
	TaskDurationDesc  string       `json:"task_duration_desc"` // 任务时间描述
}

func formatBytes(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
		PB = TB * 1024
	)

	switch {
	case bytes >= PB:
		return fmt.Sprintf("%.2f PB", float64(bytes)/float64(PB))
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func formatTime(second int64) string {
	hours := second / 3600
	minutes := (second % 3600) / 60
	seconds := second % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

type testerStatus int

const (
	// 未知状态
	testerStatusUnknow testerStatus = 0
	// 正常
	testerStatusNormal testerStatus = 1
	// 运行中
	testerStatusRunning testerStatus = 2
	// 错误
	testerStatusErr testerStatus = 3
)
