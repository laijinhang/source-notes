// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"errors"
	"sort"
)

// ReadDirFS is the interface implemented by a file system
// that provides an optimized implementation of ReadDir.
// ReadDirFS是由一个文件系统实现的接口，它提供了ReadDir的优化实现。
type ReadDirFS interface {
	FS

	// ReadDir reads the named directory
	// and returns a list of directory entries sorted by filename.
	// ReadDir 读取命名的目录，并返回一个按文件名排序的目录条目列表。
	ReadDir(name string) ([]DirEntry, error)
}

// ReadDir reads the named directory
// and returns a list of directory entries sorted by filename.
//
// If fs implements ReadDirFS, ReadDir calls fs.ReadDir.
// Otherwise ReadDir calls fs.Open and uses ReadDir and Close
// on the returned file.

// ReadDir 读取命名的目录，并返回一个按文件名排序的目录条目列表。
//
// 如果fs实现了ReadDirFS，ReadDir调用fs.ReadDir。 否则ReadDir调用fs.Open并对返回的文件使用ReadDir和Close。
/*
这个ReadDirFS和GlobFS看着很类似，一样实现方式的两种不同功能的内容
*/
func ReadDir(fsys FS, name string) ([]DirEntry, error) {
	if fsys, ok := fsys.(ReadDirFS); ok {
		return fsys.ReadDir(name)
	}

	file, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dir, ok := file.(ReadDirFile)
	if !ok {
		return nil, &PathError{Op: "readdir", Path: name, Err: errors.New("not implemented")}
	}

	list, err := dir.ReadDir(-1)
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	return list, err
}
