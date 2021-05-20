// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"path"
)

// A GlobFS is a file system with a Glob method.
// GlobFS是一个具有Glob方法的文件系统。
type GlobFS interface {
	FS

	// Glob returns the names of all files matching pattern,
	// providing an implementation of the top-level
	// Glob function.
	// Glob返回与pattern匹配的所有文件的名称，提供一个顶级Glob函数的实现。
	Glob(pattern string) ([]string, error)
}

// Glob returns the names of all files matching pattern or nil
// if there is no matching file. The syntax of patterns is the same
// as in path.Match. The pattern may describe hierarchical names such as
// usr/*/bin/ed.
// Glob返回与模式匹配的所有文件的名称，如果没有匹配的文件则返回nil。
// 模式的语法与path.Match中相同。模式可以描述层次化的名称，如usr/*/bin/ed。
//
// Glob ignores file system errors such as I/O errors reading directories.
// The only possible returned error is path.ErrBadPattern, reporting that
// the pattern is malformed.
// Glob忽略了文件系统的错误，如I/O错误读取目录。
// 唯一可能返回的错误是path.ErrBadPattern，报告模式是malformed。
//
// If fs implements GlobFS, Glob calls fs.Glob.
// Otherwise, Glob uses ReadDir to traverse the directory tree
// and look for matches for the pattern.
// 如果fs实现了GlobFS，Glob调用fs.Glob。
// 否则，Glob使用ReadDir遍历目录树，寻找匹配的模式。
func Glob(fsys FS, pattern string) (matches []string, err error) {
	if fsys, ok := fsys.(GlobFS); ok {
		return fsys.Glob(pattern)
	}

	// Check pattern is well-formed.
	// 检查pattern是否格式良好。
	if _, err := path.Match(pattern, ""); err != nil {
		return nil, err
	}
	if !hasMeta(pattern) {
		if _, err = Stat(fsys, pattern); err != nil {
			return nil, nil
		}
		return []string{pattern}, nil
	}

	dir, file := path.Split(pattern)
	dir = cleanGlobPath(dir)

	if !hasMeta(dir) {
		return glob(fsys, dir, file, nil)
	}

	// Prevent infinite recursion. See issue 15879.
	if dir == pattern {
		return nil, path.ErrBadPattern
	}

	var m []string
	m, err = Glob(fsys, dir)
	if err != nil {
		return
	}
	for _, d := range m {
		matches, err = glob(fsys, d, file, matches)
		if err != nil {
			return
		}
	}
	return
}

// cleanGlobPath prepares path for glob matching.
func cleanGlobPath(path string) string {
	switch path {
	case "":
		return "."
	default:
		return path[0 : len(path)-1] // chop off trailing separator
	}
}

// glob searches for files matching pattern in the directory dir
// and appends them to matches, returning the updated slice.
// If the directory cannot be opened, glob returns the existing matches.
// New matches are added in lexicographical order.
func glob(fs FS, dir, pattern string, matches []string) (m []string, e error) {
	m = matches
	infos, err := ReadDir(fs, dir)
	if err != nil {
		return // ignore I/O error
	}

	for _, info := range infos {
		n := info.Name()
		matched, err := path.Match(pattern, n)
		if err != nil {
			return m, err
		}
		if matched {
			m = append(m, path.Join(dir, n))
		}
	}
	return
}

// hasMeta reports whether path contains any of the magic characters
// recognized by path.Match.
// hasMeta报告路径是否包含path.Match所识别的任何magic字符。
func hasMeta(path string) bool {
	for i := 0; i < len(path); i++ {
		switch path[i] {
		case '*', '?', '[', '\\':
			return true
		}
	}
	return false
}
