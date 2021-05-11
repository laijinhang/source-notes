package main

import (
	"fmt"
	"time"
)

func main() {
	t1 := time.Now()
	t2 := t1.Add(10 * time.Second)
	t3 := t1
	fmt.Println(t1, t2)
	fmt.Println(t1.After(t2))  // t1是否大于t2
	fmt.Println(t1.Before(t2)) // t1是否小于t2
	fmt.Println(t1.After(t3))  // t1是否大于t3
	fmt.Println(t1.Before(t3)) // t1是否小于t3
}
