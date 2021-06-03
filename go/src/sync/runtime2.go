// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !goexperiment.staticlockranking
// +build !goexperiment.staticlockranking

package sync

import "unsafe"

// Approximation of notifyList in runtime/sema.go. Size and alignment must
// agree.
type notifyList struct {
	// 等待数量
	wait uint32
	// 通知数量
	notify uint32
	// 锁
	lock uintptr // key field of the mutex
	// 链表头
	head unsafe.Pointer
	// 链表尾
	tail unsafe.Pointer
}
