// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build aix || darwin || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris || windows
// +build aix darwin dragonfly freebsd js,wasm linux netbsd openbsd solaris windows

package runtime

import (
	"runtime/internal/atomic"
	"unsafe"
)

// Integrated network poller (platform-independent part).
// A particular implementation (epoll/kqueue/port/AIX/Windows)
// must define the following functions:
//
// func netpollinit()
//     Initialize the poller. Only called once.
//	   初始化poller，仅调用一次
//
// func netpollopen(fd uintptr, pd *pollDesc) int32
//     Arm edge-triggered notifications for fd. The pd argument is to pass
//     back to netpollready when fd is ready. Return an errno value.
//	   监听文件描述符上的边缘触发事件，创建事件并加入监听
//
// func netpollclose(fd uintptr) int32
//     Disable notifications for fd. Return an errno value.
//
// func netpoll(delta int64) gList
//     Poll the network. If delta < 0, block indefinitely. If delta == 0,
//     poll without blocking. If delta > 0, block for up to delta nanoseconds.
//     Return a list of goroutines built by calling netpollready.
//     轮询网络并返回一组已经准备就绪的 Goroutine，传入的参数会决定它的行为：
//	   	1.如果参数小于0，无限等待文件描述符就绪；
//		2.如果参数等于0，非阻塞地轮询网络；
//		3.如果参数大于0，阻塞特定时间轮询网络；
//
// func netpollBreak()
//     Wake up the network poller, assumed to be blocked in netpoll.
//	   唤醒网络轮询器，例如：计时器向前修改时间时会通过该函数中断网络轮询器。
//
// func netpollIsPollDescriptor(fd uintptr) bool
//     Reports whether fd is a file descriptor used by the poller.
//	   判断文件描述符是否被轮询器使用。

// Error codes returned by runtime_pollReset and runtime_pollWait.
// These must match the values in internal/poll/fd_poll_runtime.go.
// untime_pollReset 和 runtime_pollWait 返回的错误码
// 这些必须与internal/poll/fd_poll_runtime.go中的值匹配。
const (
	pollNoError        = 0 // no error							// 没有错误
	pollErrClosing     = 1 // descriptor is closed				// 没有错误
	pollErrTimeout     = 2 // I/O timeout						// I/O超时
	pollErrNotPollable = 3 // general error polling descriptor	// 通用错误轮询描述符
)

// pollDesc contains 2 binary semaphores, rg and wg, to park reader and writer
// goroutines respectively. The semaphore can be in the following states:
// pollDesc包含2个二进制信号，rg和wg，分别用于停放读者和写者的程序。这些信号灯可以处于以下状态：
// pdReady - io readiness notification is pending;
// pdReady - io准备就绪通知正在等待；
//           a goroutine consumes the notification by changing the state to nil.
// 			一个goroutine通过改变状态为nil来消耗该通知。
// pdWait - a goroutine prepares to park on the semaphore, but not yet parked;
// pdWait - 一个goroutine准备在semaphore上，但还没有。
//          the goroutine commits to park by changing the state to G pointer,
//          or, alternatively, concurrent io notification changes the state to pdReady,
//          or, alternatively, concurrent timeout/close changes the state to nil.
// 			协作程序通过改变状态为G指针来承诺停放，或者，同时进行的io通知将状态改为pdReady，或者，同时进行的超时/关闭将状态改为nil。
// G pointer - the goroutine is blocked on the semaphore;
// G指针--goroutine在semaphore上被阻塞了；
//             io notification or timeout/close changes the state to pdReady or nil respectively
//             and unparks the goroutine.
// 			   io通知或超时/关闭分别将状态变为pdReady或nil，并解除对goroutine的阻塞。
// nil - none of the above.
// nil - 上述情况都不存在。
const (
	pdReady uintptr = 1 // io准备就绪通知正在等待
	pdWait  uintptr = 2 // 等待
)

const pollBlockSize = 4 * 1024

