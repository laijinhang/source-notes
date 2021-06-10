// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"runtime/internal/atomic"
	"runtime/internal/sys"
	"unsafe"
)

// defined constants
// 定义常量
const (
	// G status
	// G状态
	//
	// Beyond indicating the general state of a G, the G status
	// acts like a lock on the goroutine's stack (and hence its
	// ability to execute user code).
	// 除了指示G的一般状态外，G状态还充当goroutine堆栈上的锁(因此它具有执行用户代码的能力)。
	//
	// If you add to this list, add to the list
	// of "okay during garbage collection" status
	// in mgcmark.go too.
	// 如果您添加到此列表中，请同时添加到mgcmark.go中
	// 的“垃圾收集期间正常”状态列表中。
	//
	// TODO(austin): The _Gscan bit could be much lighter-weight.
	// For example, we could choose not to run _Gscanrunnable
	// goroutines found in the run queue, rather than CAS-looping
	// until they become _Grunnable. And transitions like
	// _Gscanwaiting -> _Gscanrunnable are actually okay because
	// they don't affect stack ownership.

	// _Gidle means this goroutine was just allocated and has not
	// yet been initialized.
	// 刚刚被分配并且还没有被初始化，值为0，为创建goroutine后的默认值
	_Gidle = iota // 0

	// _Grunnable means this goroutine is on a run queue. It is
	// not currently executing user code. The stack is not owned.
	// _Grunnable表示这个goroutine在运行队列上。它当前不是执行用户代码，没有栈的所有权
	_Grunnable // 1

	// _Grunning means this goroutine may execute user code. The
	// stack is owned by this goroutine. It is not on a run queue.
	// It is assigned an M and a P (g.m and g.m.p are valid).
	// Grunning意味着这个goroutine可以执行用户代码。堆栈属于此goroutine。
	//它不在运行队列中。它被分配了一个M和一个P（g.M和g.M.P是有效的）。
	_Grunning // 2

	// _Gsyscall means this goroutine is executing a system call.
	// It is not executing user code. The stack is owned by this
	// goroutine. It is not on a run queue. It is assigned an M.
	// _Gsyscall表示这个goroutine正在执行一个系统调用。它不是在执行用户代码。
	// 栈属于这个goroutine。它不在运行队列上。它被某个M绑定。
	_Gsyscall // 3

	// _Gwaiting means this goroutine is blocked in the runtime.
	// It is not executing user code. It is not on a run queue,
	// but should be recorded somewhere (e.g., a channel wait
	// queue) so it can be ready()d when necessary. The stack is
	// not owned *except* that a channel operation may read or
	// write parts of the stack under the appropriate channel
	// lock. Otherwise, it is not safe to access the stack after a
	// goroutine enters _Gwaiting (e.g., it may get moved).
	// _Gwaiting表示此goroutine在运行时被阻止。它没有执行用户代码。
	// 它不在运行队列中，但应该记录在某个地方(例如，通道等待队列)，以
	// 便在必要时准备就绪。栈是不被拥有的，除非一个通道操作可以在适当的
	// 通道锁下读或写栈的一部分。否则，在goroutine进入_Gwaiting后
	// 访问堆栈是不安全的(例如，它可能被移动)。

	/* 被阻塞的goroutine，阻塞在某个channel的发送或者接收队列 */
	_Gwaiting // 4

	// _Gmoribund_unused is currently unused, but hardcoded in gdb
	// scripts.
	_Gmoribund_unused // 5

	// _Gdead means this goroutine is currently unused. It may be
	// just exited, on a free list, or just being initialized. It
	// is not executing user code. It may or may not have a stack
	// allocated. The G and its stack (if any) are owned by the M
	// that is exiting the G or that obtained the G from the free
	// list.

	/*
		当前goroutine未被使用，没有执行代码，金额能有分配的栈，分布在空闲队列gFree，
		可能是一个刚刚初始化的goroutine，也可能是执行了goexit退出的goroutine。
	*/
	_Gdead // 6

	// _Genqueue_unused is currently unused.
	_Genqueue_unused // 7

	// _Gcopystack means this goroutine's stack is being moved. It
	// is not executing user code and is not on a run queue. The
	// stack is owned by the goroutine that put it in _Gcopystack.

	/* 栈正在被拷贝，没有执行代码，不在运行队列， */
	_Gcopystack // 8

	// _Gpreempted means this goroutine stopped itself for a
	// suspendG preemption. It is like _Gwaiting, but nothing is
	// yet responsible for ready()ing it. Some suspendG must CAS
	// the status to _Gwaiting to take responsibility for
	// ready()ing this G.
	_Gpreempted // 9

	// _Gscan combined with one of the above states other than
	// _Grunning indicates that GC is scanning the stack. The
	// goroutine is not executing user code and the stack is owned
	// by the goroutine that set the _Gscan bit.
	//
	// _Gscanrunning is different: it is used to briefly block
	// state transitions while GC signals the G to scan its own
	// stack. This is otherwise like _Grunning.
	//
	// atomicstatus&~Gscan gives the state the goroutine will
	// return to when the scan completes.
	/* GC正在扫描栈空间，没有执行代码，可以与其他状态同时存在 */
	_Gscan          = 0x1000
	_Gscanrunnable  = _Gscan + _Grunnable  // 0x1001
	_Gscanrunning   = _Gscan + _Grunning   // 0x1002
	_Gscansyscall   = _Gscan + _Gsyscall   // 0x1003
	_Gscanwaiting   = _Gscan + _Gwaiting   // 0x1004
	_Gscanpreempted = _Gscan + _Gpreempted // 0x1009
)

const (
	// P status
	// P的状态

	// _Pidle means a P is not being used to run user code or the
	// scheduler. Typically, it's on the idle P list and available
	// to the scheduler, but it may just be transitioning between
	// other states.
	//
	// The P is owned by the idle list or by whatever is
	// transitioning its state. Its run queue is empty.
	/* 处理器没有运行蝇虎代码或者调度器，被空闲队列或者改变其状态的结构持有，运行队列为空 */
	_Pidle = iota

	// _Prunning means a P is owned by an M and is being used to
	// run user code or the scheduler. Only the M that owns this P
	// is allowed to change the P's status from _Prunning. The M
	// may transition the P to _Pidle (if it has no more work to
	// do), _Psyscall (when entering a syscall), or _Pgcstop (to
	// halt for the GC). The M may also hand ownership of the P
	// off directly to another M (e.g., to schedule a locked G).
	/* 被线程M持有，并且正在执行用户代码或者调度器 */
	_Prunning

	// _Psyscall means a P is not running user code. It has
	// affinity to an M in a syscall but is not owned by it and
	// may be stolen by another M. This is similar to _Pidle but
	// uses lightweight transitions and maintains M affinity.
	//
	// Leaving _Psyscall must be done with a CAS, either to steal
	// or retake the P. Note that there's an ABA hazard: even if
	// an M successfully CASes its original P back to _Prunning
	// after a syscall, it must understand the P may have been
	// used by another M in the interim.

	/* 没有执行用户代码，当前线程陷入系统调用 */
	_Psyscall

	// _Pgcstop means a P is halted for STW and owned by the M
	// that stopped the world. The M that stopped the world
	// continues to use its P, even in _Pgcstop. Transitioning
	// from _Prunning to _Pgcstop causes an M to release its P and
	// park.
	//
	// The P retains its run queue and startTheWorld will restart
	// the scheduler on Ps with non-empty run queues.

	/* 被线程M持有，当前处理器由于垃圾回收被停止 */
	_Pgcstop

	// _Pdead means a P is no longer used (GOMAXPROCS shrank). We
	// reuse Ps if GOMAXPROCS increases. A dead P is mostly
	// stripped of its resources, though a few things remain
	// (e.g., trace buffers).

	/* 当前处理器已经不被处理 */
	_Pdead
)

