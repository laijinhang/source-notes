// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build amd64 arm64 s390x

package bytealg

//go:noescape

// Index returns the index of the first instance of b in a, or -1 if b is not present in a.
// Requires 2 <= len(b) <= MaxLen.
// Index返回a中b的第一个实例的索引，如果b在a中不存在，则返回-1。
// 要求 2 <= len(b) <= MaxLen.
func Index(a, b []byte) int

//go:noescape

// IndexString returns the index of the first instance of b in a, or -1 if b is not present in a.
// Requires 2 <= len(b) <= MaxLen.
// IndexString返回a中b的第一个实例的索引，如果a中不存在b，则返回-1。
// 要求 2 <= len(b) <= MaxLen.
func IndexString(a, b string) int
