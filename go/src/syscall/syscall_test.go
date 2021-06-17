// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syscall_test

import (
	"internal/testenv"
	"os"
	"runtime"
	"syscall"
	"testing"
)

/*
测试 设置环境环境 和 获取环境变量 功能
*/
func testSetGetenv(t *testing.T, key, value string) {
	// 设置设置key，value环境
	err := syscall.Setenv(key, value)
	// 如果设置遇到错误，打印错误
	if err != nil {
		t.Fatalf("Setenv failed to set %q: %v", value, err)
	}
	// 获取环境变量
	newvalue, found := syscall.Getenv(key)
	// 如果没有找到，打印错误
	if !found {
		t.Fatalf("Getenv failed to find %v variable (want value %q)", key, value)
	}
	// 如果设置的和获取到的不一致
	if newvalue != value {
		t.Fatalf("Getenv(%v) = %q; want %q", key, newvalue, value)
	}
}

/*
测试 环境变量
*/
func TestEnv(t *testing.T) {
	testSetGetenv(t, "TESTENV", "AVALUE")
	// make sure TESTENV gets set to "", not deleted
	// 确保TESTENV被设置为""，而不是被删除。
	testSetGetenv(t, "TESTENV", "")
}

// Check that permuting child process fds doesn't interfere with
// reporting of fork/exec status. See Issue 14979.
func TestExecErrPermutedFds(t *testing.T) {
	testenv.MustHaveExec(t)

	attr := &os.ProcAttr{Files: []*os.File{os.Stdin, os.Stderr, os.Stdout}}
	_, err := os.StartProcess("/", []string{"/"}, attr)
	if err == nil {
		t.Fatalf("StartProcess of invalid program returned err = nil")
	}
}

func TestGettimeofday(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("not implemented on " + runtime.GOOS)
	}
	tv := &syscall.Timeval{}
	if err := syscall.Gettimeofday(tv); err != nil {
		t.Fatal(err)
	}
	if tv.Sec == 0 && tv.Usec == 0 {
		t.Fatal("Sec and Usec both zero")
	}
}
