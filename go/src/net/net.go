// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package net provides a portable interface for network I/O, including
TCP/IP, UDP, domain name resolution, and Unix domain sockets.
包net为网络I/O提供了一个可移植的接口，包括 TCP/IP、UDP、域名解析和Unix域套接字。

Although the package provides access to low-level networking
primitives, most clients will need only the basic interface provided
by the Dial, Listen, and Accept functions and the associated
Conn and Listener interfaces. The crypto/tls package uses
the same interfaces and similar Dial and Listen functions.
尽管该软件包提供了对低级网络基元的访问的基元，但大多数客户只需要由Dial、Listen和
Accept函数提供的基本接口和相关的拨号、监听和接受函数以及相关的 Conn和Listener接口。
crypto/tls包使用 相同的接口和类似的拨号和监听函数。

The Dial function connects to a server:
拨号功能连接到一个服务器：

	conn, err := net.Dial("tcp", "golang.org:80")
	if err != nil {
		// handle error
	}
	fmt.Fprintf(conn, "GET / HTTP/1.0\r\n\r\n")
	status, err := bufio.NewReader(conn).ReadString('\n')
	// ...

The Listen function creates servers:
创建一个监听服务器：

	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		// handle error
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			// handle error
		}
		go handleConnection(conn)
	}

Name Resolution
域名解析

The method for resolving domain names, whether indirectly with functions like Dial
or directly with functions like LookupHost and LookupAddr, varies by operating system.
解决域名的方法，无论是间接使用Dial或直接使用LookupHost和LookupAddr等函数，都因操作系统而异。

On Unix systems, the resolver has two options for resolving names.
It can use a pure Go resolver that sends DNS requests directly to the servers
listed in /etc/resolv.conf, or it can use a cgo-based resolver that calls C
library routines such as getaddrinfo and getnameinfo.
在 Unix 系统上，解析器有两个选择来解析名字。
它可以使用一个纯粹的Go解析器，直接向/etc/resolv.conf中列出的服务器发送DNS请求。
中列出的服务器，或者可以使用基于 cgo 的解析器，调用 C 库例程，如 getaddrinfo 和 getnameinfo。

By default the pure Go resolver is used, because a blocked DNS request consumes
only a goroutine, while a blocked C call consumes an operating system thread.
When cgo is available, the cgo-based resolver is used instead under a variety of
conditions: on systems that do not let programs make direct DNS requests (OS X),
when the LOCALDOMAIN environment variable is present (even if empty),
when the RES_OPTIONS or HOSTALIASES environment variable is non-empty,
when the ASR_CONFIG environment variable is non-empty (OpenBSD only),
when /etc/resolv.conf or /etc/nsswitch.conf specify the use of features that the
Go resolver does not implement, and when the name being looked up ends in .local
or is an mDNS name.
默认情况下，使用的是纯Go解析器，因为阻塞的DNS请求只消耗一个goroutine 而阻塞的 C 调用则会消耗一个操作系统线程。
当cgo可用时，基于cgo的解析器会在各种情况下被使用。在不允许程序直接进行DNS请求的系统上（OS X）。当LOCALDOMAIN
环境变量存在时（即使是空的）。当RES_OPTIONS或HOSTALIASES环境变量为非空时。当 ASR_CONFIG 环境变量为非空时(仅 OpenBSD)。
当 /etc/resolv.conf 或 /etc/nsswitch.conf 指定使用 Go 解析器没有实现的功能时。Go解析器没有实现的功能，
以及被查询的名称以.local结尾或为mDNS名称时。结尾或为 mDNS 名称时。

The resolver decision can be overridden by setting the netdns value of the
GODEBUG environment variable (see package runtime) to go or cgo, as in:
解析器的决定可以通过设置netdns的值来重写 GODEBUG环境变量的netdns值（见包的运行时间）来覆盖决议，如：

	export GODEBUG=netdns=go    # force pure Go resolver
	export GODEBUG=netdns=cgo   # force cgo resolver

The decision can also be forced while building the Go source tree
by setting the netgo or netcgo build tag.
在构建 Go 源代码树时，也可以通过设置 netgo 或 netcgo build 标签来强制决定 的时候，也可以通过设置 netgo 或 netcgo build 标签来强制做出决定。

