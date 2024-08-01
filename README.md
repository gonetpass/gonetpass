## golang开发的内网穿透，轻量级，占用内存小，性能好，易搭建，支持绑定域名，自动https 
   
1.download zip

2.go mod init netpass

  go mod tidy  
  go mod vendor  
  go build server.go  
  go build client.go  
  go build autohttps.go  
  
3.配置文件  
  server.json  端口对应token(client连接server的时候使用的token)
  {   
  "10011": "jxasidqwieiqwoej",  
  "443":   "jxasidqwieiqwoej"  
  }  
  client.json  //这是一个数组，每个obj连接一个server  
  [  
  {  
    "serverAddress": "192.168.0.1:8080",  
    "localAddress":  "127.0.0.1:8080",  
    "token":         "jxasidqwieiqwoej"  
  }  
  ]  
  若要使用https进行传输,只需要域名解析到server的ip,并且连接到443端口，本地运行autohttps(默认端口为8086),只需要在autohttps里把反向代理的地址改为你的api即可，无需其他配置  
  [  
  {  
    "serverAddress": "192.168.0.1:443",  
    "localAddress":  "127.0.0.1:8086",  
    "token":         "jxasidqwieiqwoej"  
  }  
  ]  
