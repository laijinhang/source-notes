// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !windows

package execenv

import "syscall"

// Default will return the default environment
// variables based on the process attributes
// provided.
// Default将根据提供的进程属性返回默认的环境变量。
//
// Defaults to syscall.Environ() on all platforms
// other than Windows.
// 在Windows以外的所有平台上默认为syscall.Environ()。
func Default(sys *syscall.SysProcAttr) ([]string, error) {
	return syscall.Environ(), nil
}