// 操作系统中的I/O多路复用函数会监控文件描述符的可读或者可写,而Go语言网络轮询器会监听runtime.pollDesc结构体的状态，
// 它会封装操作系统的文件描述符
// Network poller descriptor.
// 网络poller描述符。
//
// No heap pointers.
// 没有堆指针。
//
//go:notinheap
type pollDesc struct {
	link *pollDesc // in pollcache, protected by pollcache.lock	// 在pollcache中，受pollcache.lock保护

	// The lock protects pollOpen, pollSetDeadline, pollUnblock and deadlineimpl operations.
	// This fully covers seq, rt and wt variables. fd is constant throughout the PollDesc lifetime.
	// pollReset, pollWait, pollWaitCanceled and runtime·netpollready (IO readiness notification)
	// proceed w/o taking the lock. So closing, everr, rg, rd, wg and wd are manipulated
	// in a lock-free way by all operations.
	// NOTE(dvyukov): the following code uses uintptr to store *g (rg/wg),
	// that will blow up when GC starts moving objects.

	// 该锁可保护pollOpen，pollSetDeadline，pollUnblock和durationimpl操作。
	// 这完全涵盖了seq，rt和wt变量。 在PollDesc的整个生命周期中，fd都是恒定的。
	// pollReset，pollWait，pollWaitCanceled和运行时·netpollready（IO准备就绪通知）
	//  不带锁继续操作。 因此，关闭，everr，rg，rd，wg和wd所有操作均以无锁方式进行操作。
	// 注意（dvyukov）：以下代码使用uintptr存储* g（rg / wg），
	// 当GC开始移动对象时，that will blow up???
	lock    mutex // protects the following fields				// 保护以下字段
	fd      uintptr
	closing bool

	everr bool      // marks event scanning error happened			// 标志着事件扫描错误的发生
	user  uint32    // user settable cookie							// 用户可设置的cookie
	rseq  uintptr   // protects from stale read timers				// 防止陈旧的读取计时器
	rg    uintptr   // pdReady, pdWait, G waiting for read or nil		// pdReady, pdWait, G等待读取或nil
	rt    timer     // read deadline timer (set if rt.f != nil)		// 读取最后期限的计时器（如果rt.f != nil，则设置）
	rd    int64     // read deadline									// 读取截止日期
	wseq  uintptr   // protects from stale write timers				// 防止写入计时器过期
	wg    uintptr   // pdReady, pdWait, G waiting for write or nil	// pdReady, pdWait, G等待写入或nil
	wt    timer     // write deadline timer							// 写入最后期限的定时器
	wd    int64     // write deadline									// 写下最后期限
	self  *pollDesc // storage for indirect interface. See (*pollDesc).makeArg.	// 为间接接口的存储。见（*pollDesc）.makeArg。
}

type pollCache struct {
	lock  mutex
	first *pollDesc
	// PollDesc objects must be type-stable,
	// because we can get ready notification from epoll/kqueue
	// after the descriptor is closed/reused.
	// Stale notifications are detected using seq variable,
	// seq is incremented when deadlines are changed or descriptor is reused.
	// PollDesc对象必须是类型稳定的，因为我们可以在描述符关闭/重复使用后从epoll/kqueue获得就绪的通知。
	// 陈旧的通知是通过seq变量检测的，当最后期限改变或描述符被重新使用时，seq会被增加。
}

var (
	netpollInitLock mutex
	// 如果netpollInited值为0，则表示netpoll没有进行初始化
	netpollInited uint32

	/*
		管理了一个pollDesc池，以链表的方式
	*/
	pollcache      pollCache
	netpollWaiters uint32
)

//go:linkname poll_runtime_pollServerInit internal/poll.runtime_pollServerInit
func poll_runtime_pollServerInit() {
	netpollGenericInit()
}

/*
保证netpoll只被初始化一次
*/
func netpollGenericInit() {
	// 1、如果已经初始化，则直接结束
	if atomic.Load(&netpollInited) == 0 {
		lockInit(&netpollInitLock, lockRankNetpollInit)
		// 2、上锁，在初始化完成之前，可能有多个goroutine进到这里进行初始化
		lock(&netpollInitLock)
		// 3、初始化完成前，可能有goroutine进到这里，所以需要再判断一下最新的初始化状态
		if netpollInited == 0 {
			// 4、初始化netpoll
			netpollinit()
			// 5、设置初始化状态为已初始化
			atomic.Store(&netpollInited, 1)
		}
		// 6、解锁
		unlock(&netpollInitLock)
	}
}

func netpollinited() bool {
	// atomic.Load(&netpollInited)等于0，表示没有初始化过
	return atomic.Load(&netpollInited) != 0
}

//go:linkname poll_runtime_isPollServerDescriptor internal/poll.runtime_isPollServerDescriptor

// poll_runtime_isPollServerDescriptor reports whether fd is a
// descriptor being used by netpoll.
func poll_runtime_isPollServerDescriptor(fd uintptr) bool {
	return netpollIsPollDescriptor(fd)
}