A numeric netdns setting, as in GODEBUG=netdns=1, causes the resolver
to print debugging information about its decisions.
To force a particular resolver while also printing debugging information,
join the two settings by a plus sign, as in GODEBUG=netdns=go+1.
一个数字的netdns设置，如GODEBUG=netdns=1，会导致解析器
打印有关其决定的调试信息。
要强制一个特定的解析器，同时打印调试信息。
用加号连接这两个设置，如 GODEBUG=netdns=go+1。

On Plan 9, the resolver always accesses /net/cs and /net/dns.
在Plan 9 中，解析器总是访问 /net/cs 和 /net/dns。

On Windows, the resolver always uses C library functions, such as GetAddrInfo and DnsQuery.
在 Windows 上，解析器总是使用 C 库函数，例如 GetAddrInfo 和 DnsQuery。

*/
package net

import (
	"context"
	"errors"
	"internal/poll"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
)

// netGo and netCgo contain the state of the build tags used
// to build this binary, and whether cgo is available.
// conf.go mirrors these into conf for easier testing.
// netGo 和 netCgo 包含用于构建此二进制文件的构建标签的状态，以及 cgo 是否可用。conf.go 将这些内容镜像到 conf 中，以方便测试。
var (
	netGo  bool // set true in cgo_stub.go for build tag "netgo" (or no cgo)	// 在cgo_stub.go中为构建标签 "netgo"（或没有cgo）设置true。
	netCgo bool // set true in conf_netcgo.go for build tag "netcgo"			// 在conf_netcgo.go中为构建标签 "netcgo "设置true
)

// Addr represents a network end point address.
// Addr代表一个网络端点地址。
//
// The two methods Network and String conventionally return strings
// that can be passed as the arguments to Dial, but the exact form
// and meaning of the strings is up to the implementation.
// 这两个方法Network和String约定俗成地返回可以作为参数传递给Dial的字符串，但是字符串的具体形式和含义由实现决定。
type Addr interface {
	Network() string // name of the network (for example, "tcp", "udp")								// 网络的名称（例如，"tcp"、"udp"）。
	String() string  // string form of address (for example, "192.0.2.1:25", "[2001:db8::1]:80")	// 地址的字符串形式（例如，"192.0.2.1:25"，"[2001:db8::1]:80"）。
}

// Conn is a generic stream-oriented network connection.
// Conn是一个面向流的通用网络连接。
//
// Multiple goroutines may invoke methods on a Conn simultaneously.
// 多个goroutines可以同时调用Conn上的方法。
type Conn interface {
	// Read reads data from the connection.
	// Read can be made to time out and return an error after a fixed
	// time limit; see SetDeadline and SetReadDeadline.
	// Read从连接中读取数据。读取可以使其超时，并在固定的时间限制后返回一个错误；
	// 参见SetDeadline和SetReadDeadline。
	Read(b []byte) (n int, err error)

	// Write writes data to the connection.
	// Write can be made to time out and return an error after a fixed
	// time limit; see SetDeadline and SetWriteDeadline.
	// Write将数据写入连接。写入可以在固定的时间限制后超时并返回一个错误；
	// 参见SetDeadline和SetWriteDeadline。
	Write(b []byte) (n int, err error)

	// Close closes the connection.
	// Any blocked Read or Write operations will be unblocked and return errors.
	// Close 关闭连接。
	// 任何被阻止的读或写操作都将被解除阻止并返回错误。
	Close() error

	// LocalAddr returns the local network address.
	// LocalAddr 返回本地网络地址。
	LocalAddr() Addr

	// RemoteAddr returns the remote network address.
	// RemoteAddr 返回远程网络地址。
	RemoteAddr() Addr

	// SetDeadline sets the read and write deadlines associated
	// with the connection. It is equivalent to calling both
	// SetReadDeadline and SetWriteDeadline.
	//
	// A deadline is an absolute time after which I/O operations
	// fail instead of blocking. The deadline applies to all future
	// and pending I/O, not just the immediately following call to
	// Read or Write. After a deadline has been exceeded, the
	// connection can be refreshed by setting a deadline in the future.
	//
	// If the deadline is exceeded a call to Read or Write or to other
	// I/O methods will return an error that wraps os.ErrDeadlineExceeded.
	// This can be tested using errors.Is(err, os.ErrDeadlineExceeded).
	// The error's Timeout method will return true, but note that there
	// are other possible errors for which the Timeout method will
	// return true even if the deadline has not been exceeded.
	//
	// An idle timeout can be implemented by repeatedly extending
	// the deadline after successful Read or Write calls.
	//
	// A zero value for t means I/O operations will not time out.
	/* 设置超时时间 */
	SetDeadline(t time.Time) error

	// SetReadDeadline sets the deadline for future Read calls
	// and any currently-blocked Read call.
	// A zero value for t means Read will not time out.
	/* 设置读超时时间 */
	SetReadDeadline(t time.Time) error

	// SetWriteDeadline sets the deadline for future Write calls
	// and any currently-blocked Write call.
	// Even if write times out, it may return n > 0, indicating that
	// some of the data was successfully written.
	// A zero value for t means Write will not time out.
	/* 设置写超时时间 */
	SetWriteDeadline(t time.Time) error
}

