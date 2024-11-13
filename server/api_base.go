package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/WuKongIM/StressTester/pkg/wkhttp"
	"github.com/gin-gonic/gin"
)

type baseApi struct {
	s *server
}

func newBaseApi(s *server) *baseApi {
	return &baseApi{
		s: s,
	}
}

func (b *baseApi) route(r *wkhttp.WKHttp) {
	r.POST("/v1/exchange", b.exchange)
	r.GET("/v1/health", b.health)
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
