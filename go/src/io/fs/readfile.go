// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import "io"

// ReadFileFS is the interface implemented by a file system
// that provides an optimized implementation of ReadFile.
// ReadFileFS 是一个文件系统实现的接口，它提供了 ReadFile 的优化实现。
type ReadFileFS interface {
	FS

	// ReadFile reads the named file and returns its contents.
	// A successful call returns a nil error, not io.EOF.
	// (Because ReadFile reads the whole file, the expected EOF
	// from the final Read is not treated as an error to be reported.)
	//
	// The caller is permitted to modify the returned byte slice.
	// This method should return a copy of the underlying data.
	// ReadFile 读取指定的文件并返回其内容。
	// 一个成功的调用会返回一个 nil 错误，而不是 io.EOF。
	// (因为 ReadFile 读取的是整个文件，所以最终读取的
	// 预期 EOF 不会被当作一个需要报告的错误)。
	ReadFile(name string) ([]byte, error)
}

// ReadFile reads the named file from the file system fs and returns its contents.
// A successful call returns a nil error, not io.EOF.
// (Because ReadFile reads the whole file, the expected EOF
// from the final Read is not treated as an error to be reported.)
// ReadFile 从文件系统 fs 读取指定的文件并返回其内容。一个成功的调用会返回一个 nil 错误，
// 而不是 io.EOF。(因为 ReadFile 读取的是整个文件，所以最终读取的预期 EOF 不会被当作一个需要报告的错误)。
//
// If fs implements ReadFileFS, ReadFile calls fs.ReadFile.
// Otherwise ReadFile calls fs.Open and uses Read and Close
// on the returned file.
// 如果 fs 实现了 ReadFileFS，ReadFile 会调用 fs.ReadFile。
// 否则 ReadFile 调用 fs.Open 并对返回的文件使用 Read 和 Close。
func ReadFile(fsys FS, name string) ([]byte, error) {
	if fsys, ok := fsys.(ReadFileFS); ok {
		return fsys.ReadFile(name)
	}

	file, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var size int
	if info, err := file.Stat(); err == nil {
		size64 := info.Size()
		if int64(int(size64)) == size64 {
			size = int(size64)
		}
	}

	data := make([]byte, 0, size+1)
	for {
		if len(data) >= cap(data) {
			d := append(data[:cap(data)], 0)
			data = d[:len(data)]
		}
		n, err := file.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return data, err
		}
	}
}
