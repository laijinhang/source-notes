// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build netcgo

package net

/*

// Fail if cgo isn't available.
// 如果cgo不可用，则失败。

*/
import "C"

// The build tag "netcgo" forces use of the cgo DNS resolver.
// It is the opposite of "netgo".
// 构建标签 "netcgo "强制使用 cgo DNS 解析器。
// 它与 "netgo "正好相反。
func init() { netCgo = true }
