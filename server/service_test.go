package server

import (
	"fmt"
	"reflect"
	"testing"
)

type Foo int

type Args struct {
	Inta, Intb int
}

func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Inta + args.Intb
	return nil
}

func _assert(condition bool, msg string, v ...any) {
	if !condition {
		s := fmt.Sprintf("assertion failed: "+msg, v...)
		panic(any(s))
	}
}

func TestNewService(t *testing.T) {
	var foo Foo
	s := newService(&foo)
	_assert(len(s.method) == 1, "wrong service Method, expect 1, but got %d", len(s.method))
	mType := s.method["Sum"]
	_assert(mType != nil, "wrong Method, Sum shouldn't nil")
}

func TestMethodTypeCall(t *testing.T) {
	var foo Foo
	s := newService(&foo)
	mType := s.method["Sum"]

	argv := mType.newArgv()
	replyv := mType.newReplyv()
	argv.Set(reflect.ValueOf(Args{Inta: 1, Intb: 3}))
	err := s.call(mType, argv, replyv)
	_assert(err == nil && *replyv.Interface().(*int) == 4 && mType.NumCalls() == 1, "failed to call Foo.Sum")
}
