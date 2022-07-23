package main

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/i0Ek3/rpcie/client"
	"github.com/i0Ek3/rpcie/server"
)

func startServer(addr chan string) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	server.Accept(l)
}

func main() {
	log.SetFlags(0)

	addr := make(chan string)
	go startServer(addr)

	cli, _ := client.Dial("tcp", <-addr)
	defer func() { _ = cli.Close() }()

	time.Sleep(time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := fmt.Sprintf("rpcie req %d", i)
			var reply string
			if err := cli.Call("Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Fool.Sum error:", err)
			}
			log.Println("reply:", reply)
		}(i)
	}
	wg.Wait()
}
