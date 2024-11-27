package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/WuKongIM/StressTester/pkg/wkhttp"
)

type stressApi struct {
	s *server
}

func newStressApi(s *server) *stressApi {

	return &stressApi{
		s: s,
	}
}

func (s *stressApi) route(r *wkhttp.WKHttp) {

	r.POST("/v1/stress/start", s.start)  // 开始压测
	r.POST("/v1/stress/stop", s.stop)    // 停止压测
	r.GET("/v1/stress/report", s.report) // 查看报告

}

func (s *stressApi) start(c *wkhttp.Context) {
	var req *taskCfg
	if err := c.BindJSON(&req); err != nil {
		c.ResponseError(err)
		return
	}
	if err := req.check(); err != nil {
		c.ResponseError(err)
		return
	}

	seq := s.s.opts.nextTestSeq()

	if strings.TrimSpace(req.UserPrefix) == "" {
		req.UserPrefix = fmt.Sprintf("%s:%d:usr:", s.s.opts.Id, seq)
	}
	if strings.TrimSpace(req.ChannelPrefix) == "" {
		req.ChannelPrefix = fmt.Sprintf("%s:%d:cha:", s.s.opts.Id, seq)
	}

	for _, ch := range req.Channels {
		if ch.Type == 0 {
			ch.Type = 2
		}
	}

	if s.s.isRunning() {
		c.ResponseError(errors.New("任务运行中，请先停止"))
		return
	}

	s.s.setTaskConfig(req)

	err := s.s.startTask()
	if err != nil {
		c.ResponseError(err)
		return
	}

	c.ResponseOK()

}

func (s *stressApi) stop(c *wkhttp.Context) {
	if !s.s.isRunning() {
		c.ResponseError(errors.New("任务未运行"))
		return
	}

	s.s.stopTask()

	c.ResponseOK()
}

func (s *stressApi) report(c *wkhttp.Context) {
	resp := newStatsResp(s.s.stats)

	isRunning := s.s.isRunning()

	taskDesc := "未运行"
	var taskDuration int64 = 0
	if isRunning {
		if !s.s.taskStartTime.IsZero() {
			taskDuration = time.Now().Unix() - s.s.taskStartTime.Unix()
			taskDesc = formatTime(taskDuration)
		}
	}
	resp.TaskDuration = taskDuration
	resp.TaskDurationDesc = taskDesc

	c.JSON(http.StatusOK, resp)
}

type taskCfg struct {
	Online        int           `json:"online"`         // 在线人数
	Channels      []*channelCfg `json:"channels"`       // 频道（群）聊天设置
	P2p           *p2pCfg       `json:"p2p"`            // 私聊设置
	UserPrefix    string        `json:"user_prefix"`    // 用户前缀
	ChannelPrefix string        `json:"channel_prefix"` // 频道前缀
}

func (t *taskCfg) check() error {
	if t.Online <= 0 {
		return errors.New("online must be greater than 0")
	}

	return nil
}

type channelCfg struct {
	Count      int            `json:"count"`      //  创建频道数
	Type       int            `json:"type"`       //  频道类型
	Subscriber *subscriberCfg `json:"subscriber"` //  频道成员设置
	MsgRate    int            `json:"msg_rate"`   //  每个频道消息发送速率 每分钟条数
}

type subscriberCfg struct {
	Count  int `json:"count"`  //  每个频道的成员数量
	Online int `json:"online"` //  成员在线数量
}

type p2pCfg struct {
	Count   int `json:"count"`    //  私聊数量
	MsgRate int `json:"msg_rate"` //  每个私聊消息发送速率 每分钟条数
}

type statsResp struct {
	Online         int64 `json:"online"`           // 在线人数
	Offline        int64 `json:"offline"`          // 离线人数
	Send           int64 `json:"send"`             // 发送消息数
	SendRate       int64 `json:"send_rate"`        // 发送消息速率 (条/秒)
	SendSuccess    int64 `json:"send_success"`     // 发送成功数
	SendErr        int64 `json:"send_err"`         // 发送失败数
	SendBytes      int64 `json:"send_bytes"`       // 发送字节数
	SendBytesRate  int64 `json:"send_bytes_rate"`  // 发送字节速率 (字节/秒)
	SendMinLatency int64 `json:"send_min_latency"` // 发送最小延迟 (毫秒)
	SendMaxLatency int64 `json:"send_max_latency"` // 发送最大延迟 (毫秒)
	SendAvgLatency int64 `json:"send_avg_latency"` // 发送平均延迟 (毫秒)

	Recv           int64 `json:"recv"`             // 接收消息数
	RecvRate       int64 `json:"recv_rate"`        // 接收消息速率 (条/秒)
	RecvBytes      int64 `json:"recv_bytes"`       // 接收字节数
	RecvBytesRate  int64 `json:"recv_bytes_rate"`  // 接收字节速率 (字节/秒)
	RecvMinLatency int64 `json:"recv_min_latency"` // 接收最小延迟 (毫秒)
	RecvMaxLatency int64 `json:"recv_max_latency"` // 接收最大延迟 (毫秒)
	RecvAvgLatency int64 `json:"recv_avg_latency"` // 接收平均延迟 (毫秒)

	Task             taskCfg `json:"task"`
	TaskDuration     int64   `json:"task_duration"`      // 任务时间
	TaskDurationDesc string  `json:"task_duration_desc"` // 任务时间描述
}

func newStatsResp(s stats) *statsResp {

	return &statsResp{
		Online:         s.online.Load(),
		Offline:        s.offline.Load(),
		Send:           s.send.Load(),
		SendRate:       s.sendRate.Load(),
		SendSuccess:    s.sendSuccess.Load(),
		SendErr:        s.sendErr.Load(),
		SendBytes:      s.sendBytes.Load(),
		SendBytesRate:  s.sendBytesRate.Load(),
		SendMinLatency: s.sendMinLatency.Load() / 1e6,
		SendMaxLatency: s.sendMaxLatency.Load() / 1e6,
		SendAvgLatency: s.sendAvgLatency.Load() / 1e6,
		Recv:           s.recv.Load(),
		RecvRate:       s.recvRate.Load(),
		RecvBytes:      s.recvBytes.Load(),
		RecvBytesRate:  s.recvBytesRate.Load(),
		RecvMinLatency: s.recvMinLatency.Load() / 1e6,
		RecvMaxLatency: s.recvMaxLatency.Load() / 1e6,
		RecvAvgLatency: s.recvAvgLatency.Load() / 1e6,
	}
}
