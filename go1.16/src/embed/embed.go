// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package embed provides access to files embedded in the running Go program.
// 程序包嵌入可访问正在运行的Go程序中嵌入的文件。
//
// Go source files that import "embed" can use the //go:embed directive
// to initialize a variable of type string, []byte, or FS with the contents of
// files read from the package directory or subdirectories at compile time.
// 导入“嵌入”的Go源文件可以使用 //go：embed 指令使用在编译时从包目录或子目录读取的文件的内容来初始化字符串，[] byte或FS类型的变量。
//
// For example, here are three ways to embed a file named hello.txt
// and then print its contents at run time.
// 例如，以下三种方法可以嵌入名为hello.txt的文件，然后在运行时打印其内容。
//
// Embedding one file into a string:
// 将一个文件嵌入到字符串中：
//
//	import _ "embed"
//
//	//go:embed hello.txt
//	var s string
//	print(s)
//
// Embedding one file into a slice of bytes:
// 将一个文件嵌入[]byes中：
//
//	import _ "embed"
//
//	//go:embed hello.txt
//	var b []byte
//	print(string(b))
//
// Embedded one or more files into a file system:
// 将一个或多个文件嵌入到文件系统中：
//
//	import "embed"
//
//	//go:embed hello.txt
//	var f embed.FS
//	data, _ := f.ReadFile("hello.txt")
//	print(string(data))
//
// Directives
// 指令
//
// A //go:embed directive above a variable declaration specifies which files to embed,
// using one or more path.Match patterns.
// 变量声明上方的 //:goembed 指令使用一个或多个path.Match模式指定要嵌入的文件。
//
// The directive must immediately precede a line containing the declaration of a single variable.
// Only blank lines and ‘//’ line comments are permitted between the directive and the declaration.
// 指令必须紧接在包含单个变量声明的行之前。
// 在指令和声明之间仅允许使用空行和“ //”行注释。
//
// The type of the variable must be a string type, or a slice of a byte type,
// or FS (or an alias of FS).
// 变量的类型必须是字符串类型，或者是字节类型的切片，
// 或者是FS（或FS的别名）。
//
// For example:
// 例如：
//
//	package server
//
//	import "embed"
//
//	// content holds our static web server content.
//	//go:embed image/* template/*
//	//go:embed html/index.html
//	var content embed.FS
//
// The Go build system will recognize the directives and arrange for the declared variable
// (in the example above, content) to be populated with the matching files from the file system.
// Go构建系统将识别指令并安排声明的变量
//（在上面的示例中，内容）将使用来自文件系统的匹配文件进行填充。
//
// The //go:embed directive accepts multiple space-separated patterns for
// brevity, but it can also be repeated, to avoid very long lines when there are
// many patterns. The patterns are interpreted relative to the package directory
// containing the source file. The path separator is a forward slash, even on
// Windows systems. Patterns may not contain ‘.’ or ‘..’ or empty path elements,
// nor may they begin or end with a slash. To match everything in the current
// directory, use ‘*’ instead of ‘.’. To allow for naming files with spaces in
// their names, patterns can be written as Go double-quoted or back-quoted
// string literals.
// go：embed指令接受多个空格分隔的模式
// 简短，但也可以重复，以免在出现以下情况时排长队
// 许多模式。模式是相对于包目录解释的
// 包含源文件。路径分隔符是一个正斜杠，即使在
// Windows系统。模式中不得包含“。”或“ ..”或空路径元素，
// 它们也不能以斜线开头或结尾。匹配当前的所有内容
// 目录中，使用“ *”代替“。”。允许命名文件中带有空格的
// 它们的名称，模式可以写成Go双引号或反引号
// 字符串文字。
//
// If a pattern names a directory, all files in the subtree rooted at that directory are
// embedded (recursively), except that files with names beginning with ‘.’ or ‘_’
// are excluded. So the variable in the above example is almost equivalent to:
// 如果模式命名目录，则以该目录为根的子树中的所有文件都是
//（递归）嵌入，但文件名以“。”或“ _”开头的文件除外
// 被排除在外。因此，以上示例中的变量几乎等同于：
//
//	// content is our static web server content.
//	//go:embed image template html/index.html
//	var content embed.FS
//
// The difference is that ‘image/*’ embeds ‘image/.tempfile’ while ‘image’ does not.
//
// The //go:embed directive can be used with both exported and unexported variables,
// depending on whether the package wants to make the data available to other packages.
// It can only be used with global variables at package scope,
// not with local variables.
//
// Patterns must not match files outside the package's module, such as ‘.git/*’ or symbolic links.
// Matches for empty directories are ignored. After that, each pattern in a //go:embed line
// must match at least one file or non-empty directory.
//
// If any patterns are invalid or have invalid matches, the build will fail.
//
// Strings and Bytes
//
// The //go:embed line for a variable of type string or []byte can have only a single pattern,
// and that pattern can match only a single file. The string or []byte is initialized with
// the contents of that file.
//
// The //go:embed directive requires importing "embed", even when using a string or []byte.
// In source files that don't refer to embed.FS, use a blank import (import _ "embed").
//
// File Systems
//
// For embedding a single file, a variable of type string or []byte is often best.
// The FS type enables embedding a tree of files, such as a directory of static
// web server content, as in the example above.
//
// FS implements the io/fs package's FS interface, so it can be used with any package that
// understands file systems, including net/http, text/template, and html/template.
//
// For example, given the content variable in the example above, we can write:
//
//	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(content))))
//
//	template.ParseFS(content, "*.tmpl")
//
// Tools
//
// To support tools that analyze Go packages, the patterns found in //go:embed lines
// are available in “go list” output. See the EmbedPatterns, TestEmbedPatterns,
// and XTestEmbedPatterns fields in the “go help list” output.
//
package embed

