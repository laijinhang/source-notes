// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ioutil implements some I/O utility functions.
//
// As of Go 1.16, the same functionality is now provided
// by package io or package os, and those implementations
// should be preferred in new code.
// See the specific function documentation for details.

// 包ioutil实现了一些I/O实用功能。
//
// 从Go 1.16开始，同样的功能现在由包io或包os提供，
// 在新的代码中应该首选这些实现。详情请见具体的函数文档。
package ioutil

import (
	"io"
	"io/fs"
	"os"
	"sort"
)

// ReadAll reads from r until an error or EOF and returns the data it read.
// A successful call returns err == nil, not err == EOF. Because ReadAll is
// defined to read from src until EOF, it does not treat an EOF from Read
// as an error to be reported.
// ReadAll从r读取数据，直到出现错误或EOF，并返回它所读取的数据。
// 一个成功的调用返回err == nil，而不是err == EOF。因为ReadAll
// 被定义为从src读到EOF为止，所以它不把Read的EOF当作一个需要报告的错误。
//
// As of Go 1.16, this function simply calls io.ReadAll.
// 从Go 1.16开始，这个函数只是调用io.ReadAll。
func ReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

// ReadFile reads the file named by filename and returns the contents.
// A successful call returns err == nil, not err == EOF. Because ReadFile
// reads the whole file, it does not treat an EOF from Read as an error
// to be reported.
// ReadFile 读取以文件名命名的文件并返回其内容。一个成功的调用返回err == nil，
// 而不是err == EOF。因为ReadFile读取的是整个文件，所以它不把来自Read的EOF
// 当作一个需要报告的错误。
//
// As of Go 1.16, this function simply calls os.ReadFile.
// 从Go 1.16开始，这个函数只是调用os.ReadFile。
func ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

// WriteFile writes data to a file named by filename.
// If the file does not exist, WriteFile creates it with permissions perm
// (before umask); otherwise WriteFile truncates it before writing, without changing permissions.
//
// As of Go 1.16, this function simply calls os.WriteFile.
func WriteFile(filename string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

// ReadDir reads the directory named by dirname and returns
// a list of fs.FileInfo for the directory's contents,
// sorted by filename. If an error occurs reading the directory,
// ReadDir returns no directory entries along with the error.
//
// As of Go 1.16, os.ReadDir is a more efficient and correct choice:
// it returns a list of fs.DirEntry instead of fs.FileInfo,
// and it returns partial results in the case of an error
// midway through reading a directory.
func ReadDir(dirname string) ([]fs.FileInfo, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	list, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	return list, nil
}

// NopCloser returns a ReadCloser with a no-op Close method wrapping
// the provided Reader r.
//
// As of Go 1.16, this function simply calls io.NopCloser.
func NopCloser(r io.Reader) io.ReadCloser {
	return io.NopCloser(r)
}

// Discard is an io.Writer on which all Write calls succeed
// without doing anything.
//
// As of Go 1.16, this value is simply io.Discard.
var Discard io.Writer = io.Discard
