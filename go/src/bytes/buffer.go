// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bytes

// Simple byte buffer for marshaling data.

import (
	"errors"
	"io"
	"unicode/utf8"
)

// smallBufferSize is an initial allocation minimal capacity.
// buffer最小分配单位，在grow(n)数组中被使用
const smallBufferSize = 64

// A Buffer is a variable-sized buffer of bytes with Read and Write methods.
// The zero value for Buffer is an empty buffer ready to use.
type Buffer struct {
	buf      []byte // contents are the bytes buf[off : len(buf)]				// 存放数据
	off      int    // read at &buf[off], write at &buf[len(buf)]				// 从哪开始读
	lastRead readOp // last read operation, so that Unread* can work correctly.	// 记录上一次的操作
}

// The readOp constants describe the last action performed on
// the buffer, so that UnreadRune and UnreadByte can check for
// invalid usage. opReadRuneX constants are chosen such that
// converted to int they correspond to the rune size that was read.
type readOp int8

// Don't use iota for these, as the values need to correspond with the
// names and comments, which is easier to see when being explicit.
const (
	opRead      readOp = -1 // Any other read operation.	// 任何其他读操作。
	opInvalid   readOp = 0  // Non-read operation.			// 没有读操作
	opReadRune1 readOp = 1  // Read rune of size 1.			// 读取大小为1的字符
	opReadRune2 readOp = 2  // Read rune of size 2.			// 读取大小为2的字符
	opReadRune3 readOp = 3  // Read rune of size 3.			// 读取大小为3的字符
	opReadRune4 readOp = 4  // Read rune of size 4.			// 读取大小为4的字符
)

// ErrTooLarge is passed to panic if memory cannot be allocated to store data in a buffer.
// bytes.Buffer太大错误
var ErrTooLarge = errors.New("bytes.Buffer: too large")

// 从Read中读出负数
var errNegativeRead = errors.New("bytes.Buffer: reader returned negative count from Read")

const maxInt = int(^uint(0) >> 1)

// Bytes returns a slice of length b.Len() holding the unread portion of the buffer.
// The slice is valid for use only until the next buffer modification (that is,
// only until the next call to a method like Read, Write, Reset, or Truncate).
// The slice aliases the buffer content at least until the next buffer modification,
// so immediate changes to the slice will affect the result of future reads.
// 返回剩下的字节
func (b *Buffer) Bytes() []byte { return b.buf[b.off:] }

// String returns the contents of the unread portion of the buffer
// as a string. If the Buffer is a nil pointer, it returns "<nil>".
//
// To build strings more efficiently, see the strings.Builder type.
// 转成字符串
func (b *Buffer) String() string {
	if b == nil {
		// Special case, useful in debugging.
		return "<nil>"
	}
	return string(b.buf[b.off:])
}

// empty reports whether the unread portion of the buffer is empty.
// 判断是否为空
func (b *Buffer) empty() bool { return len(b.buf) <= b.off }

// Len returns the number of bytes of the unread portion of the buffer;
// b.Len() == len(b.Bytes()).
// 计算长度
func (b *Buffer) Len() int { return len(b.buf) - b.off }

// Cap returns the capacity of the buffer's underlying byte slice, that is, the
// total space allocated for the buffer's data.
// 获取分配的内容大小
func (b *Buffer) Cap() int { return cap(b.buf) }

// Truncate discards all but the first n unread bytes from the buffer
// but continues to use the same allocated storage.
// It panics if n is negative or greater than the length of the buffer.
// n越界：panic，超出范围
// n=0，重置
// n正常，buf内容等于[:b.off+n]
func (b *Buffer) Truncate(n int) {
	if n == 0 {
		b.Reset()
		return
	}
	b.lastRead = opInvalid
	if n < 0 || n > b.Len() {
		panic("bytes.Buffer: truncation out of range")
	}
	b.buf = b.buf[:b.off+n]
}

// Reset resets the buffer to be empty,
// but it retains the underlying storage for use by future writes.
// Reset is the same as Truncate(0).
// 重置
func (b *Buffer) Reset() {
	b.buf = b.buf[:0]
	b.off = 0
	b.lastRead = opInvalid
}

