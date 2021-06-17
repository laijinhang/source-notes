# 1、数据结构
```go
// runtime/runtime2.go
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
    oldp puintptr // the p that was attached before executing a syscall
    id   int64
    /*
        mallocing不等于0，表示正在执行分配任务
    */
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
```



```go
// src/runtime/proc.go
func schedinit() {
	...
    // M（线程）最大数量限制
    sched.maxmcount = 10000
    ...
}
```
**M数量：**
1. M的最大数量限制在10000
2. runtime/debug包中的SetMaxThreads函数来设置
3. 有⼀个M阻塞，会创建⼀个新的M
4. 如果有M空闲，那么就会回收或者睡眠
# 2、初始化
```go
// src/runtime/proc.go
func schedinit() {
    ...
    // M（线程）最大数量限制
    sched.maxmcount = 10000
	...
	// M 初始化
	mcommoninit(_g_.m, -1)
    ...
}
```
# 3、工作过程
### 1. 创建一个m
```go
func newm(fn func(), _p_ *p, id int64) {
	mp := allocm(_p_, fn, id)
	mp.doesPark = (_p_ != nil)
	mp.nextp.set(_p_)
	mp.sigmask = initSigmask
	if gp := getg(); gp != nil && gp.m != nil && (gp.m.lockedExt != 0 || gp.m.incgo) && GOOS != "plan9" {
		// We're on a locked M or a thread that may have been
		// started by C. The kernel state of this thread may
		// be strange (the user may have locked it for that
		// purpose). We don't want to clone that into another
		// thread. Instead, ask a known-good thread to create
		// the thread for us.
		// 我们在一个被锁定的M或者一个可能由C启动的线程上，这个线程的内核
		// 状态可能很奇怪（用户可能为此而锁定它）。我们不想把它克隆到另一
		// 个线程中。相反，要求一个已知的好的线程为我们创建线程。
		//
		// This is disabled on Plan 9. See golang.org/issue/22227.
		// 这在Plan 9上是禁用的。参见 golang.org/issue/22227。
		//
		// TODO: This may be unnecessary on Windows, which
		// doesn't model thread creation off fork.
		lock(&newmHandoff.lock)
		if newmHandoff.haveTemplateThread == 0 {
			throw("on a locked thread with no template thread")
		}
		mp.schedlink = newmHandoff.newm
		newmHandoff.newm.set(mp)
		if newmHandoff.waiting {
			newmHandoff.waiting = false
			notewakeup(&newmHandoff.wake)
		}
		unlock(&newmHandoff.lock)
		return
	}
	newm1(mp)
}

func newm1(mp *m) {
	if iscgo {
		var ts cgothreadstart
		if _cgo_thread_start == nil {
			throw("_cgo_thread_start missing")
		}
		ts.g.set(mp.g0)
		ts.tls = (*uint64)(unsafe.Pointer(&mp.tls[0]))
		ts.fn = unsafe.Pointer(funcPC(mstart))
		if msanenabled {
			msanwrite(unsafe.Pointer(&ts), unsafe.Sizeof(ts))
		}
		execLock.rlock() // Prevent process clone.
		asmcgocall(_cgo_thread_start, unsafe.Pointer(&ts))
		execLock.runlock()
		return
	}
	execLock.rlock() // Prevent process clone.
	newosproc(mp)
	execLock.runlock()
}
```
### 2. 启动m
```go
func startm(_p_ *p, spinning bool) {
	// Disable preemption.
	//
	// Every owned P must have an owner that will eventually stop it in the
	// event of a GC stop request. startm takes transient ownership of a P
	// (either from argument or pidleget below) and transfers ownership to
	// a started M, which will be responsible for performing the stop.
	//
	// Preemption must be disabled during this transient ownership,
	// otherwise the P this is running on may enter GC stop while still
	// holding the transient P, leaving that P in limbo and deadlocking the
	// STW.
	//
	// Callers passing a non-nil P must already be in non-preemptible
	// context, otherwise such preemption could occur on function entry to
	// startm. Callers passing a nil P may be preemptible, so we must
	// disable preemption before acquiring a P from pidleget below.
	mp := acquirem()
	lock(&sched.lock)
	if _p_ == nil {
		_p_ = pidleget()
		if _p_ == nil {
			unlock(&sched.lock)
			if spinning {
				// The caller incremented nmspinning, but there are no idle Ps,
				// so it's okay to just undo the increment and give up.
				if int32(atomic.Xadd(&sched.nmspinning, -1)) < 0 {
					throw("startm: negative nmspinning")
				}
			}
			releasem(mp)
			return
		}
	}
	nmp := mget()
	if nmp == nil {
		// No M is available, we must drop sched.lock and call newm.
		// However, we already own a P to assign to the M.
		//
		// Once sched.lock is released, another G (e.g., in a syscall),
		// could find no idle P while checkdead finds a runnable G but
		// no running M's because this new M hasn't started yet, thus
		// throwing in an apparent deadlock.
		//
		// Avoid this situation by pre-allocating the ID for the new M,
		// thus marking it as 'running' before we drop sched.lock. This
		// new M will eventually run the scheduler to execute any
		// queued G's.
		id := mReserveID()
		unlock(&sched.lock)

		var fn func()
		if spinning {
			// The caller incremented nmspinning, so set m.spinning in the new M.
			fn = mspinning
		}
		newm(fn, _p_, id)
		// Ownership transfer of _p_ committed by start in newm.
		// Preemption is now safe.
		releasem(mp)
		return
	}
	unlock(&sched.lock)
	if nmp.spinning {
		throw("startm: m is spinning")
	}
	if nmp.nextp != 0 {
		throw("startm: m has p")
	}
	if spinning && !runqempty(_p_) {
		throw("startm: p has runnable gs")
	}
	// The caller incremented nmspinning, so set m.spinning in the new M.
	nmp.spinning = spinning
	nmp.nextp.set(_p_)
	notewakeup(&nmp.park)
	// Ownership transfer of _p_ committed by wakeup. Preemption is now
	// safe.
	releasem(mp)
}
```
### 3. 停止m
```go
func stopm() {
	_g_ := getg()

	if _g_.m.locks != 0 {
		throw("stopm holding locks")
	}
	if _g_.m.p != 0 {
		throw("stopm holding p")
	}
	if _g_.m.spinning {
		throw("stopm spinning")
	}

	// 将 m 返回到 空闲列表中，因为马上就要暂停了
	lock(&sched.lock)
	mput(_g_.m)
	unlock(&sched.lock)
	mPark()
	// 此时已经被复始，说明有任务要执行
	// 立即 acquire P
	acquirep(_g_.m.nextp.ptr())
	_g_.m.nextp = 0
}
```
# 4、结束