// Mutual exclusion locks.  In the uncontended case,
// as fast as spin locks (just a few user-level instructions),
// but on the contention path they sleep in the kernel.
// A zeroed Mutex is unlocked (no need to initialize each lock).
// Initialization is helpful for static lock ranking, but not required.
type mutex struct {
	// Empty struct if lock ranking is disabled, otherwise includes the lock rank
	lockRankStruct
	// Futex-based impl treats it as uint32 key,
	// while sema-based impl as M* waitm.
	// Used to be a union, but unions break precise GC.
	key uintptr
}

// sleep and wakeup on one-time events.
// before any calls to notesleep or notewakeup,
// must call noteclear to initialize the Note.
// then, exactly one thread can call notesleep
// and exactly one thread can call notewakeup (once).
// once notewakeup has been called, the notesleep
// will return.  future notesleep will return immediately.
// subsequent noteclear must be called only after
// previous notesleep has returned, e.g. it's disallowed
// to call noteclear straight after notewakeup.
//
// notetsleep is like notesleep but wakes up after
// a given number of nanoseconds even if the event
// has not yet happened.  if a goroutine uses notetsleep to
// wake up early, it must wait to call noteclear until it
// can be sure that no other goroutine is calling
// notewakeup.
//
// notesleep/notetsleep are generally called on g0,
// notetsleepg is similar to notetsleep but is called on user g.
type note struct {
	// Futex-based impl treats it as uint32 key,
	// while sema-based impl as M* waitm.
	// Used to be a union, but unions break precise GC.
	key uintptr
}

type funcval struct {
	fn uintptr
	// variable-size, fn-specific data here
}

/*
iface表示non-empty interface，即包含方法的接口

一般常用于定义接口

interface可以作为中间层进行解耦，将具体的实现和调用完全分离，上层的模块就不需要依赖某一个具体的实现，只需要依赖一个定义好的接口。
*/
type iface struct {
	tab  *itab
	data unsafe.Pointer
}

/*
eface表示empty interface，不含任何方法

一般用于存数据，如变量等
*/
type eface struct {
	_type *_type         // 实际类型，_type是Go语言中所有类型的公共描述，几乎所有的数据结构都可以抽象成_type
	data  unsafe.Pointer // 指向实际数据
}

func efaceOf(ep *interface{}) *eface {
	return (*eface)(unsafe.Pointer(ep))
}

// The guintptr, muintptr, and puintptr are all used to bypass write barriers.
// It is particularly important to avoid write barriers when the current P has
// been released, because the GC thinks the world is stopped, and an
// unexpected write barrier would not be synchronized with the GC,
// which can lead to a half-executed write barrier that has marked the object
// but not queued it. If the GC skips the object and completes before the
// queuing can occur, it will incorrectly free the object.
//
// We tried using special assignment functions invoked only when not
// holding a running P, but then some updates to a particular memory
// word went through write barriers and some did not. This breaks the
// write barrier shadow checking mode, and it is also scary: better to have
// a word that is completely ignored by the GC than to have one for which
// only a few updates are ignored.
//
// Gs and Ps are always reachable via true pointers in the
// allgs and allp lists or (during allocation before they reach those lists)
// from stack variables.
//
// Ms are always reachable via true pointers either from allm or
// freem. Unlike Gs and Ps we do free Ms, so it's important that
// nothing ever hold an muintptr across a safe point.

// A guintptr holds a goroutine pointer, but typed as a uintptr
// to bypass write barriers. It is used in the Gobuf goroutine state
// and in scheduling lists that are manipulated without a P.
//
// The Gobuf.g goroutine pointer is almost always updated by assembly code.
// In one of the few places it is updated by Go code - func save - it must be
// treated as a uintptr to avoid a write barrier being emitted at a bad time.
// Instead of figuring out how to emit the write barriers missing in the
// assembly manipulation, we change the type of the field to uintptr,
// so that it does not require write barriers at all.
//
// Goroutine structs are published in the allg list and never freed.
// That will keep the goroutine structs from being collected.
// There is never a time that Gobuf.g's contain the only references
// to a goroutine: the publishing of the goroutine in allg comes first.
// Goroutine pointers are also kept in non-GC-visible places like TLS,
// so I can't see them ever moving. If we did want to start moving data
// in the GC, we'd need to allocate the goroutine structs from an
// alternate arena. Using guintptr doesn't make that problem any worse.
type guintptr uintptr

//go:nosplit
func (gp guintptr) ptr() *g { return (*g)(unsafe.Pointer(gp)) }

//go:nosplit
func (gp *guintptr) set(g *g) { *gp = guintptr(unsafe.Pointer(g)) }

//go:nosplit
func (gp *guintptr) cas(old, new guintptr) bool {
	return atomic.Casuintptr((*uintptr)(unsafe.Pointer(gp)), uintptr(old), uintptr(new))
}

// setGNoWB performs *gp = new without a write barrier.
// For times when it's impractical to use a guintptr.
//go:nosplit
//go:nowritebarrier
func setGNoWB(gp **g, new *g) {
	(*guintptr)(unsafe.Pointer(gp)).set(new)
}

type puintptr uintptr

//go:nosplit
func (pp puintptr) ptr() *p { return (*p)(unsafe.Pointer(pp)) }

//go:nosplit
func (pp *puintptr) set(p *p) { *pp = puintptr(unsafe.Pointer(p)) }

// muintptr is a *m that is not tracked by the garbage collector.
//
// Because we do free Ms, there are some additional constrains on
// muintptrs:
//
// 1. Never hold an muintptr locally across a safe point.
//
// 2. Any muintptr in the heap must be owned by the M itself so it can
//    ensure it is not in use when the last true *m is released.
type muintptr uintptr

//go:nosplit
func (mp muintptr) ptr() *m { return (*m)(unsafe.Pointer(mp)) }

//go:nosplit
func (mp *muintptr) set(m *m) { *mp = muintptr(unsafe.Pointer(m)) }

// setMNoWB performs *mp = new without a write barrier.
// For times when it's impractical to use an muintptr.
//go:nosplit
//go:nowritebarrier
func setMNoWB(mp **m, new *m) {
	(*muintptr)(unsafe.Pointer(mp)).set(new)
}

