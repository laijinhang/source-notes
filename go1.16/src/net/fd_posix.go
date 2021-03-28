// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build aix darwin dragonfly freebsd linux netbsd openbsd solaris windows

package net

import (
	"internal/poll"
	"runtime"
	"syscall"
	"time"
)

// Network file descriptor.
// 网络文件描述符
type netFD struct {
	pfd poll.FD

	// immutable until Close
	// 直到关闭都是不可变的
	family      int
	sotype      int
	isConnected bool // handshake completed or use of association with peer，握手完成或使用与对等方的关联
	net         string
	laddr       Addr // 本地地址
	raddr       Addr // 远程地址
}

// 设置地址
func (fd *netFD) setAddr(laddr, raddr Addr) {
	fd.laddr = laddr
	fd.raddr = raddr
	// 在fd这个文件描述符被关闭之前，执行(*netFD).Close这个函数
	runtime.SetFinalizer(fd, (*netFD).Close)
}

func (fd *netFD) Close() error {
	runtime.SetFinalizer(fd, nil)
	return fd.pfd.Close()
}

func (fd *netFD) shutdown(how int) error {
	err := fd.pfd.Shutdown(how)
	runtime.KeepAlive(fd)
	return wrapSyscallError("shutdown", err)
}

func (fd *netFD) closeRead() error {
	return fd.shutdown(syscall.SHUT_RD)
}

func (fd *netFD) closeWrite() error {
	return fd.shutdown(syscall.SHUT_WR)
}

func (fd *netFD) Read(p []byte) (n int, err error) {
	n, err = fd.pfd.Read(p)
	runtime.KeepAlive(fd)
	return n, wrapSyscallError(readSyscallName, err)
}

func (fd *netFD) readFrom(p []byte) (n int, sa syscall.Sockaddr, err error) {
	n, sa, err = fd.pfd.ReadFrom(p)
	runtime.KeepAlive(fd)
	return n, sa, wrapSyscallError(readFromSyscallName, err)
}

func (fd *netFD) readMsg(p []byte, oob []byte) (n, oobn, flags int, sa syscall.Sockaddr, err error) {
	n, oobn, flags, sa, err = fd.pfd.ReadMsg(p, oob)
	runtime.KeepAlive(fd)
	return n, oobn, flags, sa, wrapSyscallError(readMsgSyscallName, err)
}

func (fd *netFD) Write(p []byte) (nn int, err error) {
	nn, err = fd.pfd.Write(p)
	runtime.KeepAlive(fd)
	return nn, wrapSyscallError(writeSyscallName, err)
}

func (fd *netFD) writeTo(p []byte, sa syscall.Sockaddr) (n int, err error) {
	n, err = fd.pfd.WriteTo(p, sa)
	runtime.KeepAlive(fd)
	return n, wrapSyscallError(writeToSyscallName, err)
}

func (fd *netFD) writeMsg(p []byte, oob []byte, sa syscall.Sockaddr) (n int, oobn int, err error) {
	n, oobn, err = fd.pfd.WriteMsg(p, oob, sa)
	runtime.KeepAlive(fd)
	return n, oobn, wrapSyscallError(writeMsgSyscallName, err)
}

func (fd *netFD) SetDeadline(t time.Time) error {
	return fd.pfd.SetDeadline(t)
}

func (fd *netFD) SetReadDeadline(t time.Time) error {
	return fd.pfd.SetReadDeadline(t)
}

func (fd *netFD) SetWriteDeadline(t time.Time) error {
	return fd.pfd.SetWriteDeadline(t)
}