type conn struct {
	fd *netFD
}

func (c *conn) ok() bool { return c != nil && c.fd != nil }

// Implementation of the Conn interface.
// Conn接口的实现。

// Read implements the Conn Read method.
// Read实现Conn Read方法。
func (c *conn) Read(b []byte) (int, error) {
	if !c.ok() {
		return 0, syscall.EINVAL
	}
	n, err := c.fd.Read(b)
	if err != nil && err != io.EOF {
		err = &OpError{Op: "read", Net: c.fd.net, Source: c.fd.laddr, Addr: c.fd.raddr, Err: err}
	}
	return n, err
}

// Write implements the Conn Write method.
// Write实现Conn Write方法。
func (c *conn) Write(b []byte) (int, error) {
	if !c.ok() {
		return 0, syscall.EINVAL
	}
	n, err := c.fd.Write(b)
	if err != nil {
		err = &OpError{Op: "write", Net: c.fd.net, Source: c.fd.laddr, Addr: c.fd.raddr, Err: err}
	}
	return n, err
}

// Close closes the connection.
// Close 关闭连接。
func (c *conn) Close() error {
	if !c.ok() {
		return syscall.EINVAL
	}
	err := c.fd.Close()
	if err != nil {
		err = &OpError{Op: "close", Net: c.fd.net, Source: c.fd.laddr, Addr: c.fd.raddr, Err: err}
	}
	return err
}

// LocalAddr returns the local network address.
// The Addr returned is shared by all invocations of LocalAddr, so
// do not modify it.
// LocalAddr 返回本地网络地址。
// 返回的Addr是所有调用LocalAddr的次数所共享的，所以不要修改它。
func (c *conn) LocalAddr() Addr {
	if !c.ok() {
		return nil
	}
	return c.fd.laddr
}

// RemoteAddr returns the remote network address.
// The Addr returned is shared by all invocations of RemoteAddr, so
// do not modify it.
// RemoteAddr 返回远程网络地址。返回的Addr是所有调用RemoteAddr的次数所共享的，所以不要修改它。
func (c *conn) RemoteAddr() Addr {
	if !c.ok() {
		return nil
	}
	return c.fd.raddr
}

// SetDeadline implements the Conn SetDeadline method.
// SetDeadline实现Conn SetDeadline方法。
func (c *conn) SetDeadline(t time.Time) error {
	if !c.ok() {
		return syscall.EINVAL
	}
	if err := c.fd.SetDeadline(t); err != nil {
		return &OpError{Op: "set", Net: c.fd.net, Source: nil, Addr: c.fd.laddr, Err: err}
	}
	return nil
}

// SetReadDeadline implements the Conn SetReadDeadline method.
// SetReadDeadline实现Conn SetReadDeadline方法。
func (c *conn) SetReadDeadline(t time.Time) error {
	if !c.ok() {
		return syscall.EINVAL
	}
	if err := c.fd.SetReadDeadline(t); err != nil {
		return &OpError{Op: "set", Net: c.fd.net, Source: nil, Addr: c.fd.laddr, Err: err}
	}
	return nil
}