/*
gobuf结构体用于保存goroutine的调度信息，主要包含CPU的几个寄存器的值
*/
type gobuf struct {
	// The offsets of sp, pc, and g are known to (hard-coded in) libmach.
	//
	// ctxt is unusual with respect to GC: it may be a
	// heap-allocated funcval, so GC needs to track it, but it
	// needs to be set and cleared from assembly, where it's
	// difficult to have write barriers. However, ctxt is really a
	// saved, live register, and we only ever exchange it between
	// the real register and the gobuf. Hence, we treat it as a
	// root during stack scanning, which means assembly that saves
	// and restores it doesn't need write barriers. It's still
	// typed as a pointer so that any other writes from Go get
	// write barriers.
	// 栈指针
	sp uintptr // 保存CPU的rsp寄存器的值
	// 程序计数器
	pc uintptr // 保存CPU的rip寄存器的值
	// gobuf对应的Goroutine
	g    guintptr // 记录当前这个gobuf对象属于哪个goroutine
	ctxt unsafe.Pointer
	// 系统调用的返回值，因为从系统调用返回之后如果p被其他工作线程抢占，
	// 则这个goroutine会被放入全局队列被其它工作线程调度，其它线程需要知道系统调用的返回值
	ret uintptr
	lr  uintptr
	// 保存CPU的rip寄存器的值
	bp uintptr // for framepointer-enabled architectures
}

// sudog represents a g in a wait list, such as for sending/receiving
// on a channel.
//
// sudog is necessary because the g ↔ synchronization object relation
// is many-to-many. A g can be on many wait lists, so there may be
// many sudogs for one g; and many gs may be waiting on the same
// synchronization object, so there may be many sudogs for one object.
//
// sudogs are allocated from a special pool. Use acquireSudog and
// releaseSudog to allocate and free them.
type sudog struct {
	// The following fields are protected by the hchan.lock of the
	// channel this sudog is blocking on. shrinkstack depends on
	// this for sudogs involved in channel ops.

	g *g

	next *sudog
	prev *sudog
	elem unsafe.Pointer // data element (may point to stack)

	// The following fields are never accessed concurrently.
	// For channels, waitlink is only accessed by g.
	// For semaphores, all fields (including the ones above)
	// are only accessed when holding a semaRoot lock.

	acquiretime int64
	releasetime int64
	ticket      uint32

	// isSelect indicates g is participating in a select, so
	// g.selectDone must be CAS'd to win the wake-up race.
	isSelect bool

	// success indicates whether communication over channel c
	// succeeded. It is true if the goroutine was awoken because a
	// value was delivered over channel c, and false if awoken
	// because c was closed.
	success bool

	parent   *sudog // semaRoot binary tree
	waitlink *sudog // g.waiting list or semaRoot
	waittail *sudog // semaRoot
	c        *hchan // channel
}

type libcall struct {
	fn   uintptr
	n    uintptr // number of parameters
	args uintptr // parameters
	r1   uintptr // return values
	r2   uintptr
	err  uintptr // error number
}

/*
stack结构体主要用来记录goroutine所使用的栈信息，包含栈顶和栈底位置
*/
// Stack describes a Go execution stack.
// The bounds of the stack are exactly [lo, hi),
// with no implicit data structures on either side.
type stack struct {
	lo uintptr // 栈顶，指向内存低地址
	hi uintptr // 栈底，指向内存高地址
}

// heldLockInfo gives info on a held lock and the rank of that lock
type heldLockInfo struct {
	lockAddr uintptr
	rank     lockRank
}

type g struct {
	// Stack parameters.
	// stack describes the actual stack memory: [stack.lo, stack.hi).
	// stackguard0 is the stack pointer compared in the Go stack growth prologue.
	// It is stack.lo+StackGuard normally, but can be StackPreempt to trigger a preemption.
	// stackguard1 is the stack pointer compared in the C stack growth prologue.
	// It is stack.lo+StackGuard on g0 and gsignal stacks.
	// It is ~0 on other goroutine stacks, to trigger a call to morestackc (and crash).
	// 当前 Goroutine 的栈内存范围[stack.lo, stack.hi]
	stack stack // offset known to runtime/cgo
	// 用于调度器抢占式调度
	stackguard0 uintptr // offset known to liblink
	stackguard1 uintptr // offset known to liblink

	// panic组成的链表
	_panic *_panic // innermost panic - offset known to liblink
	// defer组成的先进后出的链表，同栈
	_defer *_defer // innermost defer
	// 当前 Goroutine 占用的线程
	m *m // current m; offset known to arm liblink
	// 存储 Goroutine 的调度相关的数据
	sched     gobuf
	syscallsp uintptr // if status==Gsyscall, syscallsp = sched.sp to use during gc
	syscallpc uintptr // if status==Gsyscall, syscallpc = sched.pc to use during gc
	stktopsp  uintptr // expected sp at top of stack, to check in traceback
	// param is a generic pointer parameter field used to pass
	// values in particular contexts where other storage for the
	// parameter would be difficult to find. It is currently used
	// in three ways:
	// 1. When a channel operation wakes up a blocked goroutine, it sets param to
	//    point to the sudog of the completed blocking operation.
	// 2. By gcAssistAlloc1 to signal back to its caller that the goroutine completed
	//    the GC cycle. It is unsafe to do so in any other way, because the goroutine's
	//    stack may have moved in the meantime.
	// 3. By debugCallWrap to pass parameters to a new goroutine because allocating a
	//    closure in the runtime is forbidden.
	param unsafe.Pointer
	/*
		Goroutine的状态，
		_Gidle：0，刚刚被分配并且还没有被初始化
		_Grunnable：1，没有执行代码，没有栈的所有权，存储在运行队列中
		_Grunning：2，可以执行代码，拥有栈的所有权，被赋予了内核线程 M 和处理器 P
		_Gsyscall：3，正在执行系统调用，拥有栈的所有权，没有执行用户代码，被赋予了内核线程 M 但是不在运行队列上
		_Gwaiting：4，由于运行时而被阻塞，没有执行用户代码并且不在运行队列上，但是可能存在于 Channel 的等待队列上
		_Gdead：5，没有被使用，没有执行代码，可能有分配的栈
		_Gcopystack：6，栈正在被拷贝，没有执行代码，不在运行队列上
		_Gpreempted：7，由于抢占而被阻塞，没有执行用户代码并且不在运行队列上，等待唤醒
		_Gscan：0x1000，GC 正在扫描栈空间，没有执行代码，可以与其他状态同时存在
		_Gscanrunnable  = _Gscan + _Grunnable  // 0x1001
		_Gscanrunning   = _Gscan + _Grunning   // 0x1002
		_Gscansyscall   = _Gscan + _Gsyscall   // 0x1003
		_Gscanwaiting   = _Gscan + _Gwaiting   // 0x1004
		_Gscanpreempted = _Gscan + _Gpreempted // 0x1009
	*/
	atomicstatus uint32
	stackLock    uint32 // sigprof/scang lock; TODO: fold in to atomicstatus
	/*
		标识协程的唯一id

		[Golang 获取 goroutine id 完全指南](https://liudanking.com/performance/golang-%E8%8E%B7%E5%8F%96-goroutine-id-%E5%AE%8C%E5%85%A8%E6%8C%87%E5%8D%97/)
		这篇文章总结了五种方式在应用层来获取协程id
		通过stack信息获取goroutine id.
		通过修改源代码获取goroutine id.
		通过CGo获取goroutine id.
		通过汇编获取goroutine id.
		通过汇编获取伪goroutine id.
	*/
	goid       int64
	schedlink  guintptr
	waitsince  int64      // approx time when the g become blocked	// g被阻塞的大约时间
	waitreason waitReason // if status==Gwaiting

	// 抢占信号
	preempt bool // preemption signal, duplicates stackguard0 = stackpreempt
	// 抢占时将状态修改程 `_Gpreempted`
	preemptStop bool // transition to _Gpreempted on preemption; otherwise, just deschedule
	// 在同步安全点收缩栈
	preemptShrink bool // shrink stack at synchronous safe point

	// asyncSafePoint is set if g is stopped at an asynchronous
	// safe point. This means there are frames on the stack
	// without precise pointer information.
	asyncSafePoint bool

	paniconfault bool // panic (instead of crash) on unexpected fault address
	gcscandone   bool // g has scanned stack; protected by _Gscan bit in status
	throwsplit   bool // must not split stack
	// activeStackChans indicates that there are unlocked channels
	// pointing into this goroutine's stack. If true, stack
	// copying needs to acquire channel locks to protect these
	// areas of the stack.
	activeStackChans bool
	// parkingOnChan indicates that the goroutine is about to
	// park on a chansend or chanrecv. Used to signal an unsafe point
	// for stack shrinking. It's a boolean value, but is updated atomically.
	parkingOnChan uint8

	raceignore     int8     // ignore race detection events
	sysblocktraced bool     // StartTrace has emitted EvGoInSyscall about this goroutine
	tracking       bool     // whether we're tracking this G for sched latency statistics
	trackingSeq    uint8    // used to decide whether to track this G
	runnableStamp  int64    // timestamp of when the G last became runnable, only used when tracking
	runnableTime   int64    // the amount of time spent runnable, cleared when running, only used when tracking
	sysexitticks   int64    // cputicks when syscall has returned (for tracing)
	traceseq       uint64   // trace event sequencer
	tracelastp     puintptr // last P emitted an event for this goroutine
	/*
		G被锁定只在这个m上运行
	*/
	lockedm    muintptr
	sig        uint32
	writebuf   []byte
	sigcode0   uintptr
	sigcode1   uintptr
	sigpc      uintptr
	gopc       uintptr         // pc of go statement that created this goroutine
	ancestors  *[]ancestorInfo // ancestor information goroutine(s) that created this goroutine (only used if debug.tracebackancestors)
	startpc    uintptr         // pc of goroutine function
	racectx    uintptr
	waiting    *sudog         // sudog structures this g is waiting on (that have a valid elem ptr); in lock order
	cgoCtxt    []uintptr      // cgo traceback context
	labels     unsafe.Pointer // profiler labels
	timer      *timer         // cached timer for time.Sleep
	selectDone uint32         // are we participating in a select and did someone win the race?

	// Per-G GC state

	// gcAssistBytes is this G's GC assist credit in terms of
	// bytes allocated. If this is positive, then the G has credit
	// to allocate gcAssistBytes bytes without assisting. If this
	// is negative, then the G must correct this by performing
	// scan work. We track this in bytes to make it fast to update
	// and check for debt in the malloc hot path. The assist ratio
	// determines how this corresponds to scan work debt.
	gcAssistBytes int64
}