// tryGrowByReslice is a inlineable version of grow for the fast-case where the
// internal buffer only needs to be resliced.
// It returns the index where bytes should be written and whether it succeeded.
// 尝试重新分配切片
// 如果写入n长度数据，不超出已分配的内存，则不需要重新分配
// 如果写入n长度数据，超出已分配的内存，则需要重新分配
func (b *Buffer) tryGrowByReslice(n int) (int, bool) {
	if l := len(b.buf); n <= cap(b.buf)-l {
		b.buf = b.buf[:l+n]
		return l, true
	}
	return 0, false
}

// grow grows the buffer to guarantee space for n more bytes.
// It returns the index where bytes should be written.
// If the buffer can't grow it will panic with ErrTooLarge.
// 扩展缓冲区
/*
	1、读取buf的长度
	2、如果buf的长度为0，且偏移不为0，则重置
		buf长度置空
		偏移置0
		设置为没有读操作
	3、尝试重新裁剪，如果长度够，则直接返回裁剪后的长度
	4、如果buf为空，且n小于buf的最小长度64，则按64进行分配
	5、如果 已使用+要使用的 还有到预分配的一般，则把已读部分清空
	6、如果要分配的大小，超出了int最大，则抛出 太大错误
	7、其他情况，则buf按 2*c+n 的长度进行分配，并把之前偏移后的复制过去，也就是把未读的拿过去，已读的数据抛弃掉
	8、偏移量置0
	9、buf的len部分设为增长后的实际长度
*/
func (b *Buffer) grow(n int) int {
	m := b.Len()
	// If buffer is empty, reset to recover space.
	// 如果缓冲区的长度为空，且则当前读取位置不为0，则重置
	if m == 0 && b.off != 0 {
		b.Reset()
	}
	// Try to grow by means of a reslice.
	// 尝试通过重新裁切来增长
	if i, ok := b.tryGrowByReslice(n); ok {
		// 缓冲区长度
		return i
	}
	if b.buf == nil && n <= smallBufferSize {
		b.buf = make([]byte, n, smallBufferSize)
		return 0
	}
	c := cap(b.buf)
	// 已使用+要使用的还没到一半，则把已读部分清空
	if n <= c/2-m {
		// We can slide things down instead of allocating a new
		// slice. We only need m+n <= c to slide, but
		// we instead let capacity get twice as large so we
		// don't spend all our time copying.
		// 把已读的部分清空
		copy(b.buf, b.buf[b.off:])
	} else if c > maxInt-c-n {
		panic(ErrTooLarge)
	} else {
		// Not enough space anywhere, we need to allocate.
		// 如果要分配的大小，超出了int最大，则抛出 太大错误
		buf := makeSlice(2*c + n)
		copy(buf, b.buf[b.off:])
		b.buf = buf
	}
	// Restore b.off and len(b.buf).
	b.off = 0
	b.buf = b.buf[:m+n]
	return m
}

// Grow grows the buffer's capacity, if necessary, to guarantee space for
// another n bytes. After Grow(n), at least n bytes can be written to the
// buffer without another allocation.
// If n is negative, Grow will panic.
// If the buffer can't grow it will panic with ErrTooLarge.
// 申请扩展缓冲区
func (b *Buffer) Grow(n int) {
	if n < 0 {
		panic("bytes.Buffer.Grow: negative count")
	}
	m := b.grow(n)
	b.buf = b.buf[:m]
}

// Write appends the contents of p to the buffer, growing the buffer as
// needed. The return value n is the length of p; err is always nil. If the
// buffer becomes too large, Write will panic with ErrTooLarge.
// 写入数据
// 先判断当前空间还能不能写下，如果不能写下，则扩展缓冲区，之后再写入
func (b *Buffer) Write(p []byte) (n int, err error) {
	b.lastRead = opInvalid
	m, ok := b.tryGrowByReslice(len(p))
	if !ok {
		m = b.grow(len(p))
	}
	return copy(b.buf[m:], p), nil
}

// WriteString appends the contents of s to the buffer, growing the buffer as
// needed. The return value n is the length of s; err is always nil. If the
// buffer becomes too large, WriteString will panic with ErrTooLarge.
/*
	向buf写入数据s
	1、因为现在是在写数据，所以清空读记录
	2、判断是否需要增长切片
	3、如果需要增长，则增长
	4、拷贝数据过去
*/
func (b *Buffer) WriteString(s string) (n int, err error) {
	b.lastRead = opInvalid
	m, ok := b.tryGrowByReslice(len(s))
	if !ok {
		m = b.grow(len(s))
	}
	return copy(b.buf[m:], s), nil
}