//go:linkname poll_runtime_pollOpen internal/poll.runtime_pollOpen
func poll_runtime_pollOpen(fd uintptr) (*pollDesc, int) {
	// 1、从pollcache里拿出第一个pollDesc，如果pollcache里面第一个是空的，则为其分配一个，然后返回第一个，pollcache指向第二个
	pd := pollcache.alloc()
	// 2、上锁
	lock(&pd.lock)

	// 3、正在写
	if pd.wg != 0 && pd.wg != pdReady {
		throw("runtime: blocked write on free polldesc") // 运行时：在空闲的Polldesc上写东西受阻
	}
	// 4、正在读
	if pd.rg != 0 && pd.rg != pdReady {
		throw("runtime: blocked read on free polldesc") // 运行时：阻断了对free polldesc的读取
	}
	// 5、初始化pd
	pd.fd = fd
	pd.closing = false
	pd.everr = false
	pd.rseq++
	pd.rg = 0
	pd.rd = 0
	pd.wseq++
	pd.wg = 0
	pd.wd = 0
	pd.self = pd
	// 6、解锁
	unlock(&pd.lock)
	// 7、事件注册函数，将监听套接字描述符加入监听事件
	errno := netpollopen(fd, pd)
	// 8、如果注册事件失败，则将其放回到pollcache，并返回错误信息
	if errno != 0 {
		pollcache.free(pd)
		return nil, int(errno)
	}
	return pd, 0
}

//go:linkname poll_runtime_pollClose internal/poll.runtime_pollClose
func poll_runtime_pollClose(pd *pollDesc) {
	if !pd.closing {
		throw("runtime: close polldesc w/o unblock")
	}
	if pd.wg != 0 && pd.wg != pdReady {
		throw("runtime: blocked write on closing polldesc")
	}
	if pd.rg != 0 && pd.rg != pdReady {
		throw("runtime: blocked read on closing polldesc")
	}
	netpollclose(pd.fd)
	pollcache.free(pd)
}

func (c *pollCache) free(pd *pollDesc) {
	lock(&c.lock)
	pd.link = c.first
	c.first = pd
	unlock(&c.lock)
}

// poll_runtime_pollReset, which is internal/poll.runtime_pollReset,
// prepares a descriptor for polling in mode, which is 'r' or 'w'.
// This returns an error code; the codes are defined above.
/*
poll_runtime_pollReset，即内部/poll.runtime_pollReset，为模式下的轮询准备一个描述符，
这个描述符是'r'或'w'。这将返回一个错误代码，代码的定义在上面。
*/
//go:linkname poll_runtime_pollReset internal/poll.runtime_pollReset
func poll_runtime_pollReset(pd *pollDesc, mode int) int {
	errcode := netpollcheckerr(pd, int32(mode))
	if errcode != pollNoError {
		return errcode
	}
	if mode == 'r' {
		pd.rg = 0
	} else if mode == 'w' {
		pd.wg = 0
	}
	return pollNoError
}

// poll_runtime_pollWait, which is internal/poll.runtime_pollWait,
// waits for a descriptor to be ready for reading or writing,
// according to mode, which is 'r' or 'w'.
// This returns an error code; the codes are defined above.
// poll_runtime_pollWait，即内部/poll.runtime_pollWait，根据模式，即'r'或'w'，
// 等待描述符准备好进行读取或写入。这将返回一个错误代码；这些代码在上面有定义。
//go:linkname poll_runtime_pollWait internal/poll.runtime_pollWait
func poll_runtime_pollWait(pd *pollDesc, mode int) int {
	errcode := netpollcheckerr(pd, int32(mode))
	if errcode != pollNoError {
		return errcode
	}
	// As for now only Solaris, illumos, and AIX use level-triggered IO.
	if GOOS == "solaris" || GOOS == "illumos" || GOOS == "aix" {
		netpollarm(pd, mode)
	}
	for !netpollblock(pd, int32(mode), false) {
		errcode = netpollcheckerr(pd, int32(mode))
		if errcode != pollNoError {
			return errcode
		}
		// Can happen if timeout has fired and unblocked us,
		// but before we had a chance to run, timeout has been reset.
		// Pretend it has not happened and retry.
		// 如果超时已经发生，并解除了对我们的封锁，但在我们有机会运行之前，
		// 超时已经被重置，就会发生这种情况。 假设它没有发生并重试。
	}
	return pollNoError
}