// gTrackingPeriod is the number of transitions out of _Grunning between
// latency tracking runs.
const gTrackingPeriod = 8

const (
	// tlsSlots is the number of pointer-sized slots reserved for TLS on some platforms,
	// like Windows.
	tlsSlots = 6
	tlsSize  = tlsSlots * sys.PtrSize
)

/*
M的结构，M是OS线程的实体
*/
type m struct {
	/*
	   1.  所有调用栈的Goroutine,这是一个比较特殊的Goroutine。
	   2.  普通的Goroutine栈是在Heap分配的可增长的stack,而g0的stack是M对应的线程栈。
	   3.  所有调度相关代码,会先切换到该Goroutine的栈再执行。
	*/
	// 持有调度栈的 Goroutine
	g0 *g // goroutine with scheduling stack	// 用于执行调度指令的 Goroutine
	/*

	 */
	morebuf gobuf // gobuf arg to morestack
	/*

	 */
	divmod uint32 // div/mod denominator for arm - known to liblink

	// Fields not known to debuggers.
	procid uint64 // for debuggers, but offset not hard-coded
	/*
		用于处理信号的goroutine
	*/
	gsignal    *g           // signal-handling g
	goSigStack gsignalStack // Go-allocated signal handling stack
	sigmask    sigset       // storage for saved signal mask
	// 线程本地存储 thread-local
	tls      [tlsSlots]uintptr // thread-local storage (for x86 extern register)	// 线程本地存储
	mstartfn func()
	// 当前运行的G
	curg      *g       // current running goroutine	// 当前运行的用户 Goroutine
	caughtsig guintptr // goroutine running during fatal signal
	// 正在运行代码的p
	p puintptr // attached p for executing go code (nil if not executing go code)	// 执行go代码时持有的P（如果没有执行则为nil）
	// 下一个要执行的p
	nextp puintptr
	// 之前使用的P
	oldp       puintptr // the p that was attached before executing a syscall
	id         int64
	mallocing  int32
	throwing   int32
	preemptoff string // if != "", keep curg running on this m
	locks      int32
	dying      int32
	profilehz  int32
	/*
		m是否out of work
	*/
	spinning bool // m is out of work and is actively looking for work	// m当前没有运行的work且正处于work的活跃状态
	/*
		m是否被阻塞
	*/
	blocked     bool // m is blocked on a note
	newSigstack bool // minit on C thread called sigaltstack
	printlock   int8
	/*
		m是否在在执行cgo调用
	*/
	incgo bool // m is executing a cgo call
	/*
		如果等于0，安全释放g0并删除m（原子性）。
	*/
	freeWait   uint32 // if == 0, safe to free g0 and delete m (atomic)
	fastrand   [2]uint32
	needextram bool
	traceback  uint8
	/*
		cgo调用的总数
	*/
	ncgocall uint64 // number of cgo calls in total
	/*
		当前cgo调用的数目
	*/
	ncgo          int32       // number of cgo calls currently in progress
	cgoCallersUse uint32      // if non-zero, cgoCallers in use temporarily	// cgo调用崩溃的cgo回溯
	cgoCallers    *cgoCallers // cgo traceback if crashing in cgo call
	doesPark      bool        // non-P running threads: sysmon and newmHandoff never use .park
	park          note
	/*
		用于链接allm
	*/
	alllink   *m // on allm
	schedlink muintptr
	/*
		锁定g在当前m上执行，而不会切换到其他m
	*/
	lockedg guintptr
	/*
		thread创建的栈
	*/
	createstack   [32]uintptr // stack that created this thread.
	lockedExt     uint32      // tracking for external LockOSThread
	lockedInt     uint32      // tracking for internal lockOSThread
	nextwaitm     muintptr    // next m waiting for lock
	waitunlockf   func(*g, unsafe.Pointer) bool
	waitlock      unsafe.Pointer
	waittraceev   byte
	waittraceskip int
	startingtrace bool
	syscalltick   uint32
	freelink      *m // on sched.freem

	// mFixup is used to synchronize OS related m state
	// (credentials etc) use mutex to access. To avoid deadlocks
	// an atomic.Load() of used being zero in mDoFixupFn()
	// guarantees fn is nil.
	mFixup struct {
		lock mutex
		used uint32
		fn   func(bool) bool
	}

	// these are here because they are too large to be on the stack
	// of low-level NOSPLIT functions.
	libcall   libcall
	libcallpc uintptr // for cpu profiler
	libcallsp uintptr
	libcallg  guintptr
	syscall   libcall // stores syscall parameters on windows

	vdsoSP uintptr // SP for traceback while in VDSO call (0 if not in call)
	vdsoPC uintptr // PC for traceback while in VDSO call

	// preemptGen counts the number of completed preemption
	// signals. This is used to detect when a preemption is
	// requested, but fails. Accessed atomically.
	preemptGen uint32

	// Whether this is a pending preemption signal on this M.
	// Accessed atomically.
	signalPending uint32

	dlogPerM

	mOS

	// Up to 10 locks held by this m, maintained by the lock ranking code.
	locksHeldLen int
	locksHeld    [10]heldLockInfo
}

