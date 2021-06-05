// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package poll supports non-blocking I/O on file descriptors with polling.
// This supports I/O operations that block only a goroutine, not a thread.
// This is used by the net and os packages.
// It uses a poller built into the runtime, with support from the
// runtime scheduler.

// Package poll支持文件描述符上的非阻塞I/O与轮询。
// 这支持I/O操作只阻塞一个goroutine，而不是一个线程。
// 这被net和os包所使用。
// 它使用一个内置于运行时的轮询器，并得到运行时调度器的支持。
package poll

import (
	"errors"
)

// errNetClosing is the type of the variable ErrNetClosing.
// This is used to implement the net.Error interface.
// errNetClosing是变量ErrNetClosing的类型。
// 这是用来实现net.Error接口的。
type errNetClosing struct{}

// Error returns the error message for ErrNetClosing.
// Keep this string consistent because of issue #4373:
// since historically programs have not been able to detect
// this error, they look for the string.

// Error返回ErrNetClosing的错误信息。
// 因为第4373号问题，保持这个字符串的一致性。
// 因为历史上程序无法检测到这个错误，他们寻找这个字符串。
func (e errNetClosing) Error() string { return "use of closed network connection" }

func (e errNetClosing) Timeout() bool   { return false }
func (e errNetClosing) Temporary() bool { return false }

// ErrNetClosing is returned when a network descriptor is used after
// it has been closed.
// 当一个网络描述符被关闭后被使用时，ErrNetClosing被返回。
var ErrNetClosing = errNetClosing{}

// ErrFileClosing is returned when a file descriptor is used after it
// has been closed.
// 当一个文件描述符被关闭后被使用时，会返回ErrFileClosing。
var ErrFileClosing = errors.New("use of closed file")

// ErrNoDeadline is returned when a request is made to set a deadline
// on a file type that does not use the poller.
// 当请求对不使用轮询器的文件类型设置截止日期时，返回ErrNoDeadline。
var ErrNoDeadline = errors.New("file type does not support deadline")

// Return the appropriate closing error based on isFile.
// 根据isFile返回适当的关闭错误。
func errClosing(isFile bool) error {
	if isFile {
		return ErrFileClosing
	}
	return ErrNetClosing
}

// ErrDeadlineExceeded is returned for an expired deadline.
// This is exported by the os package as os.ErrDeadlineExceeded.
// ErrDeadlineExceeded被返回给过期的期限。
// 这是由os包导出的os.ErrDeadlineExceeded。
var ErrDeadlineExceeded error = &DeadlineExceededError{}

// DeadlineExceededError is returned for an expired deadline.
// 截止日期过期时，返回DeadlineExceededError。
type DeadlineExceededError struct{}

// Implement the net.Error interface.
// The string is "i/o timeout" because that is what was returned
// by earlier Go versions. Changing it may break programs that
// match on error strings.

// 实现 net.Error 接口。
// 字符串是 "i/o timeout"，因为这是早期 Go 版本所返回的。改变它可能会破坏在错误字符串上匹配的程序。
func (e *DeadlineExceededError) Error() string   { return "i/o timeout" }
func (e *DeadlineExceededError) Timeout() bool   { return true }
func (e *DeadlineExceededError) Temporary() bool { return true }

// ErrNotPollable is returned when the file or socket is not suitable
// for event notification.
// 当文件或套接字不适合用于事件通知时，将返回ErrNotPollable。
var ErrNotPollable = errors.New("not pollable")

// consume removes data from a slice of byte slices, for writev.
// 消耗从一个字节片中删除数据，用于 writev。
func consume(v *[][]byte, n int64) {
	for len(*v) > 0 {
		ln0 := int64(len((*v)[0]))
		if ln0 > n {
			(*v)[0] = (*v)[0][n:]
			return
		}
		n -= ln0
		*v = (*v)[1:]
	}
}

// TestHookDidWritev is a hook for testing writev.
// TestHookDidWritev是一个用于测试writev的钩子。
var TestHookDidWritev = func(wrote int) {}
