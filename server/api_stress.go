package server

import (
	"errors"
	"fmt"
	"strings"

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

	r.POST("/v1/stress/start", s.start)
	r.POST("/v1/stress/stop", s.stop)
	r.GET("/v1/stress/result", s.result)

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

	if strings.TrimSpace(req.UserPrefix) == "" {
		req.UserPrefix = fmt.Sprintf("%susr", s.s.opts.Id)
	}
	if strings.TrimSpace(req.ChannelPrefix) == "" {
		req.ChannelPrefix = fmt.Sprintf("%scha", s.s.opts.Id)
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

func (s *stressApi) result(c *wkhttp.Context) {

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
