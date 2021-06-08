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

* linux：epoll
* freeBSD/MacOS：kqueue
* windows：iocp

##### 4. 设置socket选项

```go
if err = setDefaultSockopts(s, family, sotype, ipv6only); err != nil {
    poll.CloseFunc(s)
    return nil, err
}
```

##### 5. 创建网络描述符
**linux：**
```go
net/fd_unix.go

func newFD(sysfd, family, sotype int, net string) (*netFD, error) {
	ret := &netFD{
		pfd: poll.FD{
			Sysfd:         sysfd,
			IsStream:      sotype == syscall.SOCK_STREAM,
			ZeroReadIsEOF: sotype != syscall.SOCK_DGRAM && sotype != syscall.SOCK_RAW,
		},
		family: family,
		sotype: sotype,
		net:    net,
	}
	return ret, nil
}
```
##### 6. 监听端口

```go
在linux中，在程序第一次调用时，会先去读 /proc/sys/net/core/somaxconn 文件中记录的最大可连接数，如果读取这个文件失败，则使用默认大小128作为最大可连接数
```

# 三、Accept

```go
有新的连接进来后，会创建一个新的fd，与客户端进行通信，后面进行write和read都是通过这个fd进行交互的
```

之后会阻塞，直到有请求连接进来

linux

src/internal/poll/fd_unix.go
```go
func (fd *FD) Accept() (int, syscall.Sockaddr, string, error) {
	// 获取读锁
	if err := fd.readLock(); err != nil {
		return -1, nil, "", err
	}
	defer fd.readUnlock()

	// fd.pd.prepareRead 检查当前fd是否允许accept，
	// 实际上是检查更底层的 pollDesc 是否可读。
	// 检查完毕之后，尝试调用 accept 获取已连接的socket，注意此待代码在for循环内，
	// 说明 Accept 是阻塞的，直到有连接进来；当遇到 EAGIN 和 ECONNABORTED 错误
	// 会重试，其他错误都抛给更上一层。
	if err := fd.pd.prepareRead(fd.isFile); err != nil {
		return -1, nil, "", err
	}
	for {
		s, rsa, errcall, err := accept(fd.Sysfd)
		if err == nil {
			return s, rsa, "", err
		}
		switch err {
		case syscall.EINTR:
			continue
		case syscall.EAGAIN:
			if fd.pd.pollable() {
				if err = fd.pd.waitRead(fd.isFile); err == nil {
					continue
				}
			}
		case syscall.ECONNABORTED:
			// This means that a socket on the listen
			// queue was closed before we Accept()ed it;
			// it's a silly error, so try again.
			continue
		}
		return -1, nil, errcall, err
	}
}
```

# 四、Write

# 五、Read

