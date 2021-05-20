package fs

import (
	"internal/oserror"
	"time"
	"unicode/utf8"
)

type FS interface {
	Open(name string) (File, error)
}

// 检查是否有效文件名
// 1、传入的完全由utf8组成
// 2、如果存在/，则最后一个/后面必须是一个有效文件名
// 3、name不能为空，首字符或者/后面不能以 ./，../，.，..，/，出现
// 如：./，../，a/./，a/../，a//
func ValidPath(name string) bool {
	// 检查name是否完全由utf8组成
	if !utf8.ValidString(name) {
		return false
	}

	if name == "." {
		// special case
		return true
	}

	// Iterate over elements in name, checking each.
	for {
		i := 0
		for i < len(name) && name[i] != '/' {
			i++
		}
		elem := name[:i]
		// 绝对路径，当前路径，上一个目录开始
		if elem == "" || elem == "." || elem == ".." {
			return false
		}
		if i == len(name) {
			return true // reached clean ending
		}
		name = name[i+1:]
	}
}

type File interface {
	Stat() (FileInfo, error)
	Read([]byte) (int, error)
	Close() error
}

type DirEntry interface {
	Name() string
	IsDir() bool
	Type() FileMode
	Info() (FileInfo, error)
}

type ReadDirFile interface {
	File
	ReadDir(n int) ([]DirEntry, error)
}

var (
	ErrInvalid    = errInvalid()    // "invalid argument"，无效参数
	ErrPermission = errPermission() // "permission denied"，没有权限
	ErrExist      = errExist()      // "file already exists"，文件已存在
	ErrNotExist   = errNotExist()   // "file does not exist"，文件不存在
	ErrClosed     = errClosed()     // "file already closed"，文件已经关闭
)

func errInvalid() error    { return oserror.ErrInvalid }
func errPermission() error { return oserror.ErrPermission }
func errExist() error      { return oserror.ErrExist }
func errNotExist() error   { return oserror.ErrNotExist }
func errClosed() error     { return oserror.ErrClosed }

type FileInfo interface {
	Name() string       // base name of the file，文件的名字
	Size() int64        // length in bytes for regular files; system-dependent for others，普通文件返回值表示其大小；其他文件的返回值含义各系统不同
	Mode() FileMode     // file mode bits，文件的模式位
	ModTime() time.Time // modification time，文件的修改时间
	IsDir() bool        // abbreviation for Mode().IsDir()，等价于Mode().IsDir()
	/*
		struct stat {
		    dev_t    st_dev;    // 设备 ID
		    ino_t    st_ino;    // 文件 i 节点号
		    mode_t    st_mode;    // 位掩码，文件类型和文件权限
		    nlink_t    st_nlink;    // 硬链接数
		    uid_t    st_uid;    // 文件属主，用户 ID
		    gid_t    st_gid;    // 文件属组，组 ID
		    dev_t    st_rdev;    // 如果针对设备 i 节点，则此字段包含主、辅 ID
		    off_t    st_size;    // 常规文件，则是文件字节数；符号链接，则是链接所指路径名的长度，字节为单位；对于共享内存对象，则是对象大小
		    blksize_t    st_blsize;    // 分配给文件的总块数，块大小为 512 字节
		    blkcnt_t    st_blocks;    // 实际分配给文件的磁盘块数量
		    time_t    st_atime;        // 对文件上次访问时间
		    time_t    st_mtime;        // 对文件上次修改时间
		    time_t    st_ctime;        // 文件状态发生改变的上次时间
		}
	*/
	Sys() interface{} // underlying data source (can return nil)，底层数据来源（可以返回nil）
}

/*
从左到右：
1、目录
2、
*/
type FileMode uint32

const (
	// The single letters are the abbreviations
	// used by the String method's formatting.
	// 单个字母是String方法的格式化所使用的缩略语。
	ModeDir        FileMode = 1 << (32 - 1 - iota) // d: is a directory，目录
	ModeAppend                                     // a: append-only，只能写入，且只能写入末尾
	ModeExclusive                                  // l: exclusive use，用于执行
	ModeTemporary                                  // T: temporary file; Plan 9 only，临时文件；仅在Plan 9
	ModeSymlink                                    // L: symbolic link，符号链接（不是快捷方式文件）
	ModeDevice                                     // D: device file，设备
	ModeNamedPipe                                  // p: named pipe (FIFO)，命名管道（FIFO）
	ModeSocket                                     // S: Unix domain socket，Unix域socket
	ModeSetuid                                     // u: setuid，设置uid
	ModeSetgid                                     // g: setgid，设置gid
	ModeCharDevice                                 // c: Unix character device, when ModeDevice is set，Unix字符设备，当ModeDevice设置为Unix时
	ModeSticky                                     // t: sticky，只有root/创建者能删除/移动文件
	ModeIrregular                                  // ?: non-regular file; nothing else is known about this file

	// Mask for the type bits. For regular files, none will be set.
	// 覆盖所有类型位，对于普通文件，所有这些位都不应该被设置
	ModeType = ModeDir | ModeSymlink | ModeNamedPipe | ModeSocket | ModeDevice | ModeCharDevice | ModeIrregular

	ModePerm FileMode = 0777 // Unix permission bits，覆盖所有Unix权限位
)

func (m FileMode) String() string {
	const str = "dalTLDpSugct?"
	var buf [32]byte // Mode is uint32.
	w := 0
	for i, c := range str {
		if m&(1<<uint(32-1-i)) != 0 {
			buf[w] = byte(c)
			w++
		}
	}
	/*
		w最后多加1的目的是最后截取
	*/
	if w == 0 {
		buf[w] = '-' // 文件
		w++
	}
	const rwx = "rwxrwxrwx"
	for i, c := range rwx {
		if m&(1<<uint(9-1-i)) != 0 {
			buf[w] = byte(c)
		} else {
			buf[w] = '-'
		}
		w++
	}
	return string(buf[:w])
}

// IsDir reports whether m describes a directory.
// That is, it tests for the ModeDir bit being set in m.
// IsDir报告m是否描述了一个目录。
// 也就是说，它测试m中的ModeDir位是否被设置。
func (m FileMode) IsDir() bool {
	return m&ModeDir != 0
}

// IsRegular reports whether m describes a regular file.
// That is, it tests that no mode type bits are set.
// IsRegular报告m是否描述了一个常规文件。
// 也就是说，它测试是否没有设置模式类型位。
func (m FileMode) IsRegular() bool {
	return m&ModeType == 0
}

// Perm returns the Unix permission bits in m (m & ModePerm).
// Perm返回m中的Unix权限位（m & ModePerm）。
func (m FileMode) Perm() FileMode {
	return m & ModePerm
}

// Type returns type bits in m (m & ModeType).
// Type返回m中的类型位（m & ModeType）。
func (m FileMode) Type() FileMode {
	return m & ModeType
}

// PathError records an error and the operation and file path that caused it.
// PathError记录了一个错误以及导致该错误的操作和文件路径。
type PathError struct {
	Op   string
	Path string
	Err  error
}

func (e *PathError) Error() string { return e.Op + " " + e.Path + ": " + e.Err.Error() }

func (e *PathError) Unwrap() error { return e.Err }

// Timeout reports whether this error represents a timeout.
// 超时报告这个错误是否代表超时。
func (e *PathError) Timeout() bool {
	t, ok := e.Err.(interface{ Timeout() bool })
	return ok && t.Timeout()
}