//go:linkname poll_runtime_pollWaitCanceled internal/poll.runtime_pollWaitCanceled
func poll_runtime_pollWaitCanceled(pd *pollDesc, mode int) {
	// This function is used only on windows after a failed attempt to cancel
	// a pending async IO operation. Wait for ioready, ignore closing or timeouts.
	// 这个函数只在试图取消一个待定的异步IO操作失败后的窗口上使用。等待ioready，忽略关闭或超时。
	for !netpollblock(pd, int32(mode), true) {
	}
}

//go:linkname poll_runtime_pollSetDeadline internal/poll.runtime_pollSetDeadline
func poll_runtime_pollSetDeadline(pd *pollDesc, d int64, mode int) {
	lock(&pd.lock)
	// 如果已经关闭，则直接解锁，返回
	if pd.closing {
		unlock(&pd.lock)
		return
	}
	rd0, wd0 := pd.rd, pd.wd
	combo0 := rd0 > 0 && rd0 == wd0
	if d > 0 {
		d += nanotime()
		if d <= 0 {
			// If the user has a deadline in the future, but the delay calculation
			// overflows, then set the deadline to the maximum possible value.
			// 如果用户在未来有一个截止日期，但延迟计算溢出，那么将截止日期设置为可能的最大值。
			d = 1<<63 - 1
		}
	}
	if mode == 'r' || mode == 'r'+'w' {
		pd.rd = d
	}
	if mode == 'w' || mode == 'r'+'w' {
		pd.wd = d
	}
	combo := pd.rd > 0 && pd.rd == pd.wd
	rtf := netpollReadDeadline
	if combo {
		rtf = netpollDeadline
	}
	if pd.rt.f == nil {
		if pd.rd > 0 {
			pd.rt.f = rtf
			// Copy current seq into the timer arg.
			// Timer func will check the seq against current descriptor seq,
			// if they differ the descriptor was reused or timers were reset.
			// 将当前的seq复制到定时器参数中。定时器函数将对当前描述符的序列进行检查，
			// 如果它们不一致，说明描述符被重新使用或定时器被重置。
			pd.rt.arg = pd.makeArg()
			pd.rt.seq = pd.rseq
			resettimer(&pd.rt, pd.rd)
		}
	} else if pd.rd != rd0 || combo != combo0 {
		pd.rseq++ // invalidate current timers
		if pd.rd > 0 {
			modtimer(&pd.rt, pd.rd, 0, rtf, pd.makeArg(), pd.rseq)
		} else {
			deltimer(&pd.rt)
			pd.rt.f = nil
		}
	}
	if pd.wt.f == nil {
		if pd.wd > 0 && !combo {
			pd.wt.f = netpollWriteDeadline
			pd.wt.arg = pd.makeArg()
			pd.wt.seq = pd.wseq
			resettimer(&pd.wt, pd.wd)
		}
	} else if pd.wd != wd0 || combo != combo0 {
		pd.wseq++ // invalidate current timers
		if pd.wd > 0 && !combo {
			modtimer(&pd.wt, pd.wd, 0, netpollWriteDeadline, pd.makeArg(), pd.wseq)
		} else {
			deltimer(&pd.wt)
			pd.wt.f = nil
		}
	}
	// If we set the new deadline in the past, unblock currently pending IO if any.
	// 如果我们在过去设置了新的截止日期，如果有的话，就解除当前待处理的IO。
	var rg, wg *g
	if pd.rd < 0 || pd.wd < 0 {
		atomic.StorepNoWB(noescape(unsafe.Pointer(&wg)), nil) // full memory barrier between stores to rd/wd and load of rg/wg in netpollunblock
		if pd.rd < 0 {
			rg = netpollunblock(pd, 'r', false)
		}
		if pd.wd < 0 {
			wg = netpollunblock(pd, 'w', false)
		}
	}
	unlock(&pd.lock)
	if rg != nil {
		netpollgoready(rg, 3)
	}
	if wg != nil {
		netpollgoready(wg, 3)
	}
}

//go:linkname poll_runtime_pollUnblock internal/poll.runtime_pollUnblock
func poll_runtime_pollUnblock(pd *pollDesc) {
	lock(&pd.lock)
	if pd.closing {
		throw("runtime: unblock on closing polldesc")
	}
	pd.closing = true
	pd.rseq++
	pd.wseq++
	var rg, wg *g
	atomic.StorepNoWB(noescape(unsafe.Pointer(&rg)), nil) // full memory barrier between store to closing and read of rg/wg in netpollunblock
	rg = netpollunblock(pd, 'r', false)
	wg = netpollunblock(pd, 'w', false)
	if pd.rt.f != nil {
		deltimer(&pd.rt)
		pd.rt.f = nil
	}
	if pd.wt.f != nil {
		deltimer(&pd.wt)
		pd.wt.f = nil
	}
	unlock(&pd.lock)
	if rg != nil {
		netpollgoready(rg, 3)
	}
	if wg != nil {
		netpollgoready(wg, 3)
	}
}

