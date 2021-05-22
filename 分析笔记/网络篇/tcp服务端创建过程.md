### 一、一个简单的TCP服务端案例

```go
package main

import "net"

func main() {
	s, err := net.Listen("tcp", ":10000")
	if err != nil {
		panic(err)
	}
	defer s.Close()
	for {
		c, err := s.Accept()
		if err != nil {
			continue
		}
		go func(conn net.Conn) {
			
		}(c)
	}
}
```

### 二、Listen

##### 1. 解析network

```
一共有三种格式：
"tcp","tcp4", "tcp6"，"udp","udp4","udp6"，"unix", "unixgram", "unixpacket"

"ip:xxxxx","ip4:xxxxx","ip6:xxxxx"
xxxxx 经过解析之后必须是 >= 0 && < 65535


其他的都是错误格式
```

##### 2. 解析addr



##### 3. 调用对应平台的socket api创建socket