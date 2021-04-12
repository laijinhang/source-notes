// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

import "runtime/internal/sys"

const MaxUintptr = ^uintptr(0)

// MulUintptr returns a * b and whether the multiplication overflowed.
// On supported platforms this is an intrinsic lowered by the compiler.

// MulUintptr 返回 a * b 以及乘法是否溢出。
// 在支持的平台上，这是编译器的内在降低。
/*
作用：用于判断内存申请，如果越界就返回true，否则返回false
*/
func MulUintptr(a, b uintptr) (uintptr, bool) {
	// 因为sys.PtrSize的值是8，所以 =>
	// a|b < 1 << 32			 =>
	// 如果 a和b 都小于 2^32 或者 a=0，返回 a * b，没有越界
	if a|b < 1<<(4*sys.PtrSize) || a == 0 {
		return a * b, false
	}
	// overflow := b > MaxUintptr/a	=>
	// overflow := b > (2^64)/a	    =>
	// overflow := (a * b) > (2^64)
	overflow := b > MaxUintptr/a
	return a * b, overflow
}
