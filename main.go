package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/i0Ek3/rpcie/registry"
	"github.com/i0Ek3/rpcie/server"
	"github.com/i0Ek3/rpcie/xclient"
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

func (f Foo) Sleep(args Args, reply *int) error {
	time.Sleep(time.Second * time.Duration(args.Inta))
	*reply = args.Inta + args.Intb
	return nil
}

func startRegistry(wg *sync.WaitGroup) {
	l, _ := net.Listen("tcp", ":8888")
	registry.HandleHTTP()
	wg.Done()
	_ = http.Serve(l, nil)
}

func startServer(registryAddr string, wg *sync.WaitGroup) {
	var foo Foo
	l, _ := net.Listen("tcp", ":0")
	server := server.NewServer()
	_ = server.Register(&foo)
	registry.Heartbeat(registryAddr, "tcp@"+l.Addr().String(), 0)
	wg.Done()
	server.Accept(l)
}

func foo(ctx context.Context, xc *xclient.XClient, typ, serviceMethod string, args *Args) {
	var reply int
	var err error
	switch typ {
	case "call":
		err = xc.Call(ctx, serviceMethod, args, &reply)
	case "broadcast":
		err = xc.Broadcast(ctx, serviceMethod, args, &reply)
	}
	if err != nil {
		log.Printf("%s %s error: %v", typ, serviceMethod, err)
	} else {
		log.Printf("%s %s success: %d + %d = %d", typ, serviceMethod, args.Inta, args.Intb, reply)
	}
}

func call(registry string) {
	d := xclient.NewRegistryDiscovery(registry, 0)
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
	defer func() { _ = xc.Close() }()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(context.Background(), xc, "call", "Foo.Sum", &Args{Inta: i, Intb: i * i})
		}(i)
	}
	wg.Wait()
}

func broadcast(registry string) {
	d := xclient.NewRegistryDiscovery(registry, 0)
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
	defer func() { _ = xc.Close() }()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(context.Background(), xc, "broadcast", "Foo.Sum", &Args{Inta: i, Intb: i * i})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
			defer cancel()
			foo(ctx, xc, "broadcast", "Foo.Sleep", &Args{Inta: i, Intb: i * i})
		}(i)
	}
	wg.Wait()
}

func main() {
	log.SetFlags(0)
	registryAddr := "http://localhost:8888/_rpcie_/registry"

	var wg sync.WaitGroup
	wg.Add(1)
	go startRegistry(&wg)
	wg.Wait()

	time.Sleep(time.Second)
	wg.Add(2)
	go startServer(registryAddr, &wg)
	go startServer(registryAddr, &wg)
	wg.Wait()

	time.Sleep(time.Second)
	call(registryAddr)
	broadcast(registryAddr)
}
