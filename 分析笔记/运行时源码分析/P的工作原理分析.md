# 1、结构
### 1. 数据结构
```go
/*
P的结构
P只是处理器的抽象，而被处理器本身，它存在的意义在于实现工作窃取（work stealing）算法。简单来说，每个P持有一个G的本地队列。
在没有P的情况下，所以G只能放在一个全局队列中，当M执行完而没有G可执行时，虽然仍然会先检查全局队列、网络，
但这时增加了一个从其他P的队列偷取（steal）一个G来执行的过程。优先级为本地 > 全局 > 网络 > 偷取。
偷取：当有若个P时，其中有P当前维护的本地G队列为空时，而全局队列中存在G，那么本地队列为空的P会去全局队列中偷取。
*/
type p struct {
	id          int32
	status      uint32 // one of pidle/prunning/...	// p的状态 pidle/prunning/...
	link        puintptr
	schedtick   uint32     // incremented on every scheduler call	// 每次调度程序调用时递增
	syscalltick uint32     // incremented on every system call		// 每次系统调用时递增
	sysmontick  sysmontick // last tick observed by sysmon			// sysmon观察到的最后一个刻度
	m           muintptr   // back-link to associated m (nil if idle)	// 反向链接到关联的m（nil则表示idle）
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
	preempt bool

	// Padding is no longer needed. False sharing is now not a worry because p is large enough
	// that its size class is an integer multiple of the cache line size (for any of our architectures).
}
```
### 2. 状态
### 3. 本地队列
```go
type p struct {
    ...
	// 可运行的 Goroutine 队列，可无锁访问
	/*
		运行队列的头指针
	*/
	runqhead uint32
	/*
		运行队列的尾指针
	*/
	runqtail uint32
	/*
		运行队列
	*/
	runq [256]guintptr
	/*
		下一个要执行的协程
	*/
	runnext guintptr
	...
}
```
### 4. 全局队列
```go
type schedt struct {
    ...
	// 全局 runnable G 队列
	runq     gQueue
	runqsize int32
    ...
}
```
# 2、本地队列的维护
1. func runqget(_p_ *p) (gp *g, inheritTime bool)
2. func runqput(_p_ *p, gp *g, next bool)
3. func runqempty(_p_ *p) bool
4. func runqputslow(_p_ *p, gp *g, h, t uint32) bool
5. func runqputbatch(pp *p, q *gQueue, qsize int)
6. func runqdrain(_p_ *p) (drainQ gQueue, n uint32)
7. func runqgrab(_p_ *p, batch *[256]guintptr, batchHead uint32, stealRunNextG bool) uint32
8. func runqsteal(_p_, p2 *p, stealRunNextG bool) *g
### 1. runget
* 入参：p
* 作用：从传入的p中的本地队列中获取g
* 出参：获取到g（没获取到的话，则为nil），inheritTime（如果inheritTime为true，gp应该继承当前时间片的剩余时间。否则，它应该开始一个新的时间片。）

1. 尝试先从p.runnext获取，如果runnext不为空，则直接获取并返回
2. 如果runnext为空，则从本地队列头指针遍历本地队列
### 2. runput
* 入参：p，协程，next
* 作用：尝试将g放入到本地可运行队列中
* 出参：无

如果next为false，runqput将g添加到可运行队列的尾部
如果next为true，runqput将g放入_p_.runnext中
如果运行队列已满，runnext将g放到全局队列中。

### 3. func runqempty(_p_ *p) bool
* 入参：p
* 作用：判断_p_的本地运行队列中有没有g
* 出参：_p_的本地运行队列中有没有g

### 4. func runqputslow(_p_ *p, gp *g, h, t uint32) bool
* 入参：p，协程，p本地队列的头指针，p本地队列的尾指针
* 作用：把g和本地可运行队列中的一批协程放到全局队列中
* 出参：
### 5. func runqputbatch(pp *p, q *gQueue, qsize int)
### 6. func runqdrain(_p_ *p) (drainQ gQueue, n uint32)
### 7. func runqgrab(_p_ *p, batch *[256]guintptr, batchHead uint32, stealRunNextG bool) uint32
### 8. func runqsteal(_p_, p2 *p, stealRunNextG bool) *g