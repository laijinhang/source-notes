// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bytes

import (
	"errors"
	"io"
	"unicode/utf8"
)

// A Reader implements the io.Reader, io.ReaderAt, io.WriterTo, io.Seeker,
// io.ByteScanner, and io.RuneScanner interfaces by reading from
// a byte slice.
// Unlike a Buffer, a Reader is read-only and supports seeking.
// The zero value for Reader operates like a Reader of an empty slice.

// 读取器通过读取字节片实现io.Reader, io.ReaderAt, io.WriterTo, io.Seeker,
// io.ByteScanner和io.RuneScanner接口。与Buffer不同，Reader是只读的，
// 并且支持寻址。读取器的零值与空片的读取器操作类似。
type Reader struct {
	s        []byte
	i        int64 // current reading index				// 当前读的索引位置
	prevRune int   // index of previous rune; or < 0	// 前一个符文的索引；或 < 0
}

// Len returns the number of bytes of the unread portion of the
// slice.
// Len返回切片未读部分的字节数。
func (r *Reader) Len() int {
	if r.i >= int64(len(r.s)) {
		return 0
	}
	return int(int64(len(r.s)) - r.i)
}

// Size returns the original length of the underlying byte slice.
// Size is the number of bytes available for reading via ReadAt.
// The returned value is always the same and is not affected by calls
// to any other method.
// Size返回底层字节片的原始长度。Size是可通过ReadAt读取的字节数。
// 返回值总是相同的，并且不受调用任何其他方法的影响。
func (r *Reader) Size() int64 { return int64(len(r.s)) }

// Read implements the io.Reader interface.
// 读取实现了io.Reader接口。
func (r *Reader) Read(b []byte) (n int, err error) {
	if r.i >= int64(len(r.s)) {
		return 0, io.EOF
	}
	r.prevRune = -1
	n = copy(b, r.s[r.i:])
	r.i += int64(n)
	return
}

// ReadAt implements the io.ReaderAt interface.
// ReadAt 实现了 io.ReaderAt 接口。
func (r *Reader) ReadAt(b []byte, off int64) (n int, err error) {
	// cannot modify state - see io.ReaderAt
	// 不能修改状态--见io.ReaderAt
	if off < 0 {
		return 0, errors.New("bytes.Reader.ReadAt: negative offset")
	}
	if off >= int64(len(r.s)) {
		return 0, io.EOF
	}
	n = copy(b, r.s[off:])
	if n < len(b) {
		err = io.EOF
	}
	return
}

// ReadByte implements the io.ByteReader interface.
// ReadByte 实现了 io.ByteReader 接口。
func (r *Reader) ReadByte() (byte, error) {
	r.prevRune = -1
	if r.i >= int64(len(r.s)) {
		return 0, io.EOF
	}
	b := r.s[r.i]
	r.i++
	return b, nil
}

// UnreadByte complements ReadByte in implementing the io.ByteScanner interface.
// UnreadByte在实现io.ByteScanner接口时补充了ReadByte。
func (r *Reader) UnreadByte() error {
	if r.i <= 0 {
		return errors.New("bytes.Reader.UnreadByte: at beginning of slice")
	}
	r.prevRune = -1
	r.i--
	return nil
}

// ReadRune implements the io.RuneReader interface.
// ReadRune 实现了 io.RuneReader 接口。
func (r *Reader) ReadRune() (ch rune, size int, err error) {
	if r.i >= int64(len(r.s)) {
		r.prevRune = -1
		return 0, 0, io.EOF
	}
	r.prevRune = int(r.i)
	if c := r.s[r.i]; c < utf8.RuneSelf {
		r.i++
		return rune(c), 1, nil
	}
	ch, size = utf8.DecodeRune(r.s[r.i:])
	r.i += int64(size)
	return
}

// UnreadRune complements ReadRune in implementing the io.RuneScanner interface.
// UnreadRune在实现io.RuneScanner接口时是对ReadRune的补充。
func (r *Reader) UnreadRune() error {
	if r.i <= 0 {
		return errors.New("bytes.Reader.UnreadRune: at beginning of slice")
	}
	if r.prevRune < 0 {
		return errors.New("bytes.Reader.UnreadRune: previous operation was not ReadRune")
	}
	r.i = int64(r.prevRune)
	r.prevRune = -1
	return nil
}

// Seek implements the io.Seeker interface.
// Seek实现了io.Seeker接口。
func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	r.prevRune = -1
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = r.i + offset
	case io.SeekEnd:
		abs = int64(len(r.s)) + offset
	default:
		return 0, errors.New("bytes.Reader.Seek: invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("bytes.Reader.Seek: negative position")
	}
	r.i = abs
	return abs, nil
}

// WriteTo implements the io.WriterTo interface.
// WriteTo 实现了 io.WriterTo 接口。
func (r *Reader) WriteTo(w io.Writer) (n int64, err error) {
	r.prevRune = -1
	if r.i >= int64(len(r.s)) {
		return 0, nil
	}
	b := r.s[r.i:]
	m, err := w.Write(b)
	if m > len(b) {
		panic("bytes.Reader.WriteTo: invalid Write count")
	}
	r.i += int64(m)
	n = int64(m)
	if m != len(b) && err == nil {
		err = io.ErrShortWrite
	}
	return
}

// Reset resets the Reader to be reading from b.
// Reset 重置Reader，使其从b中读取。
func (r *Reader) Reset(b []byte) { *r = Reader{b, 0, -1} }

// NewReader returns a new Reader reading from b.
// NewReader返回一个新的Reader，从b中读取信息。
func NewReader(b []byte) *Reader { return &Reader{b, 0, -1} }
