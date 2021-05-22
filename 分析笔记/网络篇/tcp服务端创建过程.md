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
xxxxx 经过解析之后必须是 >= 0 && < 65535 或者 对应平台支持协议（如linux中查看/etc/protocols，支持那些协议）


其他的都是错误格式
```

##### 2. 解析addr



##### 3. 调用对应平台的socket api创建socket

```go
s, err := sysSocket(family, sotype, proto)
```

##### 4. 设置socket选项

```go
if err = setDefaultSockopts(s, family, sotype, ipv6only); err != nil {
    poll.CloseFunc(s)
    return nil, err
}
```

##### 5. 创建网络描述符

##### 6. 监听端口

```go
在linux中，在程序第一次调用时，会先去读 /proc/sys/net/core/somaxconn 文件中记录的最大可连接数，如果读取这个文件失败，则使用默认大小128作为最大可连接数
```

# 三、Accept

# 四、Write

# 五、Read

