package main

import (
	"flag"

	"github.com/WuKongIM/StressTester/server"
)

// go ldflags
var Version string    // version
var Commit string     // git commit id
var CommitDate string // git commit date
var TreeState string  // git tree state

func main() {

	seq := flag.Bool("seq", true, "是否每次测试都生成新的批次序号")
	serverAddr := flag.String("server", "", "WuKongIM服务的接口地址，例如http://xxx.xxx.xxx.xx:5001")

	flag.Parse()

	var err error

	// logFile, err := os.OpenFile("./fatal.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0660)
	// if err != nil {
	// 	log.Println("服务启动出错", "打开异常日志文件失败", err)
	// 	return
	// }
	// // 将进程标准出错重定向至文件，进程崩溃时运行时将向该文件记录协程调用栈信息
	// syscall.Dup2(int(logFile.Fd()), int(os.Stderr.Fd()))

	s := server.New(server.NewOptions(server.WithAddr(":9466"), server.WithServer(*serverAddr), server.WithNewSeq(*seq)))
	err = s.Run()
	if err != nil {
		panic(err)
	}
}