/*
P的结构
P只是处理器的抽象，而被处理器本身，它存在的意义在于实现工作窃取（work stealing）算法。简单来说，每个P持有一个G的本地队列。
在没有P的情况下，所以G只能放在一个全局队列中，当M执行完而没有G可执行时，虽然仍然会先检查全局队列、网络，
但这时增加了一个从其他P的队列偷取（steal）一个G来执行的过程。优先级为本地 > 全局 > 网络 > 偷取。
偷取：当有若个P时，其中有P当前维护的本地G队列为空时，而全局队列中存在G，那么本地队列为空的P会去全局队列中偷取。
*/
type p struct {
	id int32
	/*
		_Pidle：空闲中，当M发现无待运行的G时会进入休眠，这时M拥有的P会变为空闲并加到空闲P链表中
		_Prunning：运行中，当M拥有一个P后，这个P的状态就会变为运行中，M运行G会使用这个P中的置业
		_Psyscall：系统调用中
		_Pgctop：GC STW时，P会变为此状态
		_Pdead：已终止，当P的数量在运行中时改变，且数量减少时多余的P会变为此状态
	*/
	status      uint32 // one of pidle/prunning/...	// p的状态 pidle/prunning/...
	link        puintptr
	schedtick   uint32     // incremented on every scheduler call	// 每次调度程序调用时递增
	syscalltick uint32     // incremented on every system call		// 每次系统调用时递增
	sysmontick  sysmontick // last tick observed by sysmon			// sysmon观察到的最后一个刻度
	m           muintptr   // back-link to associated m (nil if idle)	// 反向链接到关联的m（nil则表示idle）
	/*
		当前m的内存缓存
	*/
	mcache      *mcache
	pcache      pageCache
	raceprocctx uintptr

	// defer 结构池
	deferpool    [5][]*_defer // pool of available defer structs of different sizes (see panic.go)	// 不同大小的可用的defer结构池
	deferpoolbuf [5][32]*_defer

	// Cache of goroutine ids, amortizes accesses to runtime·sched.goidgen.
	// goroutine ID的缓存将分摊对runtime.sched.goidgen的访问。
	goidcache    uint64
	goidcacheend uint64

	// Queue of runnable goroutines. Accessed without lock.
	runqhead uint32 // 可运行的 Goroutine 队列，可无锁访问
	runqtail uint32
	runq     [256]guintptr
	// runnext, if non-nil, is a runnable G that was ready'd by
	// the current G and should be run next instead of what's in
	// runq if there's time remaining in the running G's time
	// slice. It will inherit the time left in the current time
	// slice. If a set of goroutines is locked in a
	// communicate-and-wait pattern, this schedules that set as a
	// unit and eliminates the (potentially large) scheduling
	// latency that otherwise arises from adding the ready'd
	// goroutines to the end of the run queue.
	//
	// Note that while other P's may atomically CAS this to zero,
	// only the owner P can CAS it to a valid G.

	// runnext，如果不是nil，则是一个可运行的G，它由当前G准备好，如果运行G的时间片中还有剩余时间，
	// 则应该运行next而不是runq中的内容。它将继承当前时间片中剩余的时间。如果一组goroutine被锁定
	// 在一个communicate and wait模式中，那么这会将其作为一个单元进行调度，并消除由于将准备好的
	// goroutine添加到运行队列末尾而产生的（可能较大的）调度延迟。
	runnext guintptr

	// Available G's (status == Gdead)
	// 可用G (G状态status等于 Gdead)列表
	gFree struct {
		gList
		n int32
	}

	sudogcache []*sudog
	sudogbuf   [128]*sudog

	// Cache of mspan objects from the heap.
	// 从堆中缓存mspan对象。
	mspancache struct {
		// We need an explicit length here because this field is used
		// in allocation codepaths where write barriers are not allowed,
		// and eliminating the write barrier/keeping it eliminated from
		// slice updates is tricky, moreso than just managing the length
		// ourselves.
		len int
		buf [128]*mspan
	}

	tracebuf traceBufPtr

	// traceSweep indicates the sweep events should be traced.
	// This is used to defer the sweep start event until a span
	// has actually been swept.
	traceSweep bool
	// traceSwept and traceReclaimed track the number of bytes
	// swept and reclaimed by sweeping in the current sweep loop.
	traceSwept, traceReclaimed uintptr

	palloc persistentAlloc // per-P to avoid mutex

	_ uint32 // Alignment for atomic fields below

	// The when field of the first entry on the timer heap.
	// This is updated using atomic functions.
	// This is 0 if the timer heap is empty.
	timer0When uint64

	// The earliest known nextwhen field of a timer with
	// timerModifiedEarlier status. Because the timer may have been
	// modified again, there need not be any timer with this value.
	// This is updated using atomic functions.
	// This is 0 if the value is unknown.
	timerModifiedEarliest uint64

	// Per-P GC state
	gcAssistTime         int64 // Nanoseconds in assistAlloc
	gcFractionalMarkTime int64 // Nanoseconds in fractional mark worker (atomic)

	// gcMarkWorkerMode is the mode for the next mark worker to run in.
	// That is, this is used to communicate with the worker goroutine
	// selected for immediate execution by
	// gcController.findRunnableGCWorker. When scheduling other goroutines,
	// this field must be set to gcMarkWorkerNotWorker.
	gcMarkWorkerMode gcMarkWorkerMode
	// gcMarkWorkerStartTime is the nanotime() at which the most recent
	// mark worker started.
	gcMarkWorkerStartTime int64

	// gcw is this P's GC work buffer cache. The work buffer is
	// filled by write barriers, drained by mutator assists, and
	// disposed on certain GC state transitions.
	gcw gcWork

	// wbBuf is this P's GC write barrier buffer.
	//
	// TODO: Consider caching this in the running G.
	wbBuf wbBuf

	runSafePointFn uint32 // if 1, run sched.safePointFn at next safe point

	// statsSeq is a counter indicating whether this P is currently
	// writing any stats. Its value is even when not, odd when it is.
	statsSeq uint32

	// Lock for timers. We normally access the timers while running
	// on this P, but the scheduler can also do it from a different P.
	// timers 字段的锁。我们通常在 P 运行时访问 timers，但 scheduler 仍可以
	// 在不同的 P 上进行访问。
	timersLock mutex

	// Actions to take at some time. This is used to implement the
	// standard library's time package.
	// Must hold timersLock to access.
	// 某段时间需要进行的动作。用于实现 time 包。
	timers []*timer

	// Number of timers in P's heap.
	// Modified using atomic instructions.
	numTimers uint32

	// Number of timerModifiedEarlier timers on P's heap.
	// This should only be modified while holding timersLock,
	// or while the timer status is in a transient state
	// such as timerModifying.
	// 在 P 堆中 timerModifiedEarlier timers 的数量。
	// 仅当持有 timersLock 或者当 timer 状态转换为 timerModifying 时才可以修改
	adjustTimers uint32

	// Number of timerDeleted timers in P's heap.
	// Modified using atomic instructions.
	deletedTimers uint32

	// Race context used while executing timer functions.
	timerRaceCtx uintptr

	// preempt is set to indicate that this P should be enter the
	// scheduler ASAP (regardless of what G is running on it).
	// 抢占调度标志，如果需要抢占调度，设置preempt为true
	preempt bool

	// Padding is no longer needed. False sharing is now not a worry because p is large enough
	// that its size class is an integer multiple of the cache line size (for any of our architectures).
}

