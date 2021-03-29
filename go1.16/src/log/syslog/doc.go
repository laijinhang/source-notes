// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package syslog provides a simple interface to the system log
// service. It can send messages to the syslog daemon using UNIX
// domain sockets, UDP or TCP.
// syslog包提供了一个简单的系统日志服务的接口。
// 它可以使用UNIX域套接字、UDP或TCP向syslog守护进程发送消息
//
// Only one call to Dial is necessary. On write failures,
// the syslog client will attempt to reconnect to the server
// and write again.
// 只需一个Dial就可以了。当写入失败时，
// syslog客户端将尝试重新连接到服务器
// 并尝试再次写入
//
// The syslog package is frozen and is not accepting new features.
// Some external packages provide more functionality. See:
//
// https://godoc.org/?q=syslog
// syslog包被冻结，不接受新特性。一些外部包提供了更多的功能。
// 请参考：https://godoc.org/?q=syslog
package syslog

// BUG(brainman): This package is not implemented on Windows. As the
// syslog package is frozen, Windows users are encouraged to
// use a package outside of the standard library. For background,
// see https://golang.org/issue/1108.
// BUG(brainman): 这个包没有在Windows上实现。由于syslog包被冻结，Windows用户
// 被鼓励使用标准库之外的包。
//	有关背景信息， 请参见：https://github.com/golang/go/blob/master/issue/1108

// BUG(akumar): This package is not implemented on Plan 9.
// BUG(akumar): 这个包没有在Plan 9上实现。
