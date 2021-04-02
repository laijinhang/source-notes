package main

import (
	"fmt"
	"net"
)

func main() {
	conn, err := net.Dial("tcp", "127.0.0.1:10001")
	if err != nil {
		panic(err)
	}
	read := func() {
		buf := make([]byte, 1024)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				panic(err)
			}
			fmt.Println("服务端>>", string(buf[:n]))
		}
	}
	write := func() {
		buf := []byte{}
		for {
			fmt.Print("请输入要发送的数据：")
			fmt.Scan(&buf)
			n, err := conn.Write(buf)
			if err != nil {
				panic(err)
			}
			fmt.Println("客户端>>", string(buf[:n]))
		}
	}

	go read()
	write()
}