// SetWriteDeadline implements the Conn SetWriteDeadline method.
// SetWriteDeadline实现Conn SetWriteDeadline方法。
func (c *conn) SetWriteDeadline(t time.Time) error {
	if !c.ok() {
		return syscall.EINVAL
	}
	if err := c.fd.SetWriteDeadline(t); err != nil {
		return &OpError{Op: "set", Net: c.fd.net, Source: nil, Addr: c.fd.laddr, Err: err}
	}
	return nil
}

// SetReadBuffer sets the size of the operating system's
// receive buffer associated with the connection.
// SetReadBuffer设置操作系统与连接相关的接收缓冲区的大小。
func (c *conn) SetReadBuffer(bytes int) error {
	if !c.ok() {
		return syscall.EINVAL
	}
	if err := setReadBuffer(c.fd, bytes); err != nil {
		return &OpError{Op: "set", Net: c.fd.net, Source: nil, Addr: c.fd.laddr, Err: err}
	}
	return nil
}

// SetWriteBuffer sets the size of the operating system's
// transmit buffer associated with the connection.
// SetWriteBuffer设置操作系统与连接相关的传输缓冲区的大小。
func (c *conn) SetWriteBuffer(bytes int) error {
	if !c.ok() {
		return syscall.EINVAL
	}
	if err := setWriteBuffer(c.fd, bytes); err != nil {
		return &OpError{Op: "set", Net: c.fd.net, Source: nil, Addr: c.fd.laddr, Err: err}
	}
	return nil
}

// File returns a copy of the underlying os.File.
// It is the caller's responsibility to close f when finished.
// Closing c does not affect f, and closing f does not affect c.
// File返回底层os.File的副本。
// 完成后关闭f是调用者的责任。
// 关闭c不会影响f，关闭f也不会影响c。
//
// The returned os.File's file descriptor is different from the connection's.
// Attempting to change properties of the original using this duplicate
// may or may not have the desired effect.
// 返回的os.File的文件描述符与连接的不同。
// 试图使用这个副本来改变原文件的属性，可能会或可能不会达到预期的效果。
func (c *conn) File() (f *os.File, err error) {
	f, err = c.fd.dup()
	if err != nil {
		err = &OpError{Op: "file", Net: c.fd.net, Source: c.fd.laddr, Addr: c.fd.raddr, Err: err}
	}
	return
}

// PacketConn is a generic packet-oriented network connection.
// PacketConn是一个面向数据包的通用网络连接。
//
// Multiple goroutines may invoke methods on a PacketConn simultaneously.
// 多个goroutines可以同时调用PacketConn上的方法。
type PacketConn interface {
	// ReadFrom reads a packet from the connection,
	// copying the payload into p. It returns the number of
	// bytes copied into p and the return address that
	// was on the packet.
	// It returns the number of bytes read (0 <= n <= len(p))
	// and any error encountered. Callers should always process
	// the n > 0 bytes returned before considering the error err.
	// ReadFrom can be made to time out and return an error after a
	// fixed time limit; see SetDeadline and SetReadDeadline.
	ReadFrom(p []byte) (n int, addr Addr, err error)

	// WriteTo writes a packet with payload p to addr.
	// WriteTo can be made to time out and return an Error after a
	// fixed time limit; see SetDeadline and SetWriteDeadline.
	// On packet-oriented connections, write timeouts are rare.
	WriteTo(p []byte, addr Addr) (n int, err error)

	// Close closes the connection.
	// Any blocked ReadFrom or WriteTo operations will be unblocked and return errors.
	Close() error

	// LocalAddr returns the local network address.
	LocalAddr() Addr

	// SetDeadline sets the read and write deadlines associated
	// with the connection. It is equivalent to calling both
	// SetReadDeadline and SetWriteDeadline.
	//
	// A deadline is an absolute time after which I/O operations
	// fail instead of blocking. The deadline applies to all future
	// and pending I/O, not just the immediately following call to
	// Read or Write. After a deadline has been exceeded, the
	// connection can be refreshed by setting a deadline in the future.
	//
	// If the deadline is exceeded a call to Read or Write or to other
	// I/O methods will return an error that wraps os.ErrDeadlineExceeded.
	// This can be tested using errors.Is(err, os.ErrDeadlineExceeded).
	// The error's Timeout method will return true, but note that there
	// are other possible errors for which the Timeout method will
	// return true even if the deadline has not been exceeded.
	//
	// An idle timeout can be implemented by repeatedly extending
	// the deadline after successful ReadFrom or WriteTo calls.
	//
	// A zero value for t means I/O operations will not time out.
	SetDeadline(t time.Time) error

	// SetReadDeadline sets the deadline for future ReadFrom calls
	// and any currently-blocked ReadFrom call.
	// A zero value for t means ReadFrom will not time out.
	SetReadDeadline(t time.Time) error

	// SetWriteDeadline sets the deadline for future WriteTo calls
	// and any currently-blocked WriteTo call.
	// Even if write times out, it may return n > 0, indicating that
	// some of the data was successfully written.
	// A zero value for t means WriteTo will not time out.
	SetWriteDeadline(t time.Time) error
}

