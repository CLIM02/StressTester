package main

import (
	"github.com/WuKongIM/StressTester/server"
)

// go ldflags
var Version string    // version
var Commit string     // git commit id
var CommitDate string // git commit date
var TreeState string  // git tree state

func main() {

	s := server.New(server.NewOptions(server.WithAddr(":9466")))
	err := s.Run()
	if err != nil {
		panic(err)
	}
}
