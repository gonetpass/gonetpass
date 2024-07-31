package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"netpass/controls"
)

var myToken = map[string]string{}

//端口对应的token，对应client的token

func init() {
	resp, err := ioutil.ReadFile("server.json")
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(resp, &myToken)
	if err != nil {
		panic(err)
	}
	if len(myToken) == 0 {
		panic("server.json里没数据，请填充")
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	for port, token := range myToken {
		go func(port, token string) {
			defer cancel()
			controls.Run(port, controls.Tcp, token)
		}(port, token)
	}
	<-ctx.Done()
}
