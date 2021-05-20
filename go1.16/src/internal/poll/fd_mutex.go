// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package poll

import "sync/atomic"

// fdMutex是专门的同步原语，
// 用于管理fd的生存期并序列化对FD上的Read，
// Write和Close方法的访问。
type fdMutex struct {
	state uint64
	rsema uint32
	wsema uint32
}

// fdMutex.state is organized as follows:
// 1 bit - whether FD is closed, if set all subsequent lock operations will fail.
// 1 bit - lock for read operations.
// 1 bit - lock for write operations.
// 20 bits - total number of references (read+write+misc).
// 20 bits - number of outstanding read waiters.
// 20 bits - number of outstanding write waiters.
// fdMutex.state的组成如下：
// 1 bit - 是否关闭FD，如果置位，则所有后续锁定操作将失败。
// 1 bit - 锁定读取操作。
// 1 bit - 锁定写入操作。
// 20 bits - 引用总数（读+写+杂项）。
// 20 bits - read waiter的数量
// 20 bits - write waiter的数量
const (
	mutexClosed  = 1 << 0 // 是否关闭FD，如果置位，则所有后续锁定操作将失败
	mutexRLock   = 1 << 1 // 锁定读取操作
	mutexWLock   = 1 << 2 // 锁定写入操作
	mutexRef     = 1 << 3 //  引用总数（读+写+杂项）。
	mutexRefMask = (1<<20 - 1) << 3
	mutexRWait   = 1 << 23
	mutexRMask   = (1<<20 - 1) << 23
	mutexWWait   = 1 << 43
	mutexWMask   = (1<<20 - 1) << 43
)

// 在一个文件或套接字上有太多的并发操作（最大1048575）。
const overflowMsg = "too many concurrent operations on a single file or socket (max 1048575)"

// Read operations must do rwlock(true)/rwunlock(true).
// 读取操作必须做rwlock(true)/rwunlock(true)。
//
// Write operations must do rwlock(false)/rwunlock(false).
// 写操作必须做rwlock(false)/rwunlock(false)。
//
// Misc operations must do incref/decref.
// Misc operations include functions like setsockopt and setDeadline.
// They need to use incref/decref to ensure that they operate on the
// correct fd in presence of a concurrent close call (otherwise fd can
// be closed under their feet).
// 杂项操作必须做增量/减量。
// 杂项操作包括 setsockopt 和 setDeadline 等函数。
// 它们需要使用incref/decref来确保在出现并发的关闭调用时对正确的fd进行操作（否则fd会在它们脚下被关闭）。
//
// Close operations must do increfAndClose/decref.
// 关闭操作必须做increfAndClose/decref。

// incref adds a reference to mu.
// It reports whether mu is available for reading or writing.
// incref添加一个对mu的引用。
// 它报告mu是否可用于读或写。
/*
	1、获取锁状态
	2、判断锁是否已关闭，如果已关闭，则返回false，没有关闭，则继续
	3、锁的引用总数加一
	4、如果太多引用（文件或套接字上有太多并发，超过了最大1048575），也就是越界了
	5、使用cas尝试获取添加对锁的引用，如果成功，则返回true，否则进入执行第1步
*/
func (mu *fdMutex) incref() bool {
	for {
		old := atomic.LoadUint64(&mu.state)
		if old&mutexClosed != 0 {
			return false
		}
		new := old + mutexRef
		if new&mutexRefMask == 0 {
			panic(overflowMsg)
		}
		if atomic.CompareAndSwapUint64(&mu.state, old, new) {
			return true
		}
	}
}

// increfAndClose sets the state of mu to closed.
// It returns false if the file was already closed.
// increfAndClose将mu的状态设为关闭。
// 如果文件已经被关闭，它将返回false。
/*
	1、获取锁状态，放入临时锁变量old
	2、判断是否关闭，如果已关闭则返回false
	3、old上标记关闭，并将引用数加1
	4、如果太多引用（文件或套接字上有太多并发，超过了最大1048575），也就是越界了
	5、删除所有读和写的等待者
	6、使用cas尝试设置新锁状态，如果失败，则继续执行第1步，如果成功
	7、唤醒所有读和写的等待者，他们将在唤醒后观察关闭的标志（也就是通知这个锁的所有等待者，表示这个锁要关闭了，去做相应的处理吧）
	8、返回true
 */
func (mu *fdMutex) increfAndClose() bool {
	for {
		old := atomic.LoadUint64(&mu.state)
		if old&mutexClosed != 0 {
			return false
		}
		// Mark as closed and acquire a reference.
		// 标记为关闭，并获得一个引用。
		new := (old | mutexClosed) + mutexRef
		if new&mutexRefMask == 0 {
			panic(overflowMsg)
		}
		// Remove all read and write waiters.
		// 删除所有读和写的等待者。
		new &^= mutexRMask | mutexWMask
		if atomic.CompareAndSwapUint64(&mu.state, old, new) {
			// Wake all read and write waiters,
			// they will observe closed flag after wakeup.
			// 唤醒所有读和写的等待者。
			// 他们将在唤醒后观察关闭的标志。
			for old&mutexRMask != 0 {
				old -= mutexRWait
				runtime_Semrelease(&mu.rsema)
			}
			for old&mutexWMask != 0 {
				old -= mutexWWait
				runtime_Semrelease(&mu.wsema)
			}
			return true
		}
	}
}

// decref removes a reference from mu.
// It reports whether there is no remaining reference.
// decref 从 mu 中删除一个引用。
// 它返回是否有剩余的引用。
func (mu *fdMutex) decref() bool {
	for {
		old := atomic.LoadUint64(&mu.state)
		if old&mutexRefMask == 0 {
			panic("inconsistent poll.fdMutex")
		}
		new := old - mutexRef
		if atomic.CompareAndSwapUint64(&mu.state, old, new) {
			return new&(mutexClosed|mutexRefMask) == mutexClosed
		}
	}
}

