// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"runtime"
	"syscall"
)

func hostname() (name string, err error) {
	// 首先尝试uname，因为这只是一个系统调用，并且在Android上不允许从 /proc 读取。
	var un syscall.Utsname
	err = syscall.Uname(&un)

	var buf [512]byte // 足够存放一个DNS名称
	for i, b := range un.Nodename[:] {
		buf[i] = uint8(b)
		if b == 0 {
			name = string(buf[:i])
			break
		}
	}
	// 如果我们得到一个名称，并且该名称没有被截断（节点名称为65个字节），请返回该名称。
	if err == nil && len(name) > 0 && len(name) < 64 {
		return name, nil
	}
	if runtime.GOOS == "android" {
		if name != "" {
			return name, nil
		}
		return "localhost", nil
	}

	f, err := Open("/proc/sys/kernel/hostname")
	if err != nil {
		return "", err
	}
	defer f.Close()

	n, err := f.Read(buf[:])
	if err != nil {
		return "", err
	}

	if n > 0 && buf[n-1] == '\n' {
		n--
	}
	return string(buf[:n]), nil
}
