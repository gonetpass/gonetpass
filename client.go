package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"netpass/pipe"
	"strconv"
	"strings"
	"sync"
	"time"
)

type netClient struct {
	ServerAddress string `json:"serverAddress"`
	LocalAddress  string `json:"localAddress"`
	Token         string `json:"token"`
}

func (c netClient) Get(key string) (resp string, err error) {
	conn, err := net.Dial("tcp", c.ServerAddress)
	if err != nil {
		return
	}
	defer conn.Close()
	token := pipe.Bytes2md5([]byte(c.Token))
	_, err = conn.Write([]byte(token[:16] + key[:16]))
	if err != nil {
		return
	}
	e := pipe.NewExec()
	ctx, cancel := context.WithTimeout(context.Background(), pipe.PingPongTime)
	str := make(chan string, 1)
	defer e.Exec(func() {
		cancel()
		close(str)
		for i := 0; i < len(str); i++ {
			_ = <-str
		}
	})
	go func() {
		defer cancel()
		buf := bufio.NewReader(conn)
		resp, err := buf.ReadString('\n')
		if err != nil {
			return
		}
		e.Exec(func() {
			select {
			case <-ctx.Done():
				return
			default:
				str <- strings.TrimSpace(resp)
			}
		})
	}()
	select {
	case <-ctx.Done():
		err = ctx.Err()
		return
	case resp = <-str:
		return
	}
}

var (
	myNetClient = []netClient{}
)

func init() {
	resp, err := ioutil.ReadFile("client.json")
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(resp, &myNetClient)
	if err != nil {
		panic(err)
	}
	if len(myNetClient) == 0 {
		panic("client.json里没数据，请填充")
	}
}

var (
	buffer = pipe.NewBytesPool(pipe.HeaderBufSize)
)

func handShake(c netClient) {
	token := pipe.Bytes2md5([]byte(c.Token))
	//连接本地API
	local, err := net.Dial("tcp", c.LocalAddress)
	if err != nil {
		//log.Println("api", err)
		return
	}
	defer local.Close()
	//连接远程server
	server, err := net.Dial("tcp", c.ServerAddress)
	if err != nil {
		//log.Println("server1", err)
		return
	}
	defer server.Close()
	//发送token
	_, err = server.Write([]byte(token))
	if err != nil {
		//log.Println("server2", err)
		return
	}
	pong := buffer.Get().([]byte)
	defer buffer.Put(pong)
	n := 0
	jump := false
	for !jump {
		n, err = server.Read(pong)
		if err != nil {
			return
		}
		resp := string(pong[:n])
		if err != nil {
			return
		}
		switch resp {
		case pipe.Md5Ping:
			_, err = server.Write([]byte(pipe.Md5Pong))
			if err != nil {
				return
			}
		case token:
			_, err = server.Write([]byte(token))
			if err != nil {
				return
			}
			jump = true
		default:
			return
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := sync.WaitGroup{}
	a.Add(1)
	beat := pipe.NewBeat()
	go func() {
		defer func() {
			cancel()
			a.Done()
		}()
		_, _ = pipe.Copy(ctx, server, local, pipe.ReadTimeout, beat)
	}()
	defer func() {
		cancel()
		a.Wait()
	}()
	_, _ = pipe.Copy(ctx, local, server, pipe.ReadTimeout, beat)
}

func run(c netClient) {
	for {
		resp, err := c.Get(pipe.Md5CanCount)
		if err != nil {
			continue
		}
		canCount, err := strconv.Atoi(resp)
		if err != nil {
			continue
		}
		resp, err = c.Get(pipe.Md5UseCount)
		if err != nil {
			continue
		}
		log.Println(c.ServerAddress, "canCount：", canCount, "useCount：", resp)
		for i := pipe.AddCanCount - canCount; i > 0; i-- {
			go handShake(c)
		}
		time.Sleep(time.Second)
	}
}

func main() {
	a := sync.WaitGroup{}
	a.Add(len(myNetClient))
	for _, c := range myNetClient {
		go func(c netClient) {
			defer a.Done()
			run(c)
		}(c)
	}
	a.Wait()
}
