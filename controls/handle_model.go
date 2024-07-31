package controls

import (
	"context"
	"net"
	"netpass/pipe"
)

type handleContext struct {
	port      string        //端口
	kind      Kind          //转发类型
	token     string        //端口的token，对应client里的token
	forwardCh chan *forward //forward的通道
	canCount  int64         //可用数量
	useCount  int64         //使用中的数量
}

type forward struct {
	conn chan net.Conn
	ctx  context.Context
	e    *pipe.Exec
}
