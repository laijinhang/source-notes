# 1、linux平台
```go
package main

import (
	"fmt"
	"net"
)

func main() {
	d, err := net.Dial("tcp", "www.baidu.com:80")
	if err != nil {
		panic(err)
	}
	defer d.Close()
	fmt.Println(d.RemoteAddr())
}
```
通过对上面的代码进行单步调试，定位到域名解析核心的实现方法：net/lookup_unix.go lookupIP
1. 对于使用cgo进行解析的实现：net/cgo_unix.go cgoLookupIP
2. 对于非cgo进行解析的实现：net/dnsclient_unix.go Resolver对象的goLookupIPCNAMEOrder方法

### 1. cgo解析
```go
func cgoLookupIP(ctx context.Context, network, name string) (addrs []IPAddr, err error, completed bool) {
	if ctx.Done() == nil {
		addrs, _, err = cgoLookupIPCNAME(network, name)
		return addrs, err, true
	}
	result := make(chan ipLookupResult, 1)
	go cgoIPLookup(result, network, name)
	select {
	case r := <-result:
		return r.addrs, r.err, true
	case <-ctx.Done():
		return nil, mapErr(ctx.Err()), false
	}
}
```