// MinRead is the minimum slice size passed to a Read call by
// Buffer.ReadFrom. As long as the Buffer has at least MinRead bytes beyond
// what is required to hold the contents of r, ReadFrom will not grow the
// underlying buffer.
const MinRead = 512

// ReadFrom reads data from r until EOF and appends it to the buffer, growing
// the buffer as needed. The return value n is the number of bytes read. Any
// error except io.EOF encountered during the read is also returned. If the
// buffer becomes too large, ReadFrom will panic with ErrTooLarge.
func (b *Buffer) ReadFrom(r io.Reader) (n int64, err error) {
	b.lastRead = opInvalid
	for {
		i := b.grow(MinRead)
		b.buf = b.buf[:i]
		m, e := r.Read(b.buf[i:cap(b.buf)])
		if m < 0 {
			panic(errNegativeRead)
		}

		b.buf = b.buf[:i+m]
		n += int64(m)
		if e == io.EOF {
			return n, nil // e is EOF, so return nil explicitly
		}
		if e != nil {
			return n, e
		}
	}
}

// makeSlice allocates a slice of size n. If the allocation fails, it panics
// with ErrTooLarge.
// 分配一个长度为n的切片
func makeSlice(n int) []byte {
	// If the make fails, give a known error.
	defer func() {
		if recover() != nil {
			panic(ErrTooLarge)
		}
	}()
	return make([]byte, n)
}

// WriteTo writes data to w until the buffer is drained or an error occurs.
// The return value n is the number of bytes written; it always fits into an
// int, but it is int64 to match the io.WriterTo interface. Any error
// encountered during the write is also returned.
func (b *Buffer) WriteTo(w io.Writer) (n int64, err error) {
	b.lastRead = opInvalid
	if nBytes := b.Len(); nBytes > 0 {
		m, e := w.Write(b.buf[b.off:])
		if m > nBytes {
			panic("bytes.Buffer.WriteTo: invalid Write count")
		}
		b.off += m
		n = int64(m)
		if e != nil {
			return n, e
		}
		// all bytes should have been written, by definition of
		// Write method in io.Writer
		// 根据io.Writer中Write方法的定义，所有的字节都应该被写入。
		if m != nBytes {
			return n, io.ErrShortWrite
		}
	}
	// Buffer is now empty; reset.
	b.Reset()
	return n, nil
}

// WriteByte appends the byte c to the buffer, growing the buffer as needed.
// The returned error is always nil, but is included to match bufio.Writer's
// WriteByte. If the buffer becomes too large, WriteByte will panic with
// ErrTooLarge.
// WriteByte将字节c追加到缓冲区中，根据需要增加缓冲区。返回的错误总是nil，但被包括在内以匹配
// bufio.Writer的WriteByte。如果缓冲区变得太大，WriteByte将以ErrTooLarge panic。
func (b *Buffer) WriteByte(c byte) error {
	b.lastRead = opInvalid
	m, ok := b.tryGrowByReslice(1)
	if !ok {
		m = b.grow(1)
	}
	b.buf[m] = c
	return nil
}

// WriteRune appends the UTF-8 encoding of Unicode code point r to the
// buffer, returning its length and an error, which is always nil but is
// included to match bufio.Writer's WriteRune. The buffer is grown as needed;
// if it becomes too large, WriteRune will panic with ErrTooLarge.
// WriteRune将Unicode代码点r的UTF-8编码附加到缓冲区，返回其长度和一个错误，
// 这个错误总是为零，但被包括在内以匹配bufio.Writer的WriteRune。
// 缓冲区根据需要增长；如果它变得太大，WriteRune将以ErrTooLarge panic失措。
func (b *Buffer) WriteRune(r rune) (n int, err error) {
	// Compare as uint32 to correctly handle negative runes.
	if uint32(r) < utf8.RuneSelf {
		b.WriteByte(byte(r))
		return 1, nil
	}
	b.lastRead = opInvalid
	m, ok := b.tryGrowByReslice(utf8.UTFMax)
	if !ok {
		m = b.grow(utf8.UTFMax)
	}
	n = utf8.EncodeRune(b.buf[m:m+utf8.UTFMax], r)
	b.buf = b.buf[:m+n]
	return n, nil
}

