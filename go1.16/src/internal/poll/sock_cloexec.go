// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements sysSocket and accept for platforms that
// provide a fast path for setting SetNonblock and CloseOnExec.

// +build dragonfly freebsd illumos linux netbsd openbsd

package poll

import "syscall"

// Wrapper around the accept system call that marks the returned file
// descriptor as nonblocking and close-on-exec.
// 围绕accept系统调用的包装，将返回的文件描述符标记为非阻塞和执行时关闭。
func accept(s int) (int, syscall.Sockaddr, string, error) {
	ns, sa, err := Accept4Func(s, syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC)
	// On Linux the accept4 system call was introduced in 2.6.28
	// kernel and on FreeBSD it was introduced in 10 kernel. If we
	// get an ENOSYS error on both Linux and FreeBSD, or EINVAL
	// error on Linux, fall back to using accept.
	// 在Linux上，accept4系统调用是在2.6.28内核中引入的，在FreeBSD上是在10内核中引入的。
	// 如果我们在Linux和FreeBSD上都遇到ENOSYS错误，或者在Linux上遇到EINVAL错误，那么就退回到使用accept。
	switch err {
	case nil:
		return ns, sa, "", nil
	default: // errors other than the ones listed	// 所列错误以外的错误
		return -1, sa, "accept4", err
	case syscall.ENOSYS: // syscall missing	// 系统调用丢失
	case syscall.EINVAL: // some Linux use this instead of ENOSYS	// 有些Linux使用这个来代替ENOSYS。
	case syscall.EACCES: // some Linux use this instead of ENOSYS	// 有些Linux使用这个来代替ENOSYS。
	case syscall.EFAULT: // some Linux use this instead of ENOSYS	// 有些Linux使用这个来代替ENOSYS。
	}

	// See ../syscall/exec_unix.go for description of ForkLock.
	// It is probably okay to hold the lock across syscall.Accept
	// because we have put fd.sysfd into non-blocking mode.
	// However, a call to the File method will put it back into
	// blocking mode. We can't take that risk, so no use of ForkLock here.
	// 参见 .../syscall/exec_unix.go 关于ForkLock的描述。在syscall.Accept中保持锁可能是好的，
	// 因为我们已经把fd.sysfd放到了非阻塞模式。然而，对File方法的调用将使它回到阻塞模式。我们不能冒这个险，
	// 所以这里没有使用ForkLock。
	ns, sa, err = AcceptFunc(s)
	if err == nil {
		syscall.CloseOnExec(ns)
	}
	if err != nil {
		return -1, nil, "accept", err
	}
	if err = syscall.SetNonblock(ns, true); err != nil {
		CloseFunc(ns)
		return -1, nil, "setnonblock", err
	}
	return ns, sa, "", nil
}
