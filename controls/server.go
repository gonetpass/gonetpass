package controls

import (
	"context"
	"log"
	"net"
	"netpass/controls/bytes"
	"netpass/pipe"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Kind string

const (
	Tcp = "tcp"
)

func Run(port string, kind Kind, token string) (err error) {
	hm := &handleContext{
		port:      port,
		forwardCh: make(chan *forward, 1000),
		token:     pipe.Bytes2md5([]byte(token)),
		kind:      kind,
	}
	l, err := net.Listen(string(kind), ":"+port)
	if err != nil {
		return
	}
	defer l.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	log.Println(port, "running")
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn, err := l.Accept()
			if err != nil {
				log.Println(err)
				continue
			}
			go func(conn net.Conn) {
				hm.handle(conn)
			}(conn)
		}
	}
}

func (hm *handleContext) sendString(conn net.Conn, str string) {
	defer conn.Close()
	conn.Write([]byte(str + "\n"))
}

var (
	buffer = pipe.NewBytesPool(pipe.HeaderBufSize)
)

func (hm *handleContext) handle(conn net.Conn) {
	head := buffer.Get().([]byte) //创建一个可回收的bytes
	defer buffer.Put(head)
	n, err := conn.Read(head)
	if err != nil && n == 0 {
		log.Println("read head size", n)
		conn.Close()
		return
	}
	//当读到的大小和bytes一致
	if n == len(head) {
		switch string(head[:n]) {
		//判断是令牌
		case hm.token:
			//进入转发后端
			go func(conn net.Conn) {
				hm.forward(conn)
			}(conn)
			return
			//以下是获取api
		case hm.token[:16] + pipe.Md5CanCount[:16]:
			hm.sendString(conn, strconv.Itoa(int(atomic.LoadInt64(&hm.canCount))))
			return
		case hm.token[:16] + pipe.Md5UseCount[:16]:
			hm.sendString(conn, strconv.Itoa(int(atomic.LoadInt64(&hm.useCount))))
			return
		}
	}
	defer conn.Close()
	if atomic.LoadInt64(&hm.canCount) == 0 {
		log.Println(hm.port, "no available resources")
		return
	}
	//新建一个接收后端的chan
	fConn := make(chan net.Conn, 1)
	ctx, cancel := context.WithCancel(context.Background())
	e := pipe.NewExec()
	f := forward{
		conn: fConn,
		ctx:  ctx,
		e:    e,
	}
	timer := time.NewTimer(pipe.PingPongTime) //获取可用资源超时
	defer e.Exec(func() {
		cancel()
		close(f.conn)
		timer.Stop()
	})
	//发送到handleCh以待 forward 处理
	hm.forwardCh <- &f
	var cc net.Conn
	select {
	case cc = <-f.conn:
		//拿到了forward传过来的conn
		break
	case <-timer.C:
		log.Println(hm.port, "no available resources")
		return
	}
	defer func() {
		if cc != nil {
			cc.Close()
		}
	}()
	atomic.AddInt64(&hm.useCount, 1)
	defer atomic.AddInt64(&hm.useCount, -1)
	connRead := bytes.NewBuffer(time.Minute)
	//创建一个缓冲区用来中转，这个中间层的作用非常大
	//由于之前已经读取了 head 此时conn中数据会丢失这部分，所以我们建立一个中间层
	//可操作性会很大，例如你可以使用 http.ReadRequest...
	_, err = connRead.Write(head[:n])
	if err != nil {
		log.Println("forward write header error", err)
		return
	}
	beat := pipe.NewBeat()
	a := sync.WaitGroup{}
	a.Add(3)
	go func() {
		defer func() {
			cancel()
			a.Done()
		}()
		//把用户数据转发给中间层
		_, _ = pipe.Copy(ctx, connRead, conn, pipe.ReadTimeout, beat)
	}()
	switch hm.kind {
	case Tcp:
		go func() {
			//把后端的数据返回给用户
			defer func() {
				cancel()
				a.Done()
			}()
			_, _ = pipe.Copy(ctx, conn, cc, pipe.ReadTimeout, beat)
		}()
		go func() {
			//中间层数据转发给后端
			defer func() {
				cancel()
				a.Done()
			}()
			_, _ = pipe.Copy(ctx, cc, connRead, pipe.ReadTimeout, beat)
		}()
	default:
		panic(hm.kind)
	}
	a.Wait()
}
