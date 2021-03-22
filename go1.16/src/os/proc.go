// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Process etc.

package os

import (
	"internal/testlog"
	"runtime"
	"syscall"
)

// Args hold the command-line arguments, starting with the program name.
var Args []string

func init() {
	if runtime.GOOS == "windows" {
		// Initialized in exec_windows.go.
		// 如果是windows，则在exec_windows.go中初始化。
		return
	}
	Args = runtime_args()
}

func runtime_args() []string // in package runtime

// Getuid返回调用方的数字用户ID。
//
// 在Windows上，它返回-1。
func Getuid() int { return syscall.Getuid() }

// Geteuid返回调用者的数字有效用户ID。
//
// 在Windows上，它返回-1。
func Geteuid() int { return syscall.Geteuid() }

// Getgid返回调用者的数字组ID。
//
// 在Windows上，它返回-1。
func Getgid() int { return syscall.Getgid() }

// Getegid返回调用者的数字有效组ID。
//
// 在Windows上，它返回-1。
func Getegid() int { return syscall.Getegid() }

// Getgroups返回调用者所属的组的数字ID的列表。
//
// 在Windows上，它返回syscall.EWINDOWS。查看 os/user 软件包
// 作为可能的替代方法
func Getgroups() ([]int, error) {
	gids, e := syscall.Getgroups()
	return gids, NewSyscallError("getgroups", e)
}

// Exit causes the current program to exit with the given status code.
// Conventionally, code zero indicates success, non-zero an error.
// The program terminates immediately; deferred functions are not run.
//
// For portability, the status code should be in the range [0, 125].
func Exit(code int) {
	if code == 0 {
		if testlog.PanicOnExit0() {
			// We were told to panic on calls to os.Exit(0).
			// This is used to fail tests that make an early
			// unexpected call to os.Exit(0).
			panic("unexpected call to os.Exit(0) during test")
		}

		// Give race detector a chance to fail the program.
		// Racy programs do not have the right to finish successfully.
		runtime_beforeExit()
	}
	syscall.Exit(code)
}

func runtime_beforeExit() // implemented in runtime
