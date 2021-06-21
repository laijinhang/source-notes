# 一、结构
### 1、数据结构
**g的现场：**
```go
// g 的运行现场
type gobuf struct {
    sp   uintptr    // sp 寄存器
    pc   uintptr    // pc 寄存器
    g    guintptr   // g 指针
    ctxt unsafe.Pointer // 这个似乎是用来辅助 gc 的
    ret  sys.Uintreg
    lr   uintptr    // 这是在 arm 上用的寄存器，不用关心
    bp   uintptr    // 开启 GOEXPERIMENT=framepointer，才会有这个
}
```
**g的结构：**
```go
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

	_panic *_panic // innermost panic - offset known to liblink
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
	param        unsafe.Pointer
	atomicstatus uint32
	stackLock    uint32 // sigprof/scang lock; TODO: fold in to atomicstatus
	goid         int64
	schedlink    guintptr
	waitsince    int64      // approx time when the g become blocked	// g被阻塞的大约时间
	waitreason   waitReason // if status==Gwaiting

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
	lockedm        muintptr
	sig            uint32
	writebuf       []byte
	sigcode0       uintptr
	sigcode1       uintptr
	sigpc          uintptr
	gopc           uintptr         // pc of go statement that created this goroutine
	ancestors      *[]ancestorInfo // ancestor information goroutine(s) that created this goroutine (only used if debug.tracebackancestors)
	startpc        uintptr         // pc of goroutine function
	racectx        uintptr
	waiting        *sudog         // sudog structures this g is waiting on (that have a valid elem ptr); in lock order
	cgoCtxt        []uintptr      // cgo traceback context
	labels         unsafe.Pointer // profiler labels
	timer          *timer         // cached timer for time.Sleep
	selectDone     uint32         // are we participating in a select and did someone win the race?

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
```

### 2、状态
**Goroutine的状态：**
* _Gidle：0，刚刚被分配并且还没有被初始化
* _Grunnable：1，没有执行代码，没有栈的所有权，存储在运行队列中
* _Grunning：2，可以执行代码，拥有栈的所有权，被赋予了内核线程 M 和处理器 P
* _Gsyscall：3，正在执行系统调用，拥有栈的所有权，没有执行用户代码，被赋予了内核线程 M 但是不在运行队列上
* _Gwaiting：4，由于运行时而被阻塞，没有执行用户代码并且不在运行队列上，但是可能存在于 Channel 的等待队列上
* _Gdead：5，没有被使用，没有执行代码，可能有分配的栈
* _Gcopystack：6，栈正在被拷贝，没有执行代码，不在运行队列上
* _Gpreempted：7，由于抢占而被阻塞，没有执行用户代码并且不在运行队列上，等待唤醒
* _Gscan：0x1000，GC 正在扫描栈空间，没有执行代码，可以与其他状态同时存在
* _Gscanrunnable  = _Gscan + _Grunnable  // 0x1001
* _Gscanrunning   = _Gscan + _Grunning   // 0x1002
* _Gscansyscall   = _Gscan + _Gsyscall   // 0x1003
* _Gscanwaiting   = _Gscan + _Gwaiting   // 0x1004
* _Gscanpreempted = _Gscan + _Gpreempted // 0x1009
### 3、种类
* 主协程，g0：g0是每次启动一个m都会第一个创建的goroutine，g0仅用于负责调度g，g0不指向任何可执行的函数，
  每个m都会有一个自己的g0。在调度或系统调用时会使用g0的栈空间，全局变量的g0是m0的g0。
* 用于进行gc的协程
* 用于帮助标记内存专用后台的协程：runtime.gcBgMarkWorker
* 用于管理finalizer的协程
* 普通协程
# 2、工作原理
1. 通过runtime.newproc函数创建协程
2. 如果P的本地队列没有满，则放入本地队列，否则放入全局队列中
3. 如果在本地/全局队列，等待被调度，偷取
4. 被M调度，执行
5. 阻塞、休眠或销毁

