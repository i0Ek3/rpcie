package main

import (
	"log"
	"net"
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

func startServer(addr chan string) {
	var foo Foo
	if err := server.Register(&foo); err != nil {
		log.Fatal("register error:", err)
	}
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
			args := &Args{Inta: i, Intb: i * i}
			var reply int
			if err := cli.Call("Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Fool.Sum error:", err)
			}
			log.Printf("%d + %d = %d", args.Inta, args.Intb, reply)
		}(i)
	}
	wg.Wait()
}