/*
调度器sched结构
调度器，所有 Goroutine 被调度的核心，存放了调度器持有的全局资源，访问这些资源需要持有锁：
* 管理了能够将G和M进行绑定的M队列
* 管理了空闲的P链表（队列）
* 管理了G的全局队列
* 管理了可被复用的G的全局缓存
* 管理了defer池
*/
type schedt struct {
	// accessed atomically. keep at top to ensure alignment on 32-bit systems.
	// 在32位系统上保持在顶部以确保对齐。
	/*
		Sched.goidgen 用于为最后（最新创建）一个goroutine分配的id（goid），相当于一个全局计数器
		启动时 sched.goidgen=0，因此主 Goroutine 的goid为1
	*/
	goidgen uint64
	/*
		最后一次网络轮询的时间，如果目前正在轮询则为0
	*/
	lastpoll uint64 // time of last network poll, 0 if currently polling
	/*
		当前轮询的睡眠时间
	*/
	pollUntil uint64 // time to which current poll is sleeping

	lock mutex

	// When increasing nmidle, nmidlelocked, nmsys, or nmfreed, be
	// sure to call checkdead().
	// 在增加nmidle、nmidlelocked、nmsys或nmfreed时，一定要调用checkdead()。

	// 空闲的 M 列表
	midle muintptr // idle m's waiting for work
	// 空闲的 M 列表数量
	nmidle int32 // number of idle m's waiting for work
	// lockde状态的m个数
	nmidlelocked int32 // number of locked m's waiting for work
	// 下一个被创建的 M 的 id
	mnext int64 // number of m's that have been created and next M ID
	// 表示最多所能创建的工作线程数量
	maxmcount int32 // maximum number of m's allowed (or die)
	// 不计入死锁的系统M的数量
	nmsys int32 // number of system m's not counted for deadlock
	// 累计释放的M的数量（空闲m的数量）
	nmfreed int64 // cumulative number of freed m's

	// 系统中goroutine的数目，会自动更新
	ngsys uint32 // number of system goroutines; updated atomically

	pidle  puintptr // idle p's	// 空闲p链表
	npidle uint32   // 空闲p数量
	// 自旋状态的M的数量
	nmspinning uint32 // See "Worker thread parking/unparking" comment in proc.go.

	// Global runnable queue.
	// 全局 runnable G 队列
	runq     gQueue
	runqsize int32

	// disable controls selective disabling of the scheduler.
	//
	// Use schedEnableUser to control this.
	//
	// disable is protected by sched.lock.
	disable struct {
		// user disables scheduling of user goroutines.
		user     bool
		runnable gQueue // pending runnable Gs
		n        int32  // length of runnable
	}

	// Global cache of dead G's.
	// 有效 dead G 的全局缓存。
	gFree struct {
		lock    mutex
		stack   gList // Gs with stacks		// 包含栈的Gs
		noStack gList // Gs without stacks	// 没有栈的Gs
		n       int32
	}

	// Central cache of sudog structs.
	// sudog 结构中的集中缓存
	sudoglock  mutex
	sudogcache *sudog

	// Central pool of available defer structs of different sizes.
	// 不同大小的有效 defer 结构的池
	deferlock mutex
	deferpool [5]*_defer

	// freem is the list of m's waiting to be freed when their
	// m.exited is set. Linked through m.freelink.
	freem *m

	/*
		gcwaiting标志

		startTheWorld的时候，会将gcwaiting设为0，表示表示gc正在等待运行？？？
		stopTheWorld的时候，会将gcwaiting设为1，表示gc正在运行？？？
	*/
	gcwaiting uint32 // gc is waiting to run

	stopwait   int32
	stopnote   note
	sysmonwait uint32
	sysmonnote note

	// While true, sysmon not ready for mFixup calls.
	// Accessed atomically.
	sysmonStarting uint32

	// safepointFn should be called on each P at the next GC
	// safepoint if p.runSafePointFn is set.
	safePointFn   func(*p)
	safePointWait int32
	safePointNote note

	profilehz int32 // cpu profiling rate

	/*
		上次修改 gomaxprocs 的纳秒时间
	*/
	procresizetime int64 // nanotime() of last change to gomaxprocs
	totaltime      int64 // ∫gomaxprocs dt up to procresizetime

	// sysmonlock protects sysmon's actions on the runtime.
	//
	// Acquire and hold this mutex to block sysmon from interacting
	// with the rest of the runtime.
	sysmonlock mutex

	_ uint32 // ensure timeToRun has 8-byte alignment

	// timeToRun is a distribution of scheduling latencies, defined
	// as the sum of time a G spends in the _Grunnable state before
	// it transitions to _Grunning.
	// timeToRun是一个调度延迟的分布，定义为一个G在过渡到_Grunnable状态之前在
	// _Grunning状态下花费的时间总和。
	//
	// timeToRun is protected by sched.lock.
	// timeToRun受到sched.lock的保护。
	timeToRun timeHistogram
}