// Read reads the next len(p) bytes from the buffer or until the buffer
// is drained. The return value n is the number of bytes read. If the
// buffer has no data to return, err is io.EOF (unless len(p) is zero);
// otherwise it is nil.
// Read从缓冲区中读取下一个len(p)字节或直到缓冲区被耗尽。返回值n是读取的字节数。
// 如果缓冲区没有数据返回，err为io.EOF（除非len(p)为零）；否则为nil。
func (b *Buffer) Read(p []byte) (n int, err error) {
	b.lastRead = opInvalid
	if b.empty() {
		// Buffer is empty, reset to recover space.
		// 缓冲区是空的，重置为恢复空间。
		b.Reset()
		if len(p) == 0 {
			return 0, nil
		}
		return 0, io.EOF
	}
	n = copy(p, b.buf[b.off:])
	b.off += n
	if n > 0 {
		b.lastRead = opRead
	}
	return n, nil
}

// Next returns a slice containing the next n bytes from the buffer,
// advancing the buffer as if the bytes had been returned by Read.
// If there are fewer than n bytes in the buffer, Next returns the entire buffer.
// The slice is only valid until the next call to a read or write method.
// Next返回一个包含缓冲区下一个n个字节的片断，推进缓冲区，就像这些字节是由Read返回的一样。
// 如果缓冲区中的字节数少于n，Next将返回整个缓冲区。这个片断只在下次调用读或写方法之前有效。
func (b *Buffer) Next(n int) []byte {
	b.lastRead = opInvalid
	m := b.Len()
	if n > m {
		n = m
	}
	data := b.buf[b.off : b.off+n]
	b.off += n
	if n > 0 {
		b.lastRead = opRead
	}
	return data
}

// ReadByte reads and returns the next byte from the buffer.
// If no byte is available, it returns error io.EOF.
// ReadByte 读取并返回缓冲区的下一个字节。如果没有可用的字节，它会返回错误io.EOF。
func (b *Buffer) ReadByte() (byte, error) {
	if b.empty() {
		// Buffer is empty, reset to recover space.
		// 缓冲区是空的，重置为恢复空间。
		b.Reset()
		return 0, io.EOF
	}
	c := b.buf[b.off]
	b.off++
	b.lastRead = opRead
	return c, nil
}

// ReadRune reads and returns the next UTF-8-encoded
// Unicode code point from the buffer.
// If no bytes are available, the error returned is io.EOF.
// If the bytes are an erroneous UTF-8 encoding, it
// consumes one byte and returns U+FFFD, 1.
// ReadRune从缓冲区中读取并返回下一个UTF-8编码的Unicode码位。
// 如果没有可用的字节，返回的错误是io.EOF。如果字节是错误的UTF-8编码，
// 它将消耗一个字节并返回U+FFFD，1。
func (b *Buffer) ReadRune() (r rune, size int, err error) {
	if b.empty() {
		// Buffer is empty, reset to recover space.
		// 缓冲区是空的，重置为恢复空间。
		b.Reset()
		return 0, 0, io.EOF
	}
	c := b.buf[b.off]
	if c < utf8.RuneSelf {
		b.off++
		b.lastRead = opReadRune1
		return rune(c), 1, nil
	}
	r, n := utf8.DecodeRune(b.buf[b.off:])
	b.off += n
	b.lastRead = readOp(n)
	return r, n, nil
}

// UnreadRune unreads the last rune returned by ReadRune.
// If the most recent read or write operation on the buffer was
// not a successful ReadRune, UnreadRune returns an error.  (In this regard
// it is stricter than UnreadByte, which will unread the last byte
// from any read operation.)
// UnreadRune取消读取ReadRune返回的最后一个符文。如果最近对缓冲区的读或写操作不是一个成功
// 的ReadRune，UnreadRune会返回一个错误。 (在这方面它比UnreadByte更严格，UnreadByte
// 会取消任何读操作的最后一个字节)。
func (b *Buffer) UnreadRune() error {
	if b.lastRead <= opInvalid {
		return errors.New("bytes.Buffer: UnreadRune: previous operation was not a successful ReadRune")
	}
	if b.off >= int(b.lastRead) {
		b.off -= int(b.lastRead)
	}
	b.lastRead = opInvalid
	return nil
}

var errUnreadByte = errors.New("bytes.Buffer: UnreadByte: previous operation was not a successful read")

