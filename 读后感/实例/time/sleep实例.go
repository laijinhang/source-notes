package main

import (
	"fmt"
	"time"
)

func main() {
	runTask()
}

// 每秒执行一次，100秒后退出
func runTask() {
	t := time.NewTimer(time.Second)
	go func() {
		for {
			select {
			case now := <-t.C:
				fmt.Println("task，time:", now)
			}
			t.Reset(time.Second)
		}
	}()
	time.Sleep(100 * time.Second)
}