// Values for the flags field of a sigTabT.
// sigTabT的flags字段的值。
const (
	_SigNotify   = 1 << iota // let signal.Notify have signal, even if from kernel	// 让signal.Notify有信号，即使来自内核
	_SigKill                 // if signal.Notify doesn't take it, exit quietly		// 如果signal.Notify不接受，请安静地退出
	_SigThrow                // if signal.Notify doesn't take it, exit loudly
	_SigPanic                // if the signal is from the kernel, panic
	_SigDefault              // if the signal isn't explicitly requested, don't monitor it
	_SigGoExit               // cause all runtime procs to exit (only used on Plan 9).
	_SigSetStack             // add SA_ONSTACK to libc handler
	_SigUnblock              // always unblock; see blockableSig
	_SigIgn                  // _SIG_DFL action is to ignore the signal
)

// Layout of in-memory per-function information prepared by linker
// See https://golang.org/s/go12symtab.
// Keep in sync with linker (../cmd/link/internal/ld/pcln.go:/pclntab)
// and with package debug/gosym and with symtab.go in package runtime.
// 由链接器准备的内存中每个函数信息的布局
// 见 https://golang.org/s/go12symtab。
// 与链接器保持同步（.../cmd/link/internal/ld/pcln.go:/pclntab）。
// 与软件包 debug/gosym 和软件包 runtime 中的 symtab.go 保持同步。
type _func struct {
	entry   uintptr // start pc
	nameoff int32   // function name

	args        int32  // in/out args size
	deferreturn uint32 // offset of start of a deferreturn call instruction from entry, if any.

	pcsp      uint32
	pcfile    uint32
	pcln      uint32
	npcdata   uint32
	cuOffset  uint32 // runtime.cutab offset of this function's CU
	funcID    funcID // set for certain special runtime functions
	flag      funcFlag
	_         [1]byte // pad
	nfuncdata uint8   // must be last, must end on a uint32-aligned boundary
}

// Pseudo-Func that is returned for PCs that occur in inlined code.
// A *Func can be either a *_func or a *funcinl, and they are distinguished
// by the first uintptr.
type funcinl struct {
	zero  uintptr // set to 0 to distinguish from _func
	entry uintptr // entry of the real (the "outermost") frame.
	name  string
	file  string
	line  int
}

/*
每个itab都占用32字节的空间
*/
// layout of Itab known to compilers
// allocated in non-garbage-collected memory
// Needs to be in sync with
// ../cmd/compile/internal/gc/reflect.go:/^func.WriteTabs.
type itab struct {
	inter *interfacetype // 接口自身的元信息
	_type *_type         // 具体类型的元信息
	hash  uint32         // copy of _type.hash. Used for type switches.	// _type.hash的副本。用于类型转换（也就是断言）。
	_     [4]byte
	// 函数指针，指向具体类型所实现的方法
	fun [1]uintptr // variable sized. fun[0]==0 means _type does not implement inter.
}

// Lock-free stack node.
// Also known to export_test.go.
type lfnode struct {
	next    uint64
	pushcnt uintptr
}

type forcegcstate struct {
	lock mutex
	g    *g
	idle uint32
}

// extendRandom extends the random numbers in r[:n] to the whole slice r.
// Treats n<0 as n==0.
func extendRandom(r []byte, n int) {
	if n < 0 {
		n = 0
	}
	for n < len(r) {
		// Extend random bits using hash function & time seed
		w := n
		if w > 16 {
			w = 16
		}
		h := memhash(unsafe.Pointer(&r[n-w]), uintptr(nanotime()), uintptr(w))
		for i := 0; i < sys.PtrSize && n < len(r); i++ {
			r[n] = byte(h)
			n++
			h >>= 8
		}
	}
}

// A _defer holds an entry on the list of deferred calls.
// If you add a field here, add code to clear it in freedefer and deferProcStack
// This struct must match the code in cmd/compile/internal/gc/reflect.go:deferstruct
// and cmd/compile/internal/gc/ssa.go:(*state).call.
// Some defers will be allocated on the stack and some on the heap.
// All defers are logically part of the stack, so write barriers to
// initialize them are not required. All defers must be manually scanned,
// and for heap defers, marked.
type _defer struct {
	// 参数的大小（所有传入参数和返回值的总大学）
	siz int32 // includes both arguments and results
	// defer是否执行过了
	started bool
	// 是否在堆上分配（也就是是否调用deferprocStack进行分配），go1.13之后新增的
	heap bool
	// openDefer indicates that this _defer is for a frame with open-coded
	// defers. We have only one defer record for the entire frame (which may
	// currently have 0, 1, or more defers active).
	openDefer bool
	// 函数栈指针寄存器，一般指向当前函数栈的栈顶
	sp uintptr // sp at time of defer
	// 程序计数器，指向下一条需要执行的指令
	pc uintptr // pc at time of defer
	// 指向传入的函数地址和参数
	fn *funcval // can be nil for open-coded defers
	// defer中的panic
	// 指向 panic 链表
	_panic *_panic // panic that is running defer
	// defer链表，函数执行流程中的defer，会通过 link 这个属性进行串联
	// 指向 defer 链表
	link *_defer

	// If openDefer is true, the fields below record values about the stack
	// frame and associated function that has the open-coded defer(s). sp
	// above will be the sp for the frame, and pc will be address of the
	// deferreturn call in the function.
	fd   unsafe.Pointer // funcdata for the function associated with the frame
	varp uintptr        // value of varp for the stack frame
	// framepc is the current pc associated with the stack frame. Together,
	// with sp above (which is the sp associated with the stack frame),
	// framepc/sp can be used as pc/sp pair to continue a stack trace via
	// gentraceback().
	framepc uintptr
}

// A _panic holds information about an active panic.
//
// A _panic value must only ever live on the stack.
//
// The argp and link fields are stack pointers, but don't need special
// handling during stack growth: because they are pointer-typed and
// _panic values only live on the stack, regular stack pointer
// adjustment takes care of them.
/*
在panic中使用 _panic 作为其基础单元，每执行一次 panic 语句，都会创建一个 _panic 对象。
它包含了一些基础的字段用于存储当前的panic调用情况，涉及的字段如下：
1. argp：指向 defer 延迟调用的参数的指针
2. arg：panic的原因，也就是调用 panic 时传入的参数
3. link：指向上一个调用的 _panic，这里说明panci也是一个链表
4. recovered：panic是否已经被处理过，也就是是否被recover接收掉了
5. aborted：panic是否被终止
*/
type _panic struct {
	argp      unsafe.Pointer // pointer to arguments of deferred call run during panic; cannot move - known to liblink
	arg       interface{}    // argument to panic
	link      *_panic        // link to earlier panic
	pc        uintptr        // where to return to in runtime if this panic is bypassed
	sp        unsafe.Pointer // where to return to in runtime if this panic is bypassed
	recovered bool           // whether this panic is over
	aborted   bool           // the panic was aborted
	goexit    bool
}

// stack traces
// 栈追踪
type stkframe struct {
	fn       funcInfo   // function being run
	pc       uintptr    // program counter within fn
	continpc uintptr    // program counter where execution can continue, or 0 if not
	lr       uintptr    // program counter at caller aka link register
	sp       uintptr    // stack pointer at pc
	fp       uintptr    // stack pointer at caller aka frame pointer
	varp     uintptr    // top of local variables
	argp     uintptr    // pointer to function arguments
	arglen   uintptr    // number of bytes at argp
	argmap   *bitvector // force use of this argmap
}

