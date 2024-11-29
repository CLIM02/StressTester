package server

import "go.uber.org/atomic"

type stats struct {
	online  atomic.Int64 // 在线人数
	offline atomic.Int64 // 离线人数

	send           atomic.Int64 // 发送消息数
	sendRate       atomic.Int64 // 发送消息速率 (条/秒)
	sendSuccess    atomic.Int64 // 发送成功数
	sendErr        atomic.Int64 // 发送错误数
	sendBytes      atomic.Int64 // 发送字节数
	sendBytesRate  atomic.Int64 // 发送字节速率 (字节/秒)
	sendMinLatency atomic.Int64 // 发送最小延迟 (毫秒)
	sendMaxLatency atomic.Int64 // 发送最大延迟 (毫秒)
	sendAvgLatency atomic.Int64 // 发送平均延迟 (毫秒)

	recv           atomic.Int64 // 接收消息数
	expectRecv     atomic.Int64 // 预期接收消息数
	recvRate       atomic.Int64 // 接收消息速率 (条/秒)
	recvBytes      atomic.Int64 // 接收字节数
	recvBytesRate  atomic.Int64 // 接收字节速率 (字节/秒)
	recvMinLatency atomic.Int64 // 接收最小延迟 (毫秒)
	recvMaxLatency atomic.Int64 // 接收最大延迟 (毫秒)
	recvAvgLatency atomic.Int64 // 接收平均延迟 (毫秒)

}
