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
	c := &Context{
		port:       port,
		forwardCh:  make(chan *forward, 1000),
		token:      pipe.Bytes2md5([]byte(token)),
		kind:       kind,
		headBuffer: pipe.NewBytesPool(pipe.HeaderBufSize),
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
				c.handle(conn)
			}(conn)
		}
	}
}

func (c *Context) sendString(conn net.Conn, str string) {
	defer conn.Close()
	conn.Write([]byte(str + "\n"))
}

func (c *Context) handle(conn net.Conn) {
	head := c.headBuffer.Get() //创建一个可回收的bytes
	defer c.headBuffer.Put(head)
	n, err := conn.Read(head)
	if err != nil && n == 0 {
		log.Println("read head size", n)
		conn.Close()
		return
	}
	//当读到的大小和bytes一致
	if n == cap(head) {
		switch string(head[:n]) {
		//判断是令牌
		case c.token:
			//进入转发后端
			go func(conn net.Conn) {
				c.forward(conn)
			}(conn)
			return
			//以下是获取api
		case c.token[:16] + pipe.Md5CanCount[:16]:
			c.sendString(conn, strconv.Itoa(int(atomic.LoadInt64(&c.canCount))))
			return
		case c.token[:16] + pipe.Md5UseCount[:16]:
			c.sendString(conn, strconv.Itoa(int(atomic.LoadInt64(&c.useCount))))
			return
		}
	}
	defer conn.Close()
	if atomic.LoadInt64(&c.canCount) == 0 {
		log.Println(c.port, "no available resources")
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
	c.forwardCh <- &f
	var cc net.Conn
	select {
	case cc = <-f.conn:
		//拿到了forward传过来的conn
		break
	case <-timer.C:
		log.Println(c.port, "no available resources")
		return
	}
	defer func() {
		if cc != nil {
			cc.Close()
		}
	}()
	atomic.AddInt64(&c.useCount, 1)
	defer atomic.AddInt64(&c.useCount, -1)
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
	switch c.kind {
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
		panic(c.kind)
	}
	a.Wait()
}