var listenerBacklogCache struct {
	sync.Once
	val int
}

// listenerBacklog is a caching wrapper around maxListenerBacklog.
// listenerBacklog是一个围绕maxListenerBacklog的缓存包装器。
func listenerBacklog() int {
	listenerBacklogCache.Do(func() { listenerBacklogCache.val = maxListenerBacklog() })
	return listenerBacklogCache.val
}

// A Listener is a generic network listener for stream-oriented protocols.
// 监听器是面向流协议的通用网络监听器。
//
// Multiple goroutines may invoke methods on a Listener simultaneously.
// 多个goroutines可以同时调用一个Listener的方法。
type Listener interface {
	// Accept waits for and returns the next connection to the listener.
	// Accept 等待并返回下一个连接给监听器。
	Accept() (Conn, error)

	// Close closes the listener.
	// Any blocked Accept operations will be unblocked and return errors.
	// Close 关闭监听器。
	// 任何被阻止的Accept操作都将被解除阻止并返回错误。
	Close() error

	// Addr returns the listener's network address.
	// Addr 返回监听者的网络地址。
	Addr() Addr
}

// An Error represents a network error.
type Error interface {
	error
	Timeout() bool   // Is the error a timeout?
	Temporary() bool // Is the error temporary?
}

// Various errors contained in OpError.
var (
	// For connection setup operations.
	errNoSuitableAddress = errors.New("no suitable address found")

	// For connection setup and write operations.
	errMissingAddress = errors.New("missing address")

	// For both read and write operations.
	errCanceled         = errors.New("operation was canceled")
	ErrWriteToConnected = errors.New("use of WriteTo with pre-connected connection")
)

// mapErr maps from the context errors to the historical internal net
// error values.
//
// TODO(bradfitz): get rid of this after adjusting tests and making
// context.DeadlineExceeded implement net.Error?
func mapErr(err error) error {
	switch err {
	case context.Canceled:
		return errCanceled
	case context.DeadlineExceeded:
		return errTimeout
	default:
		return err
	}
}

// OpError is the error type usually returned by functions in the net
// package. It describes the operation, network type, and address of
// an error.
type OpError struct {
	// Op is the operation which caused the error, such as
	// "read" or "write".
	Op string

	// Net is the network type on which this error occurred,
	// such as "tcp" or "udp6".
	Net string

	// For operations involving a remote network connection, like
	// Dial, Read, or Write, Source is the corresponding local
	// network address.
	Source Addr

	// Addr is the network address for which this error occurred.
	// For local operations, like Listen or SetDeadline, Addr is
	// the address of the local endpoint being manipulated.
	// For operations involving a remote network connection, like
	// Dial, Read, or Write, Addr is the remote address of that
	// connection.
	Addr Addr

	// Err is the error that occurred during the operation.
	// The Error method panics if the error is nil.
	Err error
}

