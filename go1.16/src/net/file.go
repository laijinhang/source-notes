// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package net

import "os"

// BUG(mikio): On JS and Windows, the FileConn, FileListener and
// FilePacketConn functions are not implemented.

// BUG（mikio）：在JS和Windows上，FileConn、FileListener和FilePacketConn函数没有实现。
type fileAddr string

func (fileAddr) Network() string  { return "file+net" }
func (f fileAddr) String() string { return string(f) }

// FileConn returns a copy of the network connection corresponding to
// the open file f.
// It is the caller's responsibility to close f when finished.
// Closing c does not affect f, and closing f does not affect c.

// FileConn返回与打开的文件f对应的网络连接的副本。
// 调用程序有责任在完成后关闭f。
// 关闭c不会影响f，关闭f也不会影响c。
func FileConn(f *os.File) (c Conn, err error) {
	c, err = fileConn(f)
	if err != nil {
		err = &OpError{Op: "file", Net: "file+net", Source: nil, Addr: fileAddr(f.Name()), Err: err}
	}
	return
}

// FileListener returns a copy of the network listener corresponding
// to the open file f.
// It is the caller's responsibility to close ln when finished.
// Closing ln does not affect f, and closing f does not affect ln.
// FileListener返回与打开的文件f对应的网络侦听器的副本。
// 调用方负责在完成时关闭ln。
// 关闭ln不会影响f，关闭f也不会影响ln。
func FileListener(f *os.File) (ln Listener, err error) {
	ln, err = fileListener(f)
	if err != nil {
		err = &OpError{Op: "file", Net: "file+net", Source: nil, Addr: fileAddr(f.Name()), Err: err}
	}
	return
}

// FilePacketConn returns a copy of the packet network connection
// corresponding to the open file f.
// It is the caller's responsibility to close f when finished.
// Closing c does not affect f, and closing f does not affect c.
// FilePacketConn返回与打开的文件f对应的分组网络连接的副本。
// 完成后由调用方负责关闭f。关闭c不会影响f，关闭f也不会影响c。
func FilePacketConn(f *os.File) (c PacketConn, err error) {
	c, err = filePacketConn(f)
	if err != nil {
		err = &OpError{Op: "file", Net: "file+net", Source: nil, Addr: fileAddr(f.Name()), Err: err}
	}
	return
}
