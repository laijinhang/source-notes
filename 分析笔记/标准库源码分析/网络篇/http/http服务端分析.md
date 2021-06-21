# 1、一个简单的案例

```go
package main

import (
	"net/http"
)

func main() {
	http.HandleFunc("/index", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("hello"))
	})
	http.ListenAndServe(":8000", nil)
}
```

### 1. 注册路由

```go
http路由有两种比较经典的实现：基数树 或 哈希映射

go源码中路由是使用 哈希映射 实现
```

### 2. 监听端口

```go

```

### 3. 等待请求

### 4. 处理请求

