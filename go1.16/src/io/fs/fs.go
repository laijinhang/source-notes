package fs

import (
	"internal/oserror"
	"time"
	"unicode/utf8"
)

type FS interface {
	Open(name string) (File, error)
}

func ValidPath(name string) bool {
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
	ErrInvalid    = errInvalid()    // "invalid argument"
	ErrPermission = errPermission() // "permission denied"
	ErrExist      = errExist()      // "file already exists"
	ErrNotExist   = errNotExist()   // "file does not exist"
	ErrClosed     = errClosed()     // "file already closed"
)

func errInvalid() error    { return oserror.ErrInvalid }
func errPermission() error { return oserror.ErrPermission }
func errExist() error      { return oserror.ErrExist }
func errNotExist() error   { return oserror.ErrNotExist }
func errClosed() error     { return oserror.ErrClosed }

type FileInfo interface {
	Name() string       // base name of the file
	Size() int64        // length in bytes for regular files; system-dependent for others
	Mode() FileMode     // file mode bits
	ModTime() time.Time // modification time
	IsDir() bool        // abbreviation for Mode().IsDir()
	Sys() interface{}   // underlying data source (can return nil)
}

type FileMode uint32

const (
	// The single letters are the abbreviations
	// used by the String method's formatting.
	ModeDir        FileMode = 1 << (32 - 1 - iota) // d: is a directory
	ModeAppend                                     // a: append-only
	ModeExclusive                                  // l: exclusive use
	ModeTemporary                                  // T: temporary file; Plan 9 only
	ModeSymlink                                    // L: symbolic link
	ModeDevice                                     // D: device file
	ModeNamedPipe                                  // p: named pipe (FIFO)
	ModeSocket                                     // S: Unix domain socket
	ModeSetuid                                     // u: setuid
	ModeSetgid                                     // g: setgid
	ModeCharDevice                                 // c: Unix character device, when ModeDevice is set
	ModeSticky                                     // t: sticky
	ModeIrregular                                  // ?: non-regular file; nothing else is known about this file

	// Mask for the type bits. For regular files, none will be set.
	ModeType = ModeDir | ModeSymlink | ModeNamedPipe | ModeSocket | ModeDevice | ModeCharDevice | ModeIrregular

	ModePerm FileMode = 0777 // Unix permission bits
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
	if w == 0 {
		buf[w] = '-'
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
func (m FileMode) IsDir() bool {
	return m&ModeDir != 0
}

// IsRegular reports whether m describes a regular file.
// That is, it tests that no mode type bits are set.
func (m FileMode) IsRegular() bool {
	return m&ModeType == 0
}

// Perm returns the Unix permission bits in m (m & ModePerm).
func (m FileMode) Perm() FileMode {
	return m & ModePerm
}

// Type returns type bits in m (m & ModeType).
func (m FileMode) Type() FileMode {
	return m & ModeType
}

// PathError records an error and the operation and file path that caused it.
type PathError struct {
	Op   string
	Path string
	Err  error
}

func (e *PathError) Error() string { return e.Op + " " + e.Path + ": " + e.Err.Error() }

func (e *PathError) Unwrap() error { return e.Err }

// Timeout reports whether this error represents a timeout.
func (e *PathError) Timeout() bool {
	t, ok := e.Err.(interface{ Timeout() bool })
	return ok && t.Timeout()
}
