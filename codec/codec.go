package codec

import "io"

type Header struct {
	// ServiceMethod denotes service name and method name
	ServiceMethod string
	// Seq denotes the number id of request
	Seq   uint64
	Error string
}

// Codec is a interface used to encode and decode message body
type Codec interface {
	io.Closer
	ReadHeader(*Header) error
	ReadBody(any) error
	Write(*Header, any) error
}

type NewCodecFunc func(io.ReadWriteCloser) Codec

type Type string

const (
	GobType  Type = "application/gob"
	JsonType Type = "application/json"
)

var NewCodecFuncMap map[Type]NewCodecFunc

func init() {
	NewCodecFuncMap = make(map[Type]NewCodecFunc)
	NewCodecFuncMap[GobType] = NewGobCodec
}
