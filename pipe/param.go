package pipe

import (
	"crypto/md5"
	"fmt"
	"time"
)

const (
	HeaderBufSize = 32
	ClientNum     = 512
	CopyBuf       = 1024 * 32
	Ping          = "ping"
	Pong          = "pong"
	PingPongTime  = time.Second * 10
	ReadTimeout   = time.Minute
	CanCount      = "canCount"
	AddCanCount   = 20
	UseCount      = "useCount"
)

var (
	Md5Ping     = Bytes2md5([]byte(Ping))
	Md5Pong     = Bytes2md5([]byte(Pong))
	Md5CanCount = Bytes2md5([]byte(CanCount))
	Md5UseCount = Bytes2md5([]byte(UseCount))
)

func Bytes2md5(data []byte) string {
	has := md5.Sum(data)
	return fmt.Sprintf("%x", has)
}
