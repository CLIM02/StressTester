package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/WuKongIM/WuKongIM/pkg/network"
	"github.com/WuKongIM/WuKongIM/pkg/wkutil"
	"github.com/sendgrid/rest"
)

type wuKongImApi struct {
	baseURL string
}

func newWuKongImApi(baseURL string) *wuKongImApi {
	return &wuKongImApi{
		baseURL: baseURL,
	}
}

// 获取用户的连接地址
func (w *wuKongImApi) route(uids []string) (map[string]string, error) {

	resp, err := network.Post(w.getFullURL("/route/batch"), []byte(wkutil.ToJSON(uids)), nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("请求失败")
	}

	var userAddrs []userAddrResp
	err = wkutil.ReadJSONByByte([]byte(resp.Body), &userAddrs)
	if err != nil {
		return nil, err
	}

	resultMap := make(map[string]string)
	if len(userAddrs) > 0 {
		for _, userAddr := range userAddrs {
			if len(userAddr.UIDs) > 0 {
				for _, uid := range userAddr.UIDs {
					resultMap[uid] = userAddr.TCPAddr
				}
			}
		}
	}
	return resultMap, nil

}

// 创建频道
func (w *wuKongImApi) createChannel(req *channelCreateReq) error {

	resp, err := network.Post(w.getFullURL("/channel"), []byte(wkutil.ToJSON(req)), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return w.handleError(resp)
	}
	return nil
}

func (w *wuKongImApi) handleError(resp *rest.Response) error {
	if resp.StatusCode == http.StatusBadRequest {
		resultMap, err := wkutil.JSONToMap(resp.Body)
		if err != nil {
			return err
		}
		msg, ok := resultMap["msg"]
		if ok {
			return errors.New(msg.(string))
		}
	}
	return errors.New("未知错误")
}

func (w *wuKongImApi) getFullURL(path string) string {
	return strings.TrimSuffix(w.baseURL, "/") + path
}

type userAddrResp struct {
	TCPAddr string   `json:"tcp_addr"`
	WSAddr  string   `json:"ws_addr"`
	UIDs    []string `json:"uids"`
}

type channelInfoReq struct {
	ChannelId   string `json:"channel_id"`   // 频道ID
	ChannelType uint8  `json:"channel_type"` // 频道类型
	Large       int    `json:"large"`        // 是否是超大群
	Ban         int    `json:"ban"`          // 是否封禁频道（封禁后此频道所有人都将不能发消息，除了系统账号）
}

// 频道创建请求
type channelCreateReq struct {
	channelInfoReq
	Subscribers []string `json:"subscribers"` // 订阅者
}
