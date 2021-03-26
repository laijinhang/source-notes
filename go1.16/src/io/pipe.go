// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
pipe包内部先是实现了一个可读可写的pipe，pipe被翻译成管道，数据从写的地方流入，从读的地方流出，
用chan来实现数据流转，并配合互斥锁，达到并发安全，使用sync.Once来执行关闭操作，使得多次调用，
最多只执行一次。然后再封装了只写pipe和只读pipe，用Pipe创建的只读pipe和只写pipe是同一对，
他们底层是共用一个pipe
*/
// Pipe adapter to connect code expecting an io.Reader
// with code expecting an io.Writer.
package io

import (
	"errors"
	"sync"
)

// onceError is an object that will only store an error once.
// OnceError是一个只会存储一次错误的对象。也就是存储第一个传入不为空的err
type onceError struct {
	sync.Mutex // guards following，继承互斥锁
	err        error
}

func (a *onceError) Store(err error) {
	a.Lock()
	defer a.Unlock()
	if a.err != nil {
		return
	}
	a.err = err
}
func (a *onceError) Load() error {
	a.Lock()
	defer a.Unlock()
	return a.err
}

// ErrClosedPipe is the error used for read or write operations on a closed pipe.
// ErrClosedPipe是用于在已关闭的Pipe上进行读取或写入操作。
var ErrClosedPipe = errors.New("io: read/write on closed pipe")

// A pipe is the shared pipe structure underlying PipeReader and PipeWriter.
// pipe是PipeReader和PipeWriter底层的共享管道结构。
/*
 Pipe() (*PipeReader, *PipeWriter)的实现是先创建一个pipe对象，然后把这个对象赋值给 PipeReader 和 PipeWriter对象，
也就说，用Pipe()方法创建的只读Pipe和只写Pipe其实是同一个pipe，不过一个是屏蔽了写操作，一个是屏蔽了读操作
*/
type pipe struct {
	// 序列化写操作 使使用
	wrMu sync.Mutex // Serializes Write operations
	wrCh chan []byte
	rdCh chan int

	// 确保关闭操作只执行一次
	once sync.Once // Protects closing done
	done chan struct{}
	rerr onceError
	werr onceError
}

func (p *pipe) Read(b []byte) (n int, err error) {
	select {
	case <-p.done:
		return 0, p.readCloseError()
	default:
	}

	select {
	case bw := <-p.wrCh:
		nr := copy(b, bw)
		p.rdCh <- nr
		return nr, nil
	case <-p.done:
		return 0, p.readCloseError()
	}
}

func (p *pipe) readCloseError() error {
	rerr := p.rerr.Load()
	if werr := p.werr.Load(); rerr == nil && werr != nil {
		return werr
	}
	return ErrClosedPipe
}

func (p *pipe) CloseRead(err error) error {
	if err == nil {
		err = ErrClosedPipe
	}
	p.rerr.Store(err)
	p.once.Do(func() { close(p.done) })
	return nil
}

func (p *pipe) Write(b []byte) (n int, err error) {
	select {
	case <-p.done: // 如果写操作已关闭，返回写已关闭错误
		return 0, p.writeCloseError()
	default:
		p.wrMu.Lock()
		defer p.wrMu.Unlock()
	}

	for once := true; once || len(b) > 0; once = false {
		select {
		/*
			确保写入的数据能被读完，虽然b字节切片可以一次性把全部数据丢进chan，
			但是读并不能保证一次性全部读完，读的长度是按传入到Read的字节切片来，
			Read内部并不会为这个字节切片重新分配内存，而是调用copy进行拷贝数据，
			也就是说读取的长度是根据传入的字节切片来决定的
		*/
		case p.wrCh <- b:
			nw := <-p.rdCh
			b = b[nw:]
			n += nw
		case <-p.done:
			return n, p.writeCloseError()
		}
	}
	return n, nil
}

func (p *pipe) writeCloseError() error {
	werr := p.werr.Load()
	if rerr := p.rerr.Load(); werr == nil && rerr != nil {
		return rerr
	}
	return ErrClosedPipe
}

func (p *pipe) CloseWrite(err error) error {
	if err == nil {
		err = EOF
	}
	p.werr.Store(err)
	// 如果存在多次掉用的情况下，只执行一次关闭pipe操作
	p.once.Do(func() { close(p.done) })
	return nil
}

// A PipeReader is the read half of a pipe.
// PipeReader是一个只读的pipe
type PipeReader struct {
	p *pipe
}

