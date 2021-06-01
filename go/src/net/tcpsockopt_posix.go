// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || windows
// +build aix darwin dragonfly freebsd linux netbsd openbsd solaris windows

package net

import (
	"runtime"
	"syscall"
)

/*
在go中，TCP_NODELAY默认是开启的
TCP_NODELAY：如果开启 TCP_NODELAY 可以禁用Nagle算法，禁用Nagle算法后，数据将尽可能快的将数据发送出去
*/
func setNoDelay(fd *netFD, noDelay bool) error {
	err := fd.pfd.SetsockoptInt(syscall.IPPROTO_TCP, syscall.TCP_NODELAY, boolint(noDelay))
	runtime.KeepAlive(fd)
	return wrapSyscallError("setsockopt", err)
}
