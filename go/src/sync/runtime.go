// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import "unsafe"

// defined in package runtime
// 定义在包的运行时间

// Semacquire waits until *s > 0 and then atomically decrements it.
// It is intended as a simple sleep primitive for use by the synchronization
// library and should not be used directly.
// Semacquire一直等待到*s>0，然后原子式地递减它。
// 它的目的是作为一个简单的睡眠原语，供同步库使用，不应直接使用。
func runtime_Semacquire(s *uint32)

// SemacquireMutex is like Semacquire, but for profiling contended Mutexes.
// If lifo is true, queue waiter at the head of wait queue.
// skipframes is the number of frames to omit during tracing, counting from
// runtime_SemacquireMutex's caller.
// SemacquireMutex与Semacquire相似，但用于分析有争议的Mutex。
// 如果lifo为真，则在等待队列中排队等待。
// skipframes是追踪过程中要省略的帧数，从runtime_SemacquireMutex的调用者开始计算。
func runtime_SemacquireMutex(s *uint32, lifo bool, skipframes int)

// Semrelease atomically increments *s and notifies a waiting goroutine
// if one is blocked in Semacquire.
// It is intended as a simple wakeup primitive for use by the synchronization
// library and should not be used directly.
// If handoff is true, pass count directly to the first waiter.
// skipframes is the number of frames to omit during tracing, counting from
// runtime_Semrelease's caller.
// Semrelease以原子方式增加*s，如果Semacquire中的一个goroutine被阻塞，则通知一个等待的goroutine。
// 它旨在作为一个简单的唤醒原语，供同步库使用，不应直接使用。如果handoff为真，则直接将计数传递给第一个等待者。
// skipframes是在跟踪过程中要省略的帧数，从runtime_Semrelease的调用者算起。
func runtime_Semrelease(s *uint32, handoff bool, skipframes int)

// See runtime/sema.go for documentation.
func runtime_notifyListAdd(l *notifyList) uint32

// See runtime/sema.go for documentation.
func runtime_notifyListWait(l *notifyList, t uint32)

// See runtime/sema.go for documentation.
func runtime_notifyListNotifyAll(l *notifyList)

// See runtime/sema.go for documentation.
func runtime_notifyListNotifyOne(l *notifyList)

// Ensure that sync and runtime agree on size of notifyList.
func runtime_notifyListCheck(size uintptr)
func init() {
	var n notifyList
	runtime_notifyListCheck(unsafe.Sizeof(n))
}

// Active spinning runtime support.
// runtime_canSpin reports whether spinning makes sense at the moment.
func runtime_canSpin(i int) bool

// runtime_doSpin does active spinning.
func runtime_doSpin()

// 返回运行时时钟的当前值，单位为纳秒
func runtime_nanotime() int64
