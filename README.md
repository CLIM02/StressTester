
## 简介

WuKongIM压测机


## 用法


#### 启动

```shell

go run main.go

```

#### api

与服务器交换信息

```shell

请求方式: POST
请求地址: /v1/exchange

请求参数: 
{
    "server": "http://xx.xxx.xxx:5001" # WuKongIM服务器地址
}

```

返回参数

```shell

{
    "id": "stressxxx", # 压测机id
    "status": 1, # 压测机状态 1.正常 2.异常 3. 压测中
}

```


开始压力测试

```shell

请求方式: POST
请求地址: /v1/stress/start
请求参数: 
{
    "online": 10000, # 在线用户数
    "channels": [ # 频道（群）聊天设置
        {
            "count": 1000, # 频道数
            "type": 2, # 频道类型 2.群聊频道
            "subscriber": {
                "count": 1000, # 每个频道的成员数量
                "online": 100, # 成员在线数量
            },
            "msg_rate": 100, # 每个频道消息发送速率 每分钟条数
        }
    ],
    "p2p": { # 单聊
        "count": 1000, # 私聊数量
        "msg_rate": 100, # 每个私聊消息发送速率 每分钟条数
    },
}

```

压测试报告

```shell

请求方式: GET
请求地址: /v1/stress/report

返回参数
{
    "expected": {
        "online": 10000, # 期望在线用户数
        "channel": 10000, # 期望频道数
        "p2p": 1000, # 期望私聊数量
    },
    "actual": {
        "online": 10000, # 实际在线用户数
        "channel": 1000, # 实际频道数
        "p2p": 1000, # 实际私聊数量
    },
    "send_rate": 10000, # 消息总发送速率 每秒条数
    "recv_rate": 10000, # 消息总接收速率 每秒条数
    "total_send": 10000, # 总发送消息数
    "total_recv": 10000, # 总接收消息数
    "send_failed": 10000, # 发送消息失败数
}


```

停止压力测试

```shell

请求方式: POST  
请求地址: /v1/stress/stop

```


健康检测

```shell

请求方式: GET
请求地址: /v1/health

```