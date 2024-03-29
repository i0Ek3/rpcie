package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/i0Ek3/rpcie/codec"
)

const (
	// MagicNumber denotes this is a rpcie request
	MagicNumber = 0x3bef5c

	Connected        = "200 Connected to rpcie"
	DefaultRPCPath   = "/_rpcie_"
	DefaultDebugPath = "/debug/rpcie"
)

// Option denotes the encoding and decoding method of the message
type Option struct {
	MagicNumber int
	CodecType   codec.Type

	// timeout control
	ConnectTimeout time.Duration
	HandleTimeout  time.Duration
}

var DefaultOption = &Option{
	MagicNumber:    MagicNumber,
	CodecType:      codec.GobType,
	ConnectTimeout: 10 * time.Second,
}

// Server denotes an RPC Server
type Server struct {
	serviceMap sync.Map
}

func NewServer() *Server {
	return &Server{}
}

var DefaultServer = NewServer()

func (server *Server) Register(rcvr any) error {
	s := newService(rcvr)
	if _, dup := server.serviceMap.LoadOrStore(s.name, s); dup {
		return errors.New("rpc: service already defined: " + s.name)
	}
	return nil
}

func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = io.WriteString(w, "405 must CONNECT\n")
		return
	}
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Print("rpc hijacking ", req.RemoteAddr, ": ", err.Error())
		return
	}
	_, _ = io.WriteString(conn, "HTTP/1.0 "+Connected+"\n\n")
	server.ServeConn(conn)
}

func (server *Server) HandleHTTP() {
	http.Handle(DefaultRPCPath, server)
	http.Handle(DefaultDebugPath, debugHTTP{server})
	log.Println("rpc server debug path:", DefaultDebugPath)
}

func HandleHTTP() {
	DefaultServer.HandleHTTP()
}

func Register(rcvr any) error {
	return DefaultServer.Register(rcvr)
}

func (server *Server) findService(serviceMethod string) (svc *service, mtype *methodType, err error) {
	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc server: servicer/method request ill-formed: " + serviceMethod)
		return
	}
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]
	svc_, ok := server.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("rpc server: cannot find service " + serviceMethod)
		return
	}
	svc = svc_.(*service)
	mtype = svc.method[methodName]
	if mtype == nil {
		err = errors.New("rpc server: cannot find method " + methodName)
	}
	return
}

func (server *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error:", err)
			return
		}
		go server.ServeConn(conn)
	}
}

func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}

func (server *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() { _ = conn.Close() }()
	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("rpc server: options error:", err)
		return
	}
	if opt.MagicNumber != MagicNumber {
		log.Printf("rpc server: invalid magic number %x", opt.MagicNumber)
		return
	}
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("rpc server: invalid codec type %s:", opt.CodecType)
		return
	}
	server.serveCodec(f(conn), &opt)
}

var invalidRequest = struct{}{}

// serveCodec reads and handles request, and then sends response
func (server *Server) serveCodec(cc codec.Codec, opt *Option) {
	// sendLock uses to ensure reply to the request
	// message must be sent one by one
	sendLock := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	for {
		req, err := server.readRequest(cc)
		if err != nil {
			if req == nil {
				break
			}
			req.h.Error = err.Error()
			server.sendResponse(cc, req.h, invalidRequest, sendLock)
			continue
		}
		wg.Add(1)
		go server.handleRequest(cc, req, sendLock, wg, opt.HandleTimeout)
	}
	wg.Wait()
	_ = cc.Close()
}

// request stores the details of a Call object
type request struct {
	h *codec.Header
	// request argument
	argv reflect.Value
	// request reply
	replyv reflect.Value
	mtype  *methodType
	svc    *service
}

func (server *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server: read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}

func (server *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := server.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &request{h: h}
	req.svc, req.mtype, err = server.findService(h.ServiceMethod)
	if err != nil {
		return req, nil
	}
	// create two input parameter objects
	req.argv = req.mtype.newArgv()
	req.replyv = req.mtype.newReplyv()

	argv_ := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		argv_ = req.argv.Addr().Interface()
	}
	// deserialize the request message into the first input parameter argv
	if err = cc.ReadBody(argv_); err != nil {
		log.Println("rpc server: read body err:", err)
		return req, err
	}
	return req, nil
}

func (server *Server) sendResponse(cc codec.Codec, h *codec.Header, body any, sendLock *sync.Mutex) {
	sendLock.Lock()
	defer sendLock.Unlock()
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error:", err)
	}
}

func (server *Server) handleRequest(cc codec.Codec, req *request, sendLock *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	called := make(chan struct{})
	sent := make(chan struct{})
	go func() {
		// call method
		err := req.svc.call(req.mtype, req.argv, req.replyv)
		called <- struct{}{}
		if err != nil {
			req.h.Error = err.Error()
			server.sendResponse(cc, req.h, invalidRequest, sendLock)
			sent <- struct{}{}
			return
		}
		// pass replyv to sendResponse to complete serialization
		server.sendResponse(cc, req.h, req.replyv.Interface(), sendLock)
		sent <- struct{}{}
	}()
	if timeout == 0 {
		<-called
		<-sent
		return
	}
	select {
	case <-time.After(timeout):
		req.h.Error = fmt.Sprintf("rpc server: request handle timeout: expect within %s", timeout)
		server.sendResponse(cc, req.h, invalidRequest, sendLock)
	case <-called:
		<-sent
	}
}
