package main

import (
	"fmt"
	"index/suffixarray"
)

func main()  {
	data := []byte("abcdefghaaaa")
	// 创建数据的索引
	index := suffixarray.New(data)
	// 查找切片s
	s := []byte{'a'}
	offsets1 := index.Lookup(s, -1)
	fmt.Println("全部结果：", offsets1)
	offsets2 := index.Lookup(s, 3)
	fmt.Println("期望三个结果：", offsets2, "，实际结果个数：", len(offsets2))
}