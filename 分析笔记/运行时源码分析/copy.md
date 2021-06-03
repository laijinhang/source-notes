# 一、使用

### 1. 切片

```go
package main

import "fmt"

func main() {
	slice1 := []byte("test")
	slice2 := make([]byte, len(slice1)*2)
	n := copy(slice2, slice1)
	fmt.Println(string(slice1) == string(slice2[:n]))

	copy(slice2[n:], slice2)
	fmt.Println(string(slice2))
}
```

### 2. 数组

```go
package main

import "fmt"

func main() {
	slice1 := []byte("test")
	var array1 [10]byte
	n := copy(array1[:], slice1)
	fmt.Println(string(slice1) == string(array1[:n]))

	s1 := "test2"
	n = copy(array1[:], s1)
	fmt.Println(s1 == string(array1[:n]))
}
```