func (e *OpError) Unwrap() error { return e.Err }

func (e *OpError) Error() string {
	if e == nil {
		return "<nil>"
	}
	s := e.Op
	if e.Net != "" {
		s += " " + e.Net
	}
	if e.Source != nil {
		s += " " + e.Source.String()
	}
	if e.Addr != nil {
		if e.Source != nil {
			s += "->"
		} else {
			s += " "
		}
		s += e.Addr.String()
	}
	s += ": " + e.Err.Error()
	return s
}

var (
	// aLongTimeAgo is a non-zero time, far in the past, used for
	// immediate cancellation of dials.
	aLongTimeAgo = time.Unix(1, 0)

	// nonDeadline and noCancel are just zero values for
	// readability with functions taking too many parameters.
	noDeadline = time.Time{}
	noCancel   = (chan struct{})(nil)
)

type timeout interface {
	Timeout() bool
}

func (e *OpError) Timeout() bool {
	if ne, ok := e.Err.(*os.SyscallError); ok {
		t, ok := ne.Err.(timeout)
		return ok && t.Timeout()
	}
	t, ok := e.Err.(timeout)
	return ok && t.Timeout()
}

type temporary interface {
	Temporary() bool
}

func (e *OpError) Temporary() bool {
	// Treat ECONNRESET and ECONNABORTED as temporary errors when
	// they come from calling accept. See issue 6163.
	if e.Op == "accept" && isConnError(e.Err) {
		return true
	}

	if ne, ok := e.Err.(*os.SyscallError); ok {
		t, ok := ne.Err.(temporary)
		return ok && t.Temporary()
	}
	t, ok := e.Err.(temporary)
	return ok && t.Temporary()
}

// A ParseError is the error type of literal network address parsers.
type ParseError struct {
	// Type is the type of string that was expected, such as
	// "IP address", "CIDR address".
	Type string

	// Text is the malformed text string.
	Text string
}

func (e *ParseError) Error() string { return "invalid " + e.Type + ": " + e.Text }

func (e *ParseError) Timeout() bool   { return false }
func (e *ParseError) Temporary() bool { return false }

type AddrError struct {
	Err  string
	Addr string
}

func (e *AddrError) Error() string {
	if e == nil {
		return "<nil>"
	}
	s := e.Err
	if e.Addr != "" {
		s = "address " + e.Addr + ": " + s
	}
	return s
}

func (e *AddrError) Timeout() bool   { return false }
func (e *AddrError) Temporary() bool { return false }

type UnknownNetworkError string

func (e UnknownNetworkError) Error() string   { return "unknown network " + string(e) }
func (e UnknownNetworkError) Timeout() bool   { return false }
func (e UnknownNetworkError) Temporary() bool { return false }

type InvalidAddrError string

func (e InvalidAddrError) Error() string   { return string(e) }
func (e InvalidAddrError) Timeout() bool   { return false }
func (e InvalidAddrError) Temporary() bool { return false }

// errTimeout exists to return the historical "i/o timeout" string
// for context.DeadlineExceeded. See mapErr.
// It is also used when Dialer.Deadline is exceeded.
//
// TODO(iant): We could consider changing this to os.ErrDeadlineExceeded
// in the future, but note that that would conflict with the TODO
// at mapErr that suggests changing it to context.DeadlineExceeded.
var errTimeout error = &timeoutError{}

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "i/o timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

// DNSConfigError represents an error reading the machine's DNS configuration.
// (No longer used; kept for compatibility.)
type DNSConfigError struct {
	Err error
}

func (e *DNSConfigError) Unwrap() error   { return e.Err }
func (e *DNSConfigError) Error() string   { return "error reading DNS config: " + e.Err.Error() }
func (e *DNSConfigError) Timeout() bool   { return false }
func (e *DNSConfigError) Temporary() bool { return false }

// Various errors contained in DNSError.
var (
	errNoSuchHost = errors.New("no such host")
)

// DNSError represents a DNS lookup error.
type DNSError struct {
	Err         string // description of the error
	Name        string // name looked for
	Server      string // server used
	IsTimeout   bool   // if true, timed out; not all timeouts set this
	IsTemporary bool   // if true, error is temporary; not all errors set this
	IsNotFound  bool   // if true, host could not be found
}

