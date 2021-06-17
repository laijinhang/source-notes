// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package strings

// Compare returns an integer comparing two strings lexicographically.
// The result will be 0 if a==b, -1 if a < b, and +1 if a > b.
// 比较返回一个整数，按字母顺序比较两个字符串。如果a==b，结果是0；如果a<b，结果是-1；如果a>b，结果是+1。
//
// Compare is included only for symmetry with package bytes.
// It is usually clearer and always faster to use the built-in
// string comparison operators ==, <, >, and so on.
// 包含Compare只是为了与package bytes对称。通常使用内置的字符串比较运算符==、<、>等会更清晰、更快速。
func Compare(a, b string) int {
	// NOTE(rsc): This function does NOT call the runtime cmpstring function,
	// because we do not want to provide any performance justification for
	// using strings.Compare. Basically no one should use strings.Compare.
	// As the comment above says, it is here only for symmetry with package bytes.
	// If performance is important, the compiler should be changed to recognize
	// the pattern so that all code doing three-way comparisons, not just code
	// using strings.Compare, can benefit.
	// 注意(rsc)。这个函数不调用运行时的cmpstring函数，因为我们不想为使用strings.Compare提供任何性能方面的理由。
	// 基本上没有人应该使用strings.Compare。正如上面的评论所说，它在这里只是为了与软件包bytes对称。如果性能很重要，
	// 应该改变编译器以识别该模式，这样所有进行三方比较的代码，而不仅仅是使用strings.Compare的代码，都可以受益。
	if a == b {
		return 0
	}
	if a < b {
		return -1
	}
	return +1
}