// netpollready is called by the platform-specific netpoll function.
// It declares that the fd associated with pd is ready for I/O.
// The toRun argument is used to build a list of goroutines to return
// from netpoll. The mode argument is 'r', 'w', or 'r'+'w' to indicate
// whether the fd is ready for reading or writing or both.
// netpollready是由特定平台的netpoll函数调用的。它声明与pd相关的fd已经准备好进行I/O。
// toRun 参数用于建立一个从 netpoll 返回的 goroutines 列表。mode参数是'r'、'w'或
// 'r'+'w'，以表示fd是准备好进行读写或同时进行读写。
//
// This may run while the world is stopped, so write barriers are not allowed.
//go:nowritebarrier
// 这可能在世界停止时运行，所以不允许有写障碍。
func netpollready(toRun *gList, pd *pollDesc, mode int32) {
	var rg, wg *g
	if mode == 'r' || mode == 'r'+'w' {
		rg = netpollunblock(pd, 'r', true)
	}
	if mode == 'w' || mode == 'r'+'w' {
		wg = netpollunblock(pd, 'w', true)
	}
	if rg != nil {
		toRun.push(rg)
	}
	if wg != nil {
		toRun.push(wg)
	}
}

func netpollcheckerr(pd *pollDesc, mode int32) int {
	if pd.closing {
		return pollErrClosing
	}
	if (mode == 'r' && pd.rd < 0) || (mode == 'w' && pd.wd < 0) {
		return pollErrTimeout
	}
	// Report an event scanning error only on a read event.
	// An error on a write event will be captured in a subsequent
	// write call that is able to report a more specific error.
	if mode == 'r' && pd.everr {
		return pollErrNotPollable
	}
	return pollNoError
}

func netpollblockcommit(gp *g, gpp unsafe.Pointer) bool {
	r := atomic.Casuintptr((*uintptr)(gpp), pdWait, uintptr(unsafe.Pointer(gp)))
	if r {
		// Bump the count of goroutines waiting for the poller.
		// The scheduler uses this to decide whether to block
		// waiting for the poller if there is nothing else to do.
		atomic.Xadd(&netpollWaiters, 1)
	}
	return r
}

func netpollgoready(gp *g, traceskip int) {
	atomic.Xadd(&netpollWaiters, -1)
	goready(gp, traceskip+1)
}

// returns true if IO is ready, or false if timedout or closed
// waitio - wait only for completed IO, ignore errors
// 如果IO准备好了，则返回true；如果超时或关闭，则返回false
// waitio - 只等待完成的IO，忽略错误
func netpollblock(pd *pollDesc, mode int32, waitio bool) bool {
	gpp := &pd.rg
	if mode == 'w' {
		gpp = &pd.wg
	}

	// set the gpp semaphore to pdWait
	//将gpp信号灯设为pdWait
	for {
		old := *gpp
		if old == pdReady {
			*gpp = 0
			return true
		}
		if old != 0 {
			throw("runtime: double wait")
		}
		if atomic.Casuintptr(gpp, 0, pdWait) {
			break
		}
	}

	// need to recheck error states after setting gpp to pdWait
	// this is necessary because runtime_pollUnblock/runtime_pollSetDeadline/deadlineimpl
	// do the opposite: store to closing/rd/wd, membarrier, load of rg/wg
	// 在将gpp设置为pdWait后需要重新检查错误状态，这是必要的，
	// 因为runtime_pollUnblock/runtime_pollSetDeadline/deadlineimpl做的是相反的事情：
	// 存储到closing/rd/wd，membarrier，加载rg/wg。
	if waitio || netpollcheckerr(pd, mode) == 0 {
		gopark(netpollblockcommit, unsafe.Pointer(gpp), waitReasonIOWait, traceEvGoBlockNet, 5)
	}
	// be careful to not lose concurrent pdReady notification
	old := atomic.Xchguintptr(gpp, 0)
	if old > pdWait {
		throw("runtime: corrupted polldesc")
	}
	return old == pdReady
}