// Read implements the standard Read interface:
// it reads data from the pipe, blocking until a writer
// arrives or the write end is closed.
// If the write end is closed with an error, that error is
// returned as err; otherwise err is EOF.
// Read实现标准的Read接口：
// 它从管道读取数据，阻塞直到写入器到达或写入端关闭。
// 如果写端由于错误而关闭，则该错误将以err的形式返回；
// 否则，该错误将作为错误返回。否则err为EOF。
func (r *PipeReader) Read(data []byte) (n int, err error) {
	return r.p.Read(data)
}

// Close closes the reader; subsequent writes to the
// write half of the pipe will return the error ErrClosedPipe.
// Close关闭阅读器；随后写入管道的一半写入操作将返回错误ErrClosedPipe。
func (r *PipeReader) Close() error {
	return r.CloseWithError(nil)
}

// CloseWithError closes the reader; subsequent writes
// to the write half of the pipe will return the error err.
// CloseWithError关闭阅读器；随后写入管道的一半将返回错误err。
//
// CloseWithError never overwrites the previous error if it exists
// and always returns nil.
// CloseWithError永远不会覆盖以前的错误（如果存在）
// 并始终返回nil。
func (r *PipeReader) CloseWithError(err error) error {
	return r.p.CloseRead(err)
}

// A PipeWriter is the write half of a pipe.
// PipeReader是一个只写的pipe
type PipeWriter struct {
	p *pipe
}

// Write implements the standard Write interface:
// it writes data to the pipe, blocking until one or more readers
// have consumed all the data or the read end is closed.
// If the read end is closed with an error, that err is
// returned as err; otherwise err is ErrClosedPipe.
// Write实现标准的Write接口：
// 将数据写入管道，直到一个或多个读取器使用完所有数据或关闭读取端为止，它一直阻塞。
// 如果读取端因错误而关闭，则该err为
// 返回为err;否则err是ErrClosedPipe。
func (w *PipeWriter) Write(data []byte) (n int, err error) {
	return w.p.Write(data)
}

// Close closes the writer; subsequent reads from the
// read half of the pipe will return no bytes and EOF.
// Close关闭写操作；
// 从管道读取的后续读取将不返回任何字节和EOF。
func (w *PipeWriter) Close() error {
	return w.CloseWithError(nil)
}

// CloseWithError closes the writer; subsequent reads from the
// read half of the pipe will return no bytes and the error err,
// or EOF if err is nil.
//
// CloseWithError never overwrites the previous error if it exists
// and always returns nil.

// CloseWithError关闭写操作；
// 从管道读取的一半进行的后续读取将不返回任何字节，并且错误err；
// 如果err为nil，则返回EOF。
//
// CloseWithError永远不会覆盖以前的错误（如果存在），并且始终返回nil。
func (w *PipeWriter) CloseWithError(err error) error {
	return w.p.CloseWrite(err)
}

// Pipe creates a synchronous in-memory pipe.
// It can be used to connect code expecting an io.Reader
// with code expecting an io.Writer.
// Pipe创建一个同步的内存管道。它可以用来连接期望io.Reader的代码和期望io.Writer的代码。
//
// Reads and Writes on the pipe are matched one to one
// except when multiple Reads are needed to consume a single Write.
// That is, each Write to the PipeWriter blocks until it has satisfied
// one or more Reads from the PipeReader that fully consume
// the written data.
// 管道上的读取和写入是一对一匹配的，除非需要多个读取来消耗单个写入。
// 也就是说，每次对PipeWriter的写入都将阻塞，
// 直到它满足从PipeReader读取的一个或多个读取，这些读取会完全消耗已写入的数据。
// The data is copied directly from the Write to the corresponding
// Read (or Reads); there is no internal buffering.
// 将数据直接从Write复制到相应的Read（或多个Read）；没有内部缓冲。
//
// It is safe to call Read and Write in parallel with each other or with Close.
// 可以并发或以关闭方式并发调用读取和写入是安全的。
// Parallel calls to Read and parallel calls to Write are also safe:
// 并发调用Read和Write也是安全的：
// the individual calls will be gated sequentially.
func Pipe() (*PipeReader, *PipeWriter) {
	p := &pipe{
		wrCh: make(chan []byte),
		rdCh: make(chan int),
		done: make(chan struct{}),
	}
	return &PipeReader{p}, &PipeWriter{p}
}
