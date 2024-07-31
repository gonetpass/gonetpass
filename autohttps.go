package main

import (
	"bufio"
	"github.com/gin-gonic/gin"
	_ "github.com/gin-gonic/gin"
	"io"
	"log"
	"net"
	"net/http"
	"netpass/autohttps/autotls"
)

func main() {
	//https默认端口8086在 autohttps/autoconfig/bind.go修改
	r := gin.Default()
	// 定义目标服务器的URL
	//反向代理
	api := ":10011" //api端口
	r.Any("/*url", func(c *gin.Context) {
		var err error
		defer func() {
			if err != nil {
				log.Println(err)
			}
		}()
		conn, err := net.Dial("tcp", api)
		if err != nil {
			return
		}
		defer conn.Close()
		err = c.Request.Write(conn)
		if err != nil {
			return
		}
		res, err := http.ReadResponse(bufio.NewReader(conn), c.Request)
		if err != nil {
			return
		}
		defer res.Body.Close()
		c.Writer.WriteHeader(res.StatusCode)
		for key, values := range res.Header {
			for _, value := range values {
				c.Writer.Header().Add(key, value)
			}
		}
		_, _ = io.Copy(c.Writer, res.Body)
	})
	log.Fatal(autotls.Run(r, "haha.com"))
}