### 1. 创建
runtime/proc.go
```go
func newproc(siz int32, fn *funcval) {
	// 从fn的地址增加一个指针的长度，从而获取第一个参数地址
	argp := add(unsafe.Pointer(&fn), sys.PtrSize)
	// 获取当前的 G
	gp := getg()
	pc := getcallerpc() // 获取调用方 PC/IP 寄存器
	// 用 g0 系统栈创建 Goroutine
	// 传递的参数包括fn函数入口地址，argp参数起始地址，size参数长度，gp（g0），调用方pc（goroutine）
	systemstack(func() {
		// 获取新的 G 结构体
		newg := newproc1(fn, argp, siz, gp, pc)

		_p_ := getg().m.p.ptr()
		// 将 G 加入到 P 的运行队列
		runqput(_p_, newg, true)
		// mainStarted为 True 表示主M已经启动

		if mainStarted {
			// 唤醒新的 P 执行 G
			wakep()
		}
	})
}
```


1. 什么时候创建？
执行`go 函数()`代码
2. 创建的时候做了什么？
   1. 从当前协程所在p上尝试获取一个空闲g，如果没有获取到，则创建一个栈大小为2k的新协程，
      通过cas去把这个新协程的状态从_Gidle改为_Gdead，之后将这个新协程加入到全局allgs列表
   2. 通过cas去把这个新协程的状态从_Gdead改为_Grunnable
   3. 为这个新协程分配一个id（这个id是全局唯一）
   4. 将G加入到P的运行队列
   5. 唤醒P执行G

### 2. 初始化过程
### 3. G的切换
runtime/proc.go
```go
/*
gopack用于协程的切换，协程切换的原因一般有以下几种情况：
1. 系统调用
2. channel读写条件不满足
3. 抢占式调度时间片结束
gopack函数做的主要事情分为两点：
1. 解除当前goroutine与m的绑定关闭，将当前goroutine状态机切换为等待状态；
2. 调用一次schedule()函数，在局部调度器P发起一轮新的调度。
*/
func gopark(unlockf func(*g, unsafe.Pointer) bool, lock unsafe.Pointer, reason waitReason, traceEv byte, traceskip int) {
	if reason != waitReasonSleep {
		checkTimeouts() // timeouts may expire while two goroutines keep the scheduler busy
	}
	mp := acquirem()
	gp := mp.curg
	status := readgstatus(gp)

	//println("m.id：", mp.id, "，当前协程编号：", mp.curg.goid, "，协程当前状态：", status)
	//{
	//	curp := mp.p
	//	println("当前p：", curp.ptr().id, "，p的长度：", curp.ptr().runqtail - curp.ptr().runqhead)
	//	for i := curp.ptr().runqhead; i < curp.ptr().runqtail; i++ {
	//		println("p：", curp.ptr().id, "，gid：", curp.ptr().runq[i].ptr().goid)
	//	}
	//}
	//println("----------------------------------------------------------")
	//println("")

	if status != _Grunning && status != _Gscanrunning {
		throw("gopark: bad g status")
	}
	mp.waitlock = lock
	mp.waitunlockf = unlockf
	gp.waitreason = reason
	mp.waittraceev = traceEv
	mp.waittraceskip = traceskip
	releasem(mp)
	// can't do anything that might move the G between Ms here.
	/*
		协程切换工作：
		1. 切换当前线程的堆栈从g的堆栈切换到g0的堆栈；
		2. 并在g0的堆栈上执行新的函数fn(g)；
		3. 保存当前协程的信息（PC/SP存储到g->sched)，当后续对当前协程调用Goready函数时候能够恢复现场；
		mcall函数是通过汇编实现的，64位机的实现代码在 asm_amd64.s
		它将当前正在执行的协程状态保存起来，然后在m->g0的堆栈上调用新的函数。在新的函数内会将之前运行的协程放弃，
		然后调用一次schedule()来挑选新的协程运行（也就是在传入的函数中调用一次schedule()函数进行一次schedule的重新调度，
		让m去运行其余的goroutine）。
	*/
	mcall(park_m)
}
```
### 4. G的结束
# 5、main的协程
### 1. main协程的创建
```go
runtime/asm_amd64.s

CALL	runtime·args(SB)        // 初始化执行文件的绝对路径
CALL	runtime·osinit(SB)      // 初始化 CPU 个数和内存页大小
CALL	runtime·schedinit(SB)   // 调度器初始化

// create a new goroutine to start program
// 创建一个新的 goroutine 来启动程序
MOVQ	$runtime·mainPC(SB), AX		// entry
PUSHQ	AX
PUSHQ	$0			// arg size
// 新建一个 goroutine，该 goroutine 绑定 runtime.main
CALL	runtime·newproc(SB)
POPQ	AX
POPQ	AX

// start this M
// 启动M，开始调度 goroutine
CALL	runtime·mstart(SB)
```
# 六、g状态切换的场景