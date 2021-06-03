// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"sync/atomic"
	"unsafe"
)

/*
Cond是实现在go标准库中的一个条件变量的实现，常用于一个或一组goroutine等待满足某个条件后唤醒的场景，如常见的生产者消费者场景
我发现下面代码在执行中，并没有panic，go run是正常运行的，go vet的时候会提示 assignment copies lock value to cond: sync.Cond contains sync.noCopy
```go
func main() {
	lock := new(sync.Mutex)
	cond1 := sync.NewCond(lock)
	cond := *cond1
	for i := 0;i < 10;i++ {
		go func(x int) {
			cond.L.Lock()		// 获取锁
			defer cond.L.Unlock()	// 释放锁
			cond.Wait()	// 等待通知，阻塞当前goroutine
			fmt.Println(x)
			time.Sleep(time.Second)
		}(i)
	}
	time.Sleep(time.Second)
	fmt.Println("Signal...")
	cond.Signal()	// 发送一个通知给已经获取锁的goroutine
	time.Sleep(time.Second * 3)
	cond.Signal()
	time.Sleep(3 * time.Second)
	cond.Broadcast()
	time.Sleep(10 * time.Second)
}
```
两篇不错的文章：
* Golang sync.Cond 条件变量源码分析：https://www.cyhone.com/articles/golang-sync-cond/
* sync.Cond源码分析：https://reading.hidevops.io/articles/sync/sync_cond_source_code_analysis/
*/
// Cond implements a condition variable, a rendezvous point
// for goroutines waiting for or announcing the occurrence
// of an event.
//
// Each Cond has an associated Locker L (often a *Mutex or *RWMutex),
// which must be held when changing the condition and
// when calling the Wait method.
//
// A Cond must not be copied after first use.
// 一个Cond在被使用之后，不能被复制。
type Cond struct {
	noCopy noCopy

	// L is held while observing or changing the condition
	// 根据需求初始化不同的锁，如 *Mutex 和 *RWMutex
	L Locker

	notify  notifyList  // 通知列表，调用wait()方法的goroutine会被放入list中，每次唤醒，从这里取出
	checker copyChecker // 复制检查，检查cond实例是否被复制
}

// NewCond returns a new Cond with Locker l.
// NewCond返回一个新的带Locker l的指针Cond
func NewCond(l Locker) *Cond {
	return &Cond{L: l}
}

// Wait atomically unlocks c.L and suspends execution
// of the calling goroutine. After later resuming execution,
// Wait locks c.L before returning. Unlike in other systems,
// Wait cannot return unless awoken by Broadcast or Signal.
//
// Because c.L is not locked when Wait first resumes, the caller
// typically cannot assume that the condition is true when
// Wait returns. Instead, the caller should Wait in a loop:
//
//    c.L.Lock()
//    for !condition() {
//        c.Wait()
//    }
//    ... make use of condition ...
//    c.L.Unlock()
//
func (c *Cond) Wait() {
	// 检查c是否被复制的，如果是就panic
	c.checker.check()
	// 将当前goroutine加入等待对了
	t := runtime_notifyListAdd(&c.notify)
	// 解锁
	c.L.Unlock()
	// 等待队列中所有的goroutine执行等待唤醒操作
	runtime_notifyListWait(&c.notify, t)
	c.L.Lock()
}

// Signal wakes one goroutine waiting on c, if there is any.
//
// It is allowed but not required for the caller to hold c.L
// during the call.
func (c *Cond) Signal() {
	// 检查c是否是复制的，如果是就panic
	c.checker.check()
	// 通知等待列中的一个
	runtime_notifyListNotifyOne(&c.notify)
}

// Broadcast wakes all goroutines waiting on c.
//
// It is allowed but not required for the caller to hold c.L
// during the call.
// 唤醒等待队列中的所有goroutine
func (c *Cond) Broadcast() {
	// 检查c是否是被复制的，如果是就panic
	c.checker.check()
	runtime_notifyListNotifyAll(&c.notify)
}

// copyChecker holds back pointer to itself to detect object copying.
// 判断cond是否被复制
type copyChecker uintptr

func (c *copyChecker) check() {
	if uintptr(*c) != uintptr(unsafe.Pointer(c)) &&
		!atomic.CompareAndSwapUintptr((*uintptr)(c), 0, uintptr(unsafe.Pointer(c))) &&
		uintptr(*c) != uintptr(unsafe.Pointer(c)) {
		panic("sync.Cond is copied")
	}
}

// noCopy may be embedded into structs which must not be copied
// after the first use.
//
// See https://golang.org/issues/8005#issuecomment-190753527
// for details.
type noCopy struct{}

// Lock is a no-op used by -copylocks checker from `go vet`.
func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}