/*
	runtime.netpollunblock 会在读写事件发生时，将 runtime.pollDesc 中的读或者写信号量转换
	成 pdReady 并返回其中存储的 Goroutine；如果返回的 Goroutine 不会为空，那么运行时会将该
	Goroutine加入toRun列表，并将列表中的全部 Goroutine 加入运行队列并等待调度器的调度。
*/
func netpollunblock(pd *pollDesc, mode int32, ioready bool) *g {
	gpp := &pd.rg
	if mode == 'w' {
		gpp = &pd.wg
	}

	for {
		old := *gpp
		if old == pdReady {
			return nil
		}
		if old == 0 && !ioready {
			// Only set pdReady for ioready. runtime_pollWait
			// will check for timeout/cancel before waiting.
			return nil
		}
		var new uintptr
		if ioready {
			new = pdReady
		}
		if atomic.Casuintptr(gpp, old, new) {
			if old == pdWait {
				old = 0
			}
			return (*g)(unsafe.Pointer(old))
		}
	}
}

func netpolldeadlineimpl(pd *pollDesc, seq uintptr, read, write bool) {
	lock(&pd.lock)
	// Seq arg is seq when the timer was set.
	// If it's stale, ignore the timer event.
	currentSeq := pd.rseq
	if !read {
		currentSeq = pd.wseq
	}
	if seq != currentSeq {
		// The descriptor was reused or timers were reset.
		unlock(&pd.lock)
		return
	}
	var rg *g
	if read {
		if pd.rd <= 0 || pd.rt.f == nil {
			throw("runtime: inconsistent read deadline")
		}
		pd.rd = -1
		atomic.StorepNoWB(unsafe.Pointer(&pd.rt.f), nil) // full memory barrier between store to rd and load of rg in netpollunblock
		rg = netpollunblock(pd, 'r', false)
	}
	var wg *g
	if write {
		if pd.wd <= 0 || pd.wt.f == nil && !read {
			throw("runtime: inconsistent write deadline")
		}
		pd.wd = -1
		atomic.StorepNoWB(unsafe.Pointer(&pd.wt.f), nil) // full memory barrier between store to wd and load of wg in netpollunblock
		wg = netpollunblock(pd, 'w', false)
	}
	unlock(&pd.lock)
	if rg != nil {
		netpollgoready(rg, 0)
	}
	if wg != nil {
		netpollgoready(wg, 0)
	}
}

func netpollDeadline(arg interface{}, seq uintptr) {
	netpolldeadlineimpl(arg.(*pollDesc), seq, true, true)
}

func netpollReadDeadline(arg interface{}, seq uintptr) {
	netpolldeadlineimpl(arg.(*pollDesc), seq, true, false)
}

func netpollWriteDeadline(arg interface{}, seq uintptr) {
	netpolldeadlineimpl(arg.(*pollDesc), seq, false, true)
}

func (c *pollCache) alloc() *pollDesc {
	// 1、上锁
	lock(&c.lock)
	// 2、c.first为空的话，就先分配
	if c.first == nil {
		const pdSize = unsafe.Sizeof(pollDesc{})
		// 4kb / pdSize，看后面又乘上除数的结果，应该是为了分配大小是pdSize的整数倍
		n := pollBlockSize / pdSize
		if n == 0 {
			n = 1
		}
		// Must be in non-GC memory because can be referenced
		// only from epoll/kqueue internals.
		// 必须在非GC内存中，因为只能从epoll/kqueue内部引用。
		mem := persistentalloc(n*pdSize, 0, &memstats.other_sys)
		for i := uintptr(0); i < n; i++ {
			pd := (*pollDesc)(add(mem, i*pdSize))
			pd.link = c.first
			c.first = pd
		}
	}
	pd := c.first
	c.first = pd.link
	// 3、初始化pd.lock
	lockInit(&pd.lock, lockRankPollDesc)
	// 4、解锁
	unlock(&c.lock)
	return pd
}

// makeArg converts pd to an interface{}.
// makeArg does not do any allocation. Normally, such
// a conversion requires an allocation because pointers to
// go:notinheap types (which pollDesc is) must be stored
// in interfaces indirectly. See issue 42076.
func (pd *pollDesc) makeArg() (i interface{}) {
	x := (*eface)(unsafe.Pointer(&i))
	x._type = pdType
	x.data = unsafe.Pointer(&pd.self)
	return
}

var (
	pdEface interface{} = (*pollDesc)(nil)
	pdType  *_type      = efaceOf(&pdEface)._type
)
