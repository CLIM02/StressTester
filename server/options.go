package server

import (
	"net"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/WuKongIM/StressTester/pkg/wkutil"
)

type Options struct {
	Id   string // 压测机器的id
	Addr string // 监听地址 例如: :8080
	port int

	DataDir string // 数据目录

	Server string // WuKongIM服务地址 例如: http://xx.xx.xx.xx:5001

	// 每批上线人数
	OnlinePerBatch int

	// 每批创建频道数
	CreateChannelPerBatch int
	// 消息大小
	MsgByteSize int
}

func NewOptions(opts ...Option) *Options {
	defaultOpts := &Options{
		Addr:                  ":8080",
		DataDir:               "./stressdata",
		OnlinePerBatch:        100,
		CreateChannelPerBatch: 10,
		MsgByteSize:           1024,
	}
	for _, o := range opts {
		o(defaultOpts)
	}

	defaultOpts.parsePort()

	// 创建数据目录
	if defaultOpts.DataDir != "" {
		err := os.Mkdir(defaultOpts.DataDir, os.ModePerm)
		if err != nil && !os.IsExist(err) {
			panic(err)
		}
	}

	if defaultOpts.Id == "" {
		defaultOpts.Id = defaultOpts.genId()
	}

	defaultOpts.Server = defaultOpts.readServerAddr()

	return defaultOpts
}

// 从addr里解析出来port
func (o *Options) parsePort() {
	if o.Addr != "" {
		_, port, err := net.SplitHostPort(o.Addr)
		if err == nil {
			o.port, _ = strconv.Atoi(port)
		}
	}
}

// 生成压测机器的id
func (o *Options) genId() string {
	if o.Id != "" {
		return o.Id
	}

	// 先成本地文件读取id

	// 判断本地文件是否存在
	_, err := os.Stat(path.Join(o.DataDir, "id"))
	exist := true
	if err != nil {
		if os.IsNotExist(err) {
			exist = false
			// 不存在
			_, err = os.Create(path.Join(o.DataDir, "id"))
			if err != nil {
				panic(err)
			}
		} else {
			panic(err)
		}
	}

	if exist {
		idBytes, err := os.ReadFile(path.Join(o.DataDir, "id"))
		if err != nil {
			panic(err)
		}
		if len(idBytes) > 0 {
			return string(idBytes)
		}
	}

	intranets, err := wkutil.GetIntranetIP()
	if err != nil {
		return ""
	}
	ip := "0.0.0.0"
	if len(intranets) > 0 {
		ip = intranets[0]
	}

	suffix := wkutil.GenUUID()
	suffix = suffix[len(suffix)-4:]

	ip = strings.ReplaceAll(ip, ".", "")
	ip = ip[len(ip)-4:]
	prefix := "ss"

	id := prefix + ip + suffix

	// 保存id到本地文件
	err = os.WriteFile(path.Join(o.DataDir, "id"), []byte(id), os.ModePerm)
	if err != nil {
		panic(err)
	}

	return id
}

func (o *Options) readServerAddr() string {
	if o.Server != "" {
		return o.Server
	}
	// 读取本地文件
	serverAddrBytes, err := os.ReadFile(path.Join(o.DataDir, "server"))
	if err != nil {
		return ""
	}
	if len(serverAddrBytes) > 0 {
		return string(serverAddrBytes)
	}

	return ""
}

func (o *Options) writeServerAddr(serverAddr string) error {
	err := os.WriteFile(path.Join(o.DataDir, "server"), []byte(serverAddr), os.ModePerm)
	if err != nil {
		return err
	}
	o.Server = serverAddr
	return nil
}

type Option func(*Options)

func WithAddr(addr string) Option {
	return func(o *Options) {
		o.Addr = addr
	}
}

func WithId(id string) Option {
	return func(o *Options) {
		o.Id = id
	}
}