// lock adds a reference to mu and locks mu.
// It reports whether mu is available for reading or writing.
// lock添加对mu的引用并锁定mu。
// 报告mu是否可用于读取或写入。
func (mu *fdMutex) rwlock(read bool) bool {
	var mutexBit, mutexWait, mutexMask uint64
	var mutexSema *uint32
	if read { // 获取读锁操作
		mutexBit = mutexRLock // 0b10
		mutexWait = mutexRWait
		mutexMask = mutexRMask
		mutexSema = &mu.rsema
	} else { // 获取写锁操作
		mutexBit = mutexWLock  // 加写锁，0b100
		mutexWait = mutexWWait // 设置写等待
		mutexMask = mutexWMask
		mutexSema = &mu.wsema
	}
	for {
		old := atomic.LoadUint64(&mu.state)
		// 如果已经关闭，则读写都会失败
		if old&mutexClosed != 0 {
			return false
		}
		var new uint64
		// 如果没有关闭
		if old&mutexBit == 0 {
			// Lock is free, acquire it.
			// 锁定是免费的，获得它。
			new = (old | mutexBit) + mutexRef
			if new&mutexRefMask == 0 {
				panic(overflowMsg)
			}
		} else {
			// Wait for lock.
			new = old + mutexWait
			if new&mutexMask == 0 {
				panic(overflowMsg)
			}
		}
		if atomic.CompareAndSwapUint64(&mu.state, old, new) {
			if old&mutexBit == 0 {
				return true
			}
			runtime_Semacquire(mutexSema)
			// The signaller has subtracted mutexWait.
		}
	}
}

// unlock removes a reference from mu and unlocks mu.
// It reports whether there is no remaining reference.
// unlock从mu中删除一个引用并解锁mu。
// 它报告是否没有剩余的引用。
func (mu *fdMutex) rwunlock(read bool) bool {
	var mutexBit, mutexWait, mutexMask uint64
	var mutexSema *uint32
	if read {
		mutexBit = mutexRLock
		mutexWait = mutexRWait
		mutexMask = mutexRMask
		mutexSema = &mu.rsema
	} else {
		mutexBit = mutexWLock
		mutexWait = mutexWWait
		mutexMask = mutexWMask
		mutexSema = &mu.wsema
	}
	for {
		old := atomic.LoadUint64(&mu.state)
		if old&mutexBit == 0 || old&mutexRefMask == 0 {
			panic("inconsistent poll.fdMutex")
		}
		// Drop lock, drop reference and wake read waiter if present.
		// 丢弃锁，丢弃引用，如果存在的话，唤醒读取服务器。
		new := (old &^ mutexBit) - mutexRef
		if old&mutexMask != 0 {
			new -= mutexWait
		}
		if atomic.CompareAndSwapUint64(&mu.state, old, new) {
			if old&mutexMask != 0 {
				runtime_Semrelease(mutexSema)
			}
			return new&(mutexClosed|mutexRefMask) == mutexClosed
		}
	}
}

// Implemented in runtime package.
// 在运行时包中实现。
func runtime_Semacquire(sema *uint32)
func runtime_Semrelease(sema *uint32)

// incref adds a reference to fd.
// It returns an error when fd cannot be used.
// incref添加对fd的引用。
// 当fd不能使用时，它会返回一个错误。
func (fd *FD) incref() error {
	if !fd.fdmu.incref() {
		return errClosing(fd.isFile)
	}
	return nil
}

// decref removes a reference from fd.
// It also closes fd when the state of fd is set to closed and there
// is no remaining reference.
// decref从fd中删除一个引用。
// 当fd的状态被设置为closed且没有剩余的引用时，它也会关闭fd。
func (fd *FD) decref() error {
	if fd.fdmu.decref() {
		return fd.destroy()
	}
	return nil
}

// readLock adds a reference to fd and locks fd for reading.
// It returns an error when fd cannot be used for reading.
// readLock添加一个对fd的引用，并锁定fd以便读取。
// 当fd不能用于读取时，它会返回一个错误。
func (fd *FD) readLock() error {
	if !fd.fdmu.rwlock(true) {
		return errClosing(fd.isFile)
	}
	return nil
}

// readUnlock removes a reference from fd and unlocks fd for reading.
// It also closes fd when the state of fd is set to closed and there
// is no remaining reference.
// readUnlock从fd中删除一个引用，并解锁fd以供读取。
// 当fd的状态被设置为closed并且没有剩余的引用时，它也会关闭fd。
func (fd *FD) readUnlock() {
	if fd.fdmu.rwunlock(true) {
		fd.destroy()
	}
}

// writeLock adds a reference to fd and locks fd for writing.
// It returns an error when fd cannot be used for writing.
// writeLock添加一个对fd的引用，并锁定fd用于写入。当fd不能用于写入时，它会返回一个错误。
func (fd *FD) writeLock() error {
	if !fd.fdmu.rwlock(false) {
		return errClosing(fd.isFile)
	}
	return nil
}

// writeUnlock removes a reference from fd and unlocks fd for writing.
// It also closes fd when the state of fd is set to closed and there
// is no remaining reference.
// writeUnlock从fd中删除一个引用，并解锁fd以便写入。
// 当fd的状态被设置为closed并且没有剩余的引用时，它也会关闭fd。
func (fd *FD) writeUnlock() {
	if fd.fdmu.rwunlock(false) {
		fd.destroy()
	}
}
