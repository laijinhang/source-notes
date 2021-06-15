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

M的最大数量限制在10000
```go
// src/runtime/proc.go
func schedinit() {
	...
    // M（线程）最大数量限制
    sched.maxmcount = 10000
    ...
}
```
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
# 4、结束