// UnreadByte unreads the last byte returned by the most recent successful
// read operation that read at least one byte. If a write has happened since
// the last read, if the last read returned an error, or if the read read zero
// bytes, UnreadByte returns an error.
// UnreadRune解读由ReadRune返回的最后一个符文。如果最近在缓冲区上的读或写操作不是一个成功的ReadRune，
// UnreadRune会返回一个错误。 (在这方面它比UnreadByte更严格，UnreadByte会取消任何读操作的最后一个字节)。
func (b *Buffer) UnreadByte() error {
	if b.lastRead == opInvalid {
		return errUnreadByte
	}
	b.lastRead = opInvalid
	if b.off > 0 {
		b.off--
	}
	return nil
}

// ReadBytes reads until the first occurrence of delim in the input,
// returning a slice containing the data up to and including the delimiter.
// If ReadBytes encounters an error before finding a delimiter,
// it returns the data read before the error and the error itself (often io.EOF).
// ReadBytes returns err != nil if and only if the returned data does not end in
// delim.
// ReadBytes一直读到输入中第一次出现delim为止，返回一个包含数据的片断，直到并包括分界符。
// 如果ReadBytes在找到定界符之前遇到了错误，它会返回在错误之前读到的数据和错误本身（通常是io.EOF）。
// 如果且仅当返回的数据不以定界符结束时，ReadBytes返回err !=nil。
func (b *Buffer) ReadBytes(delim byte) (line []byte, err error) {
	slice, err := b.readSlice(delim)
	// return a copy of slice. The buffer's backing array may
	// be overwritten by later calls.
	line = append(line, slice...)
	return line, err
}

// readSlice is like ReadBytes but returns a reference to internal buffer data.
// readSlice与ReadBytes类似，但返回一个对内部缓冲区数据的引用。
func (b *Buffer) readSlice(delim byte) (line []byte, err error) {
	i := IndexByte(b.buf[b.off:], delim)
	end := b.off + i + 1
	if i < 0 {
		end = len(b.buf)
		err = io.EOF
	}
	line = b.buf[b.off:end]
	b.off = end
	b.lastRead = opRead
	return line, err
}

// ReadString reads until the first occurrence of delim in the input,
// returning a string containing the data up to and including the delimiter.
// If ReadString encounters an error before finding a delimiter,
// it returns the data read before the error and the error itself (often io.EOF).
// ReadString returns err != nil if and only if the returned data does not end
// in delim.
// ReadString 读取到输入中第一次出现的 delim，返回一个包含数据的字符串，直到并包括分界符。
// 如果ReadString在找到定界符之前遇到了错误，它会返回在错误之前读到的数据和错误本身（通常是io.EOF）。
// 如果且仅当返回的数据不以定界符结束时，ReadString返回err !=nil。
func (b *Buffer) ReadString(delim byte) (line string, err error) {
	slice, err := b.readSlice(delim)
	return string(slice), err
}

// NewBuffer creates and initializes a new Buffer using buf as its
// initial contents. The new Buffer takes ownership of buf, and the
// caller should not use buf after this call. NewBuffer is intended to
// prepare a Buffer to read existing data. It can also be used to set
// the initial size of the internal buffer for writing. To do that,
// buf should have the desired capacity but a length of zero.
// NewBuffer使用buf作为初始内容创建并初始化一个新的缓冲区。
// 新的缓冲区拥有buf的所有权，调用者在这个调用之后不应该使用buf。
// NewBuffer的目的是准备一个缓冲区来读取现有的数据。
// 它还可用于设置用于写入的内部缓冲区的初始大小。要做到这一点，BUF应该具有所需的容量，但长度应为零。
//
// In most cases, new(Buffer) (or just declaring a Buffer variable) is
// sufficient to initialize a Buffer.
// 在大多数情况下，new(Buffer)（或者只是声明一个Buffer变量）就足以初始化一个Buffer。
func NewBuffer(buf []byte) *Buffer { return &Buffer{buf: buf} }

// NewBufferString creates and initializes a new Buffer using string s as its
// initial contents. It is intended to prepare a buffer to read an existing
// string.
// NewBufferString创建并初始化一个新的Buffer，使用字符串s作为其初始内容。它的目的是为了准备一个缓冲区来读取一个现有的字符串。
//
// In most cases, new(Buffer) (or just declaring a Buffer variable) is
// sufficient to initialize a Buffer.
// 在大多数情况下，new(Buffer)（或者只是声明一个Buffer变量）就足以初始化一个Buffer。
func NewBufferString(s string) *Buffer {
	return &Buffer{buf: []byte(s)}
}