import (
	"errors"
	"io"
	"io/fs"
	"time"
)

// An FS is a read-only collection of files, usually initialized with a //go:embed directive.
// When declared without a //go:embed directive, an FS is an empty file system.
// FS是文件的只读集合，通常使用 //go:embed 指令进行初始化。
// 如果不使用 //go:embed 指令声明FS，则它是一个空文件系统。
//
// An FS is a read-only value, so it is safe to use from multiple goroutines
// simultaneously and also safe to assign values of type FS to each other.
// FS是只读值，因此可以安全地从多个goroutine中同时使用，也可以将FS类型的值相互分配。
//
// FS implements fs.FS, so it can be used with any package that understands
// file system interfaces, including net/http, text/template, and html/template.
// FS实现了fs.FS，因此可以与任何了解以下内容的软件包一起使用
// 文件系统接口，包括 net/http，text/template 和 html/template。
//
// See the package documentation for more details about initializing an FS.
// 有关初始化FS的更多详细信息，请参见软件包文档。
type FS struct {
	// The compiler knows the layout of this struct.
	// See cmd/compile/internal/gc's initEmbed.
	// 编译器知道此结构的布局。
	// 参见 cmd/compile/internal/gc的initEmbed。
	//
	// The files list is sorted by name but not by simple string comparison.
	// Instead, each file's name takes the form "dir/elem" or "dir/elem/".
	// The optional trailing slash indicates that the file is itself a directory.
	// The files list is sorted first by dir (if dir is missing, it is taken to be ".")
	// and then by base, so this list of files:
	// 文件列表按名称排序，但不按简单字符串比较排序。
	// 而是，每个文件的名称都采用 “dir/elem” 或 “dir/elem/” 的形式。
	// 可选的尾部斜杠表示文件本身是目录。
	// 文件列表首先按dir排序（如果缺少dir，则将其视为“。”）。
	// 然后按基数排列，因此文件列表如下：
	//
	//	p
	//	q/
	//	q/r
	//	q/s/
	//	q/s/t
	//	q/s/u
	//	q/v
	//	w
	//
	// is actually sorted as:
	//
	//	p       # dir=.    elem=p
	//	q/      # dir=.    elem=q
	//	w/      # dir=.    elem=w
	//	q/r     # dir=q    elem=r
	//	q/s/    # dir=q    elem=s
	//	q/v     # dir=q    elem=v
	//	q/s/t   # dir=q/s  elem=t
	//	q/s/u   # dir=q/s  elem=u
	//
	// This order brings directory contents together in contiguous sections
	// of the list, allowing a directory read to use binary search to find
	// the relevant sequence of entries.
	// 此顺序将目录内容放在列表的连续部分中，从而允许目录读取使用二进制搜索来找到相关的条目序列。
	files *[]file
}

