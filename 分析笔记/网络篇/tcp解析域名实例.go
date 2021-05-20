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