func (e *DNSError) Error() string {
	if e == nil {
		return "<nil>"
	}
	s := "lookup " + e.Name
	if e.Server != "" {
		s += " on " + e.Server
	}
	s += ": " + e.Err
	return s
}

// Timeout reports whether the DNS lookup is known to have timed out.
// This is not always known; a DNS lookup may fail due to a timeout
// and return a DNSError for which Timeout returns false.
func (e *DNSError) Timeout() bool { return e.IsTimeout }

// Temporary reports whether the DNS error is known to be temporary.
// This is not always known; a DNS lookup may fail due to a temporary
// error and return a DNSError for which Temporary returns false.
func (e *DNSError) Temporary() bool { return e.IsTimeout || e.IsTemporary }

// errClosed exists just so that the docs for ErrClosed don't mention
// the internal package poll.
var errClosed = poll.ErrNetClosing

// ErrClosed is the error returned by an I/O call on a network
// connection that has already been closed, or that is closed by
// another goroutine before the I/O is completed. This may be wrapped
// in another error, and should normally be tested using
// errors.Is(err, net.ErrClosed).
var ErrClosed error = errClosed

type writerOnly struct {
	io.Writer
}

// Fallback implementation of io.ReaderFrom's ReadFrom, when sendfile isn't
// applicable.
func genericReadFrom(w io.Writer, r io.Reader) (n int64, err error) {
	// Use wrapper to hide existing r.ReadFrom from io.Copy.
	return io.Copy(writerOnly{w}, r)
}

// Limit the number of concurrent cgo-using goroutines, because
// each will block an entire operating system thread. The usual culprit
// is resolving many DNS names in separate goroutines but the DNS
// server is not responding. Then the many lookups each use a different
// thread, and the system or the program runs out of threads.

var threadLimit chan struct{}

var threadOnce sync.Once

// 获取线程
/*
	通过acquireThread、releaseThread来限制最大线程数
	acquireThread在第一次调用的时候，生成一个指定大小的有缓冲通道来限制最大线程数，然后能够使用的线程数减1
	releaseThread释放线程，也就是在使用完之后，回收限制数
*/
func acquireThread() {
	threadOnce.Do(func() {
		threadLimit = make(chan struct{}, concurrentThreadsLimit())
	})
	threadLimit <- struct{}{}
}

func releaseThread() {
	<-threadLimit
}

// buffersWriter is the interface implemented by Conns that support a
// "writev"-like batch write optimization.
// writeBuffers should fully consume and write all chunks from the
// provided Buffers, else it should report a non-nil error.
type buffersWriter interface {
	writeBuffers(*Buffers) (int64, error)
}

// Buffers contains zero or more runs of bytes to write.
//
// On certain machines, for certain types of connections, this is
// optimized into an OS-specific batch write operation (such as
// "writev").
type Buffers [][]byte

var (
	_ io.WriterTo = (*Buffers)(nil)
	_ io.Reader   = (*Buffers)(nil)
)

func (v *Buffers) WriteTo(w io.Writer) (n int64, err error) {
	if wv, ok := w.(buffersWriter); ok {
		return wv.writeBuffers(v)
	}
	for _, b := range *v {
		nb, err := w.Write(b)
		n += int64(nb)
		if err != nil {
			v.consume(n)
			return n, err
		}
	}
	v.consume(n)
	return n, nil
}

func (v *Buffers) Read(p []byte) (n int, err error) {
	for len(p) > 0 && len(*v) > 0 {
		n0 := copy(p, (*v)[0])
		v.consume(int64(n0))
		p = p[n0:]
		n += n0
	}
	if len(*v) == 0 {
		err = io.EOF
	}
	return
}

func (v *Buffers) consume(n int64) {
	for len(*v) > 0 {
		ln0 := int64(len((*v)[0]))
		if ln0 > n {
			(*v)[0] = (*v)[0][n:]
			return
		}
		n -= ln0
		(*v)[0] = nil
		*v = (*v)[1:]
	}
}