// split splits the name into dir and elem as described in the
// comment in the FS struct above. isDir reports whether the
// final trailing slash was present, indicating that name is a directory.
// split将名称拆分为dir和elem，如上面FS结构中的注释中所述。 isDir报告最后的斜杠是否存在，指示该名称是目录。
func split(name string) (dir, elem string, isDir bool) {
	if name[len(name)-1] == '/' {
		isDir = true
		name = name[:len(name)-1]
	}
	i := len(name) - 1
	for i >= 0 && name[i] != '/' {
		i--
	}
	if i < 0 {
		return ".", name, isDir
	}
	return name[:i], name[i+1:], isDir
}

// trimSlash trims a trailing slash from name, if present,
// returning the possibly shortened name.
// trimSlash去掉name末尾的斜杠（如果最后一位是/），并返回可能会缩短的名称。
func trimSlash(name string) string {
	if len(name) > 0 && name[len(name)-1] == '/' {
		return name[:len(name)-1]
	}
	return name
}

var (
	_ fs.ReadDirFS  = FS{}
	_ fs.ReadFileFS = FS{}
)

// A file is a single file in the FS.
// It implements fs.FileInfo and fs.DirEntry.
// file是FS中的单个文件，它实现了fs.FileInfo和fs.DirEntry。
type file struct {
	// The compiler knows the layout of this struct.
	// See cmd/compile/internal/gc's initEmbed.
	// 编译器知道此结构的布局。请参阅 cmd/compile/internal/gc 的initEmbed。
	name string
	data string
	hash [16]byte // truncated SHA256 hash，截断的SHA256哈希
}

var (
	_ fs.FileInfo = (*file)(nil)
	_ fs.DirEntry = (*file)(nil)
)

func (f *file) Name() string               { _, elem, _ := split(f.name); return elem }
func (f *file) Size() int64                { return int64(len(f.data)) }
func (f *file) ModTime() time.Time         { return time.Time{} }
func (f *file) IsDir() bool                { _, _, isDir := split(f.name); return isDir }
func (f *file) Sys() interface{}           { return nil }
func (f *file) Type() fs.FileMode          { return f.Mode().Type() }
func (f *file) Info() (fs.FileInfo, error) { return f, nil }

func (f *file) Mode() fs.FileMode {
	if f.IsDir() {
		return fs.ModeDir | 0555
	}
	return 0444
}

// dotFile is a file for the root directory,
// which is omitted from the files list in a FS.
// dotFile是用于根目录的文件，在FS的文件列表中被省略。
var dotFile = &file{name: "./"}

// lookup returns the named file, or nil if it is not present.
// 查找返回命名文件，如果不存在，则返回nil。
func (f FS) lookup(name string) *file {
	if !fs.ValidPath(name) {
		// The compiler should never emit a file with an invalid name,
		// so this check is not strictly necessary (if name is invalid,
		// we shouldn't find a match below), but it's a good backstop anyway.
		return nil
	}
	if name == "." {
		return dotFile
	}
	if f.files == nil {
		return nil
	}

	// Binary search to find where name would be in the list,
	// and then check if name is at that position.
	// 二进制搜索以查找名称在列表中的位置，然后检查名称是否在该位置。
	dir, elem, _ := split(name)
	files := *f.files
	i := sortSearch(len(files), func(i int) bool {
		idir, ielem, _ := split(files[i].name)
		return idir > dir || idir == dir && ielem >= elem
	})
	if i < len(files) && trimSlash(files[i].name) == name {
		return &files[i]
	}
	return nil
}

