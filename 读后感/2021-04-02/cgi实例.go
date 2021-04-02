package main

import (
	"net/http"
	"net/http/cgi"
	"os"
)

// 测试：在浏览器中输入 127.0.0.1:10001/1.go
func main() {
	http.HandleFunc("/", Hanlder)
	http.ListenAndServe(":10001", nil)
}

func Hanlder(w http.ResponseWriter, req *http.Request) {
	var dir = "要执行的go文件的绝对路径"
	cgi_obj := new(cgi.Handler)
	// 设置CGI运行目录
	cgi_obj.Path = os.Getenv("$GOROOT") + "/bin/go"
	// 设置脚本目录
	script := dir + req.URL.Path
	// 设置CGI可执行文件的工作目录
	cgi_obj.Dir = dir
	args := []string{"run", script}
	// 设置 传递给子进程的可选参数
	cgi_obj.Args = append(cgi_obj.Args, args...)
	// 设置GOPATH目录，如果没有 会报go run: no go files listed 但是不影响运行
	cgi_obj.Env = append(cgi_obj.Env, "GOPATH=/home/laijh/go")
	// 设置GOROOT目录
	cgi_obj.Env = append(cgi_obj.Env, "GOROOT=/home/laijh/go/go1.16")
	cgi_obj.Env = append(cgi_obj.Env, "GOCACHE=/home/laijh/.cache/go-build") // 终端先执行go env，把GOCACHE对应值考过来
	// 启动http server
	cgi_obj.ServeHTTP(w, req)
}
