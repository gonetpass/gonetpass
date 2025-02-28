package controls

import (
	"errors"
	"io"
	"log"
	"net"
	"netpass/pipe"
	"sync"
	"sync/atomic"
	"time"
)

func (c *Context) forwardResponse(conn net.Conn, w []byte) (ans string, err error) {
	_, err = conn.Write(w)
	if err != nil {
		return
	}
	ch := make(chan []byte)
	complete := int32(0)
	a := sync.WaitGroup{}
	a.Add(1)
	go func() {
		buf := c.headBuffer.Get()
		defer c.headBuffer.Put(buf)
		defer func() {
			close(ch)
			a.Done()
		}()
		n := 0
		n, err = io.ReadFull(conn, buf)
		if err != nil || atomic.LoadInt32(&complete) > 0 || n != cap(buf) {
			return
		}
		p := c.headBuffer.Get()
		copy(p, buf[:n])
		ch <- p
	}()
	timeOut := time.NewTimer(pipe.PingPongTime)
	defer func() {
		atomic.AddInt32(&complete, 1)
		timeOut.Stop()
		a.Wait()
		for i := 0; i < len(ch); i++ {
			p := <-ch
			c.headBuffer.Put(p)
		}
	}()
	select {
	case p := <-ch:
		ans = string(p)
		c.headBuffer.Put(p)
		return
	case <-timeOut.C:
		err = errors.New("recover time out")
		return
	}
}

func (c *Context) forward(conn net.Conn) {
	atomic.AddInt64(&c.canCount, 1)
	defer atomic.AddInt64(&c.canCount, -1)
	var err error
	defer func() {
		if err != nil {
			log.Println(err)
			conn.Close()
		}
	}()
	pingTick := time.NewTicker(pipe.PingPongTime)
	defer pingTick.Stop()
	resp := ""
	for {
		select {
		case f := <-c.forwardCh:
			done := false
			f.e.Exec(func() {
				select {
				case <-f.ctx.Done():
					done = true
				default:
				}
			})
			if done {
				//h已经done 继续等待下一个h
				continue
			}
			resp, err = c.forwardResponse(conn, []byte(c.token))
			if err != nil || resp != c.token {
				if err == nil {
					err = errors.New("token error")
				}
				return
			}
			f.e.Exec(func() {
				select {
				case <-f.ctx.Done():
					err = f.ctx.Err()
				default:
					f.conn <- conn
				}
			})
			return
		case <-pingTick.C:
			resp, err = c.forwardResponse(conn, []byte(pipe.Md5Ping))
			if err != nil || resp != pipe.Md5Pong {
				if err == nil {
					err = errors.New("pong error")
				}
				return
			}
		}
	}
}