// readDir returns the list of files corresponding to the directory dir.
// readDir返回与目录dir对应的文件列表。
func (f FS) readDir(dir string) []file {
	if f.files == nil {
		return nil
	}
	// Binary search to find where dir starts and ends in the list
	// and then return that slice of the list.
	// 二进制搜索以查找dir在列表中的开始和结束位置，然后返回列表的该片。
	files := *f.files
	i := sortSearch(len(files), func(i int) bool {
		idir, _, _ := split(files[i].name)
		return idir >= dir
	})
	j := sortSearch(len(files), func(j int) bool {
		jdir, _, _ := split(files[j].name)
		return jdir > dir
	})
	return files[i:j]
}

// Open opens the named file for reading and returns it as an fs.File.
// 打开将打开指定的文件以供读取，并将其作为fs.File返回。
func (f FS) Open(name string) (fs.File, error) {
	file := f.lookup(name)
	if file == nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if file.IsDir() {
		return &openDir{file, f.readDir(name), 0}, nil
	}
	return &openFile{file, 0}, nil
}

// ReadDir reads and returns the entire named directory.
func (f FS) ReadDir(name string) ([]fs.DirEntry, error) {
	file, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	dir, ok := file.(*openDir)
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: name, Err: errors.New("not a directory")}
	}
	list := make([]fs.DirEntry, len(dir.files))
	for i := range list {
		list[i] = &dir.files[i]
	}
	return list, nil
}

// ReadFile reads and returns the content of the named file.
func (f FS) ReadFile(name string) ([]byte, error) {
	file, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	ofile, ok := file.(*openFile)
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: name, Err: errors.New("is a directory")}
	}
	return []byte(ofile.f.data), nil
}

// An openFile is a regular file open for reading.
type openFile struct {
	f      *file // the file itself
	offset int64 // current read offset
}

func (f *openFile) Close() error               { return nil }
func (f *openFile) Stat() (fs.FileInfo, error) { return f.f, nil }

func (f *openFile) Read(b []byte) (int, error) {
	if f.offset >= int64(len(f.f.data)) {
		return 0, io.EOF
	}
	if f.offset < 0 {
		return 0, &fs.PathError{Op: "read", Path: f.f.name, Err: fs.ErrInvalid}
	}
	n := copy(b, f.f.data[f.offset:])
	f.offset += int64(n)
	return n, nil
}

func (f *openFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		// offset += 0
	case 1:
		offset += f.offset
	case 2:
		offset += int64(len(f.f.data))
	}
	if offset < 0 || offset > int64(len(f.f.data)) {
		return 0, &fs.PathError{Op: "seek", Path: f.f.name, Err: fs.ErrInvalid}
	}
	f.offset = offset
	return offset, nil
}

// An openDir is a directory open for reading.
type openDir struct {
	f      *file  // the directory file itself
	files  []file // the directory contents
	offset int    // the read offset, an index into the files slice
}

func (d *openDir) Close() error               { return nil }
func (d *openDir) Stat() (fs.FileInfo, error) { return d.f, nil }

func (d *openDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.f.name, Err: errors.New("is a directory")}
}

func (d *openDir) ReadDir(count int) ([]fs.DirEntry, error) {
	n := len(d.files) - d.offset
	if count > 0 && n > count {
		n = count
	}
	if n == 0 {
		if count <= 0 {
			return nil, nil
		}
		return nil, io.EOF
	}
	list := make([]fs.DirEntry, n)
	for i := range list {
		list[i] = &d.files[d.offset+i]
	}
	d.offset += n
	return list, nil
}

// sortSearch is like sort.Search, avoiding an import.
func sortSearch(n int, f func(int) bool) int {
	// Define f(-1) == false and f(n) == true.
	// Invariant: f(i-1) == false, f(j) == true.
	i, j := 0, n
	for i < j {
		h := int(uint(i+j) >> 1) // avoid overflow when computing h
		// i ≤ h < j
		if !f(h) {
			i = h + 1 // preserves f(i-1) == false
		} else {
			j = h // preserves f(j) == true
		}
	}
	// i == j, f(i-1) == false, and f(j) (= f(i)) == true  =>  answer is i.
	return i
}
