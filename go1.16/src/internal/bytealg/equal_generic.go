// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bytealg

// Equal reports whether a and b
// are the same length and contain the same bytes.
// A nil argument is equivalent to an empty slice.
//
// Equal is equivalent to bytes.Equal.
// It is provided here for convenience,
// because some packages cannot depend on bytes.

// Equal报告a和b的长度是否相同，包含的字节数是否相同。
// nil参数相当于一个空分片。 Equal相当于bytes.Equal。
// 这里提供这个参数是为了方便，因为有些包不能依赖字节。
func Equal(a, b []byte) bool {
	// Neither cmd/compile nor gccgo allocates for these string conversions.
	// There is a test for this in package bytes.
	// cmd/compile 和 gccgo 都没有为这些字符串转换分配资源。
	// 在package bytes中有一个测试。
	return string(a) == string(b)
}
