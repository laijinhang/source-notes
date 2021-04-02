package main

import "fmt"

func init() {
	fmt.Print("Content-Type: text/html;charset=utf-8\n\n")
}

func main() {
	fmt.Println("cgi实例")
}
