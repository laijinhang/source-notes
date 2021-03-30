// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build aix darwin dragonfly freebsd linux netbsd openbsd windows solaris

package poll

import (
	"errors"
	"sync"
	"syscall"
	"time"
	_ "unsafe" // for go:linkname
)

// runtimeNano返回运行时时钟的当前值（以纳秒为单位）。
// runtimeNano returns the current value of the runtime clock in nanoseconds.
//go:linkname runtimeNano runtime.nanotime
func runtimeNano() int64

func runtime_pollServerInit()
func runtime_pollOpen(fd uintptr) (uintptr, int)
func runtime_pollClose(ctx uintptr)
func runtime_pollWait(ctx uintptr, mode int) int
func runtime_pollWaitCanceled(ctx uintptr, mode int) int
func runtime_pollReset(ctx uintptr, mode int) int
func runtime_pollSetDeadline(ctx uintptr, d int64, mode int)
func runtime_pollUnblock(ctx uintptr)
func runtime_isPollServerDescriptor(fd uintptr) bool

type pollDesc struct {
	runtimeCtx uintptr // 运行时上下文
}

var serverInit sync.Once

// epoll初始化
func (pd *pollDesc) init(fd *FD) error {
	// 保证最多只被调用一次，调用runtime_pollServerInit创建一个epollServerInit创建一个epoll句柄，
	// 实际执行的就是epoll_create
	serverInit.Do(runtime_pollServerInit)
	// 调用runtime_pollOpen将文件描述符fd.Sysfd注册到epoll句柄中，实际执行的就是epoll_ctl。
	// 此处返回的ctx为pollDesc结构体（src/runtime/netpoll.go）
	ctx, errno := runtime_pollOpen(uintptr(fd.Sysfd))
	if errno != 0 {
		// 处理失败需要做如下动作：
		//	1：unblock阻塞的goroutine
		//	2：回收pd
		//	3：close fd，文件描述符在init函数返回失败后close
		if ctx != 0 {
			// 在注册epoll事件失败后，需要unblock goroutine，继续执行清理工作
			runtime_pollUnblock(ctx)
			// 此处删除注册的事件并回收pd节点
			runtime_pollClose(ctx)
		}
		return errnoErr(syscall.Errno(errno))
	}
	// 保存pollDesc
	pd.runtimeCtx = ctx
	return nil
}

func (pd *pollDesc) close() {
	/*
		runtimeCtx的类型是uintptr，如果没有初始化，它的值是零值，
		也就是说，如果是runtimeCtx的值是初始值，则直接返回
	*/
	if pd.runtimeCtx == 0 {
		return
	}
	runtime_pollClose(pd.runtimeCtx)
	pd.runtimeCtx = 0
}

// Evict evicts fd from the pending list, unblocking any I/O running on fd.
// 逐出从挂起列表中逐出fd，解除对fd上运行的任何I/O的阻止。
func (pd *pollDesc) evict() {
	if pd.runtimeCtx == 0 {
		return
	}
	runtime_pollUnblock(pd.runtimeCtx)
}

func (pd *pollDesc) prepare(mode int, isFile bool) error {
	if pd.runtimeCtx == 0 {
		return nil
	}
	res := runtime_pollReset(pd.runtimeCtx, mode)
	return convertErr(res, isFile)
}

// fd.pd.prepareRead 检查当前fd是否允许accept，
// 实际上是检查更底层的 pollDesc 是否可读。
// 检查完毕之后，尝试调用 accept 获取已连接的socket，注意此待代码在for循环内，
// 说明 Accept 是阻塞的，直到有连接进来；当遇到 EAGIN 和 ECONNABORTED 错误
// 会重试，其他错误都抛给更上一层。
func (pd *pollDesc) prepareRead(isFile bool) error {
	return pd.prepare('r', isFile)
}

func (pd *pollDesc) prepareWrite(isFile bool) error {
	return pd.prepare('w', isFile)
}

func (pd *pollDesc) wait(mode int, isFile bool) error {
	if pd.runtimeCtx == 0 {
		return errors.New("waiting for unsupported file type")
	}
	res := runtime_pollWait(pd.runtimeCtx, mode)
	return convertErr(res, isFile)
}

// fd.pd.waitRead阻塞等待fd是否可读，即是否有新连接进来，
// 最终进入到runtime.poll_runtime_pollWait里(runtime/netpoll.go)
func (pd *pollDesc) waitRead(isFile bool) error {
	return pd.wait('r', isFile)
}

func (pd *pollDesc) waitWrite(isFile bool) error {
	return pd.wait('w', isFile)
}

func (pd *pollDesc) waitCanceled(mode int) {
	if pd.runtimeCtx == 0 {
		return
	}
	runtime_pollWaitCanceled(pd.runtimeCtx, mode)
}

func (pd *pollDesc) pollable() bool {
	return pd.runtimeCtx != 0
}

// Error values returned by runtime_pollReset and runtime_pollWait.
// These must match the values in runtime/netpoll.go.
const (
	pollNoError        = 0
	pollErrClosing     = 1
	pollErrTimeout     = 2
	pollErrNotPollable = 3
)

func convertErr(res int, isFile bool) error {
	switch res {
	case pollNoError:
		return nil
	case pollErrClosing:
		return errClosing(isFile)
	case pollErrTimeout:
		return ErrDeadlineExceeded
	case pollErrNotPollable:
		return ErrNotPollable
	}
	println("unreachable: ", res)
	panic("unreachable")
}

// SetDeadline sets the read and write deadlines associated with fd.
func (fd *FD) SetDeadline(t time.Time) error {
	return setDeadlineImpl(fd, t, 'r'+'w')
}

// SetReadDeadline sets the read deadline associated with fd.
func (fd *FD) SetReadDeadline(t time.Time) error {
	return setDeadlineImpl(fd, t, 'r')
}

// SetWriteDeadline sets the write deadline associated with fd.
func (fd *FD) SetWriteDeadline(t time.Time) error {
	return setDeadlineImpl(fd, t, 'w')
}

func setDeadlineImpl(fd *FD, t time.Time, mode int) error {
	var d int64
	if !t.IsZero() {
		d = int64(time.Until(t))
		if d == 0 {
			d = -1 // don't confuse deadline right now with no deadline
		}
	}
	if err := fd.incref(); err != nil {
		return err
	}
	defer fd.decref()
	if fd.pd.runtimeCtx == 0 {
		return ErrNoDeadline
	}
	runtime_pollSetDeadline(fd.pd.runtimeCtx, d, mode)
	return nil
}

// IsPollDescriptor reports whether fd is the descriptor being used by the poller.
// This is only used for testing.
func IsPollDescriptor(fd uintptr) bool {
	return runtime_isPollServerDescriptor(fd)
}