// ancestorInfo records details of where a goroutine was started.
type ancestorInfo struct {
	pcs  []uintptr // pcs from the stack of this goroutine
	goid int64     // goroutine id of this goroutine; original goroutine possibly dead
	gopc uintptr   // pc of go statement that created this goroutine
}

const (
	_TraceRuntimeFrames = 1 << iota // include frames for internal runtime functions.
	_TraceTrap                      // the initial PC, SP are from a trap, not a return PC from a call
	_TraceJumpStack                 // if traceback is on a systemstack, resume trace at g that called into it
)

// The maximum number of frames we print for a traceback
const _TracebackMaxFrames = 100

// A waitReason explains why a goroutine has been stopped.
// See gopark. Do not re-use waitReasons, add new ones.
// 一个waitReason解释一个goroutine被阻塞的原因。
// 详情见gopark。不要重复使用waitReasons，而是添加新的。
type waitReason uint8

const (
	waitReasonZero                  waitReason = iota // ""
	waitReasonGCAssistMarking                         // "GC assist marking"		// GC协助标记
	waitReasonIOWait                                  // "IO wait"					// IO等待
	waitReasonChanReceiveNilChan                      // "chan receive (nil chan)"	// chan读阻塞
	waitReasonChanSendNilChan                         // "chan send (nil chan)"		// chan写阻塞
	waitReasonDumpingHeap                             // "dumping heap"
	waitReasonGarbageCollection                       // "garbage collection"
	waitReasonGarbageCollectionScan                   // "garbage collection scan"
	waitReasonPanicWait                               // "panicwait"
	waitReasonSelect                                  // "select"
	waitReasonSelectNoCases                           // "select (no cases)"
	waitReasonGCAssistWait                            // "GC assist wait"
	waitReasonGCSweepWait                             // "GC sweep wait"
	waitReasonGCScavengeWait                          // "GC scavenge wait"
	waitReasonChanReceive                             // "chan receive"
	waitReasonChanSend                                // "chan send"
	waitReasonFinalizerWait                           // "finalizer wait"
	waitReasonForceGCIdle                             // "force gc (idle)"
	waitReasonSemacquire                              // "semacquire"
	waitReasonSleep                                   // "sleep"					// 睡眠
	waitReasonSyncCondWait                            // "sync.Cond.Wait"
	waitReasonTimerGoroutineIdle                      // "timer goroutine (idle)"
	waitReasonTraceReaderBlocked                      // "trace reader (blocked)"
	waitReasonWaitForGCCycle                          // "wait for GC cycle"
	waitReasonGCWorkerIdle                            // "GC worker (idle)"
	waitReasonPreempted                               // "preempted"
	waitReasonDebugCall                               // "debug call"
)

var waitReasonStrings = [...]string{
	waitReasonZero:                  "",
	waitReasonGCAssistMarking:       "GC assist marking",
	waitReasonIOWait:                "IO wait",
	waitReasonChanReceiveNilChan:    "chan receive (nil chan)",
	waitReasonChanSendNilChan:       "chan send (nil chan)",
	waitReasonDumpingHeap:           "dumping heap",
	waitReasonGarbageCollection:     "garbage collection",
	waitReasonGarbageCollectionScan: "garbage collection scan",
	waitReasonPanicWait:             "panicwait",
	waitReasonSelect:                "select",
	waitReasonSelectNoCases:         "select (no cases)",
	waitReasonGCAssistWait:          "GC assist wait",
	waitReasonGCSweepWait:           "GC sweep wait",
	waitReasonGCScavengeWait:        "GC scavenge wait",
	waitReasonChanReceive:           "chan receive",
	waitReasonChanSend:              "chan send",
	waitReasonFinalizerWait:         "finalizer wait",
	waitReasonForceGCIdle:           "force gc (idle)",
	waitReasonSemacquire:            "semacquire",
	waitReasonSleep:                 "sleep",
	waitReasonSyncCondWait:          "sync.Cond.Wait",
	waitReasonTimerGoroutineIdle:    "timer goroutine (idle)",
	waitReasonTraceReaderBlocked:    "trace reader (blocked)",
	waitReasonWaitForGCCycle:        "wait for GC cycle",
	waitReasonGCWorkerIdle:          "GC worker (idle)",
	waitReasonPreempted:             "preempted",
	waitReasonDebugCall:             "debug call",
}

func (w waitReason) String() string {
	if w < 0 || w >= waitReason(len(waitReasonStrings)) {
		return "unknown wait reason"
	}
	return waitReasonStrings[w]
}

var (
	allm       *m
	gomaxprocs int32 // p的最大值，默认等于ncpu，但可以通过GOMAXPROCS修改
	ncpu       int32 // 系统中cpu核的数量，程序启动时由runtime代码初始化
	forcegc    forcegcstate
	sched      schedt // 调度器结构体对象，记录了调度器的工作状态
	newprocs   int32

	// allpLock protects P-less reads and size changes of allp, idlepMask,
	// and timerpMask, and all writes to allp.
	allpLock mutex
	// len(allp) == gomaxprocs; may change at safe points, otherwise
	// immutable.
	allp []*p // 保存所有的p，len(allp) == gomaxprocs
	// Bitmask of Ps in _Pidle list, one bit per P. Reads and writes must
	// be atomic. Length may change at safe points.
	//
	// Each P must update only its own bit. In order to maintain
	// consistency, a P going idle must the idle mask simultaneously with
	// updates to the idle P list under the sched.lock, otherwise a racing
	// pidleget may clear the mask before pidleput sets the mask,
	// corrupting the bitmap.
	//
	// N.B., procresize takes ownership of all Ps in stopTheWorldWithSema.
	idlepMask pMask
	// Bitmask of Ps that may have a timer, one bit per P. Reads and writes
	// must be atomic. Length may change at safe points.
	timerpMask pMask

	// Pool of GC parked background workers. Entries are type
	// *gcBgMarkWorkerNode.
	gcBgMarkWorkerPool lfstack

	// Total number of gcBgMarkWorker goroutines. Protected by worldsema.
	gcBgMarkWorkerCount int32

	// Information about what cpu features are available.
	// Packages outside the runtime should not use these
	// as they are not an external api.
	// Set on startup in asm_{386,amd64}.s
	processorVersionInfo uint32
	isIntel              bool
	lfenceBeforeRdtsc    bool

	goarm uint8 // set by cmd/link on arm systems
)

// Set by the linker so the runtime can determine the buildmode.
// 由链接器设置，以便运行时能够确定构建模式。
var (
	islibrary bool // -buildmode=c-shared
	isarchive bool // -buildmode=c-archive
)

// Must agree with internal/buildcfg.Experiment.FramePointer.
// 必须与内部/buildcfg.Experiment.FramePointer一致。
const framepointer_enabled = GOARCH == "amd64" || GOARCH == "arm64"
