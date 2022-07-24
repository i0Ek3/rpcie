package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/i0Ek3/rpcie/client"
	"github.com/i0Ek3/rpcie/server"
)

type Foo int

type Args struct {
	Inta, Intb int
}

// Sum is a RPC method
func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Inta + args.Intb
	return nil
}

func startServer(addrCh chan string) {
	var foo Foo
	if err := server.Register(&foo); err != nil {
		log.Fatal("register error:", err)
	}
	l, err := net.Listen("tcp", ":8888")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Printf("start rpc server on %s", l.Addr())
	server.HandleHTTP()
	addrCh <- l.Addr().String()
	_ = http.Serve(l, nil)
}

func call(addrCh chan string) {
	cli, _ := client.DialHTTP("tcp", <-addrCh)
	defer func() { _ = cli.Close() }()

	time.Sleep(time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := &Args{Inta: i, Intb: i * i}
			var reply int
			if err := cli.Call(context.Background(), "Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Fool.Sum error:", err)
			}
			log.Printf("%d + %d = %d", args.Inta, args.Intb, reply)
		}(i)
	}
	wg.Wait()
	log.Println("please visit http://localhost:8888/debug/rpcie")
}

func main() {
	log.SetFlags(0)
	addr := make(chan string)
	go call(addr)
	startServer(addr)
}
