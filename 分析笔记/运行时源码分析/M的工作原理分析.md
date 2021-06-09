# 1、数据结构
```go
// runtime/runtime2.go
/*
M的结构，M是OS线程的实体
*/
type m struct {
	// 持有调度栈的 Goroutine
	g0      *g     // 用于执行调度指令的 Goroutine
	morebuf gobuf  
	divmod  uint32 

	procid     uint64       
	gsignal    *g          
	goSigStack gsignalStack
	sigmask    sigset       
	tls      [tlsSlots]uintptr // 线程本地存储
	mstartfn func()
	// 当前运行的G
	curg      *g       // 当前运行的用户 Goroutine
	caughtsig guintptr 
	// 正在运行代码的P
	p     puintptr // 执行go代码时持有的P（如果没有执行则为nil）
	nextp puintptr
	// 之前使用的P
	oldp          puintptr 
	id            int64
	mallocing     int32
	throwing      int32
	preemptoff    string 
	locks         int32
	dying         int32
	profilehz     int32
	spinning      bool // m当前没有运行的work且正处于work的活跃状态
	blocked       bool 
	newSigstack   bool 
	printlock     int8
	incgo         bool  
	freeWait      uint32
	fastrand      [2]uint32
	needextram    bool
	traceback     uint8
	ncgocall      uint64      
	ncgo          int32       
	cgoCallersUse uint32      // cgo调用崩溃的cgo回溯
	cgoCallers    *cgoCallers 
	doesPark      bool      
	park          note
	alllink       *m 
	schedlink     muintptr
	lockedg       guintptr
	createstack   [32]uintptr 
	lockedExt     uint32      
	lockedInt     uint32      
	nextwaitm     muintptr   
	waitunlockf   func(*g, unsafe.Pointer) bool
	waitlock      unsafe.Pointer
	waittraceev   byte
	waittraceskip int
	startingtrace bool
	syscalltick   uint32
	freelink      *m 

	mFixup struct {
		lock mutex
		used uint32
		fn   func(bool) bool
	}

	libcall   libcall
	libcallpc uintptr 
	libcallsp uintptr
	libcallg  guintptr
	syscall   libcall 

	vdsoSP uintptr 
	vdsoPC uintptr

	preemptGen uint32

	signalPending uint32

	dlogPerM

	mOS

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

# 3、M的状态转换
# 4、工作过程
# 5、结束