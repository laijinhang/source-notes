// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

// A StatFS is a file system with a Stat method.
// 一个StatFS是一个具有Stat方法的文件系统。
type StatFS interface {
	FS

	// Stat returns a FileInfo describing the file.
	// If there is an error, it should be of type *PathError.
	// Stat返回一个描述该文件的FileInfo。 如果有错误，它应该是*PathError类型的。
	Stat(name string) (FileInfo, error)
}

// Stat returns a FileInfo describing the named file from the file system.
//
// If fs implements StatFS, Stat calls fs.Stat.
// Otherwise, Stat opens the file to stat it.
// Stat返回一个FileInfo，描述来自文件系统的命名文件。
//
// 如果fs实现了StatFS，Stat会调用fs.Stat。否则，Stat打开文件进行统计。
func Stat(fsys FS, name string) (FileInfo, error) {
	if fsys, ok := fsys.(StatFS); ok {
		return fsys.Stat(name)
	}

	file, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.Stat()
}
