// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// package sys contains system- and configuration- and architecture-specific
// constants used by the runtime.
package sys

// The next line makes 'go generate' write the zgo*.go files with
// per-OS and per-arch information, including constants
// named Goos$GOOS and Goarch$GOARCH for every
// known GOOS and GOARCH. The constant is 1 on the
// current system, 0 otherwise; multiplying by them is
// useful for defining GOOS- or GOARCH-specific constants.
// 下一行让“go generate”编写zgo*.go文件，其中包含每个OS和每个arch的信息，
// 包括每个已知的Goos和Goarch的名为Goos$Goos和Goarch$Goarch的常量。
// 在当前系统中，该常数为1，否则为0；将它们相乘可用于定义GOOS或GOARCH特定的常数。
//go:generate go run gengoos.go
