// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements sysSocket and accept for platforms that
// provide a fast path for setting SetNonblock and CloseOnExec.

// +build dragonfly freebsd illumos linux netbsd openbsd

package net

import (
	"internal/poll"
	"os"
	"syscall"
)

// Wrapper around the socket system call that marks the returned file
// descriptor as nonblocking and close-on-exec.
// 围绕套接字系统调用的封装器，将返回的文件描述符标记为非阻塞和执行时关闭。
func sysSocket(family, sotype, proto int) (int, error) {
	/*
		syscall.SOCK_NONBLOCK：指定异步的，非阻塞
		syscall.SOCK_CLOEXEC：fork时关闭
	*/
	s, err := socketFunc(family, sotype|syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC, proto)
	// On Linux the SOCK_NONBLOCK and SOCK_CLOEXEC flags were
	// introduced in 2.6.27 kernel and on FreeBSD both flags were
	// introduced in 10 kernel. If we get an EINVAL error on Linux
	// or EPROTONOSUPPORT error on FreeBSD, fall back to using
	// socket without them.
	// 在Linux上，SOCK_NONBLOCK和SOCK_CLOEXEC标志是在2.6.27内核中引入的，
	// 在FreeBSD上，这两个标志是在10内核中引入的。如果我们在Linux上遇到EINVAL错误，
	// 或者在FreeBSD上遇到EPROTONOSUPPORT错误，就回过头来使用没有它们的socket。
	switch err {
	case nil:
		return s, nil
	default:
		return -1, os.NewSyscallError("socket", err)
	case syscall.EPROTONOSUPPORT, syscall.EINVAL:
	}

	// See ../syscall/exec_unix.go for description of ForkLock.
	syscall.ForkLock.RLock()
	s, err = socketFunc(family, sotype, proto)
	if err == nil {
		syscall.CloseOnExec(s)
	}
	syscall.ForkLock.RUnlock()
	if err != nil {
		return -1, os.NewSyscallError("socket", err)
	}
	if err = syscall.SetNonblock(s, true); err != nil {
		poll.CloseFunc(s)
		return -1, os.NewSyscallError("setnonblock", err)
	}
	return s, nil
}
