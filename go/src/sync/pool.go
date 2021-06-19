// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"internal/race"
	"runtime"
	"sync/atomic"
	"unsafe"
)

// A Pool is a set of temporary objects that may be individually saved and
// retrieved.
// 一个Pool是一组可以单独保存和检索的临时对象。
//
// Any item stored in the Pool may be removed automatically at any time without
// notification. If the Pool holds the only reference when this happens, the
// item might be deallocated.
// 存储在Pool中的任何项目都可能在任何时候被自动删除，而无需通知。
// 如果发生这种情况时，Pool持有唯一的引用，那么该项目可能会被取消分配。
//
// A Pool is safe for use by multiple goroutines simultaneously.
// 一个Pool可以被多个goroutine同时使用，是安全的。
//
// Pool's purpose is to cache allocated but unused items for later reuse,
// relieving pressure on the garbage collector. That is, it makes it easy to
// build efficient, thread-safe free lists. However, it is not suitable for all
// free lists.
// Pool的目的是缓存已分配但未使用的项目，以便以后再使用，减轻垃圾收集器的压力。
// 也就是说，它使建立高效、线程安全的空闲列表变得容易。然而，它并不适合于所有的空闲列表。
//
// An appropriate use of a Pool is to manage a group of temporary items
// silently shared among and potentially reused by concurrent independent
// clients of a package. Pool provides a way to amortize allocation overhead
// across many clients.
// Pool的一个合适的用途是管理一组临时项目，在一个包的独立客户端之间默默地共享，并有可能被重复使用。
// Pool提供了一种在许多客户端之间分摊分配开销的方法。
//
// An example of good use of a Pool is in the fmt package, which maintains a
// dynamically-sized store of temporary output buffers. The store scales under
// load (when many goroutines are actively printing) and shrinks when
// quiescent.
// 在fmt包中有一个很好的使用Pool的例子，它维护了一个动态大小的临时输出缓冲区的存储。
// 这个存储空间在负载下（当许多goroutines积极打印时）会扩大，在静止时则会缩小。
//
// On the other hand, a free list maintained as part of a short-lived object is
// not a suitable use for a Pool, since the overhead does not amortize well in
// that scenario. It is more efficient to have such objects implement their own
// free list.
// 另一方面，作为短生命周期对象的一部分而维护的空闲列表并不适合使用Pool，因为在这种情况下，开销不会被很好地摊销。
// 让这样的对象实现自己的空闲列表会更有效率。
//
// A Pool must not be copied after first use.
// 一个Pool在第一次使用后不能被复制。
type Pool struct {
	// 不复制Pool结构体，理解起来就是浅拷贝
	/*
		下面c、c1、c2指向的都是同一个Pool
		func main() {
			c := sync.Pool{}
			c1 := c
			c1.New = func() interface{} {
				return 123
			}
			c1.Put("x1")
			c1.Put("x2")
			c2 := sync.Pool{}
			c2 = c1
			fmt.Println(c2.Get())	// x1
			fmt.Println(c2.Get())	// x2
			fmt.Println(c1.Get())	// 123
			c2.Put("x3")
			fmt.Println(c1.Get())	// x3
		}
	*/
	noCopy noCopy

	local     unsafe.Pointer // local fixed-size per-P pool, actual type is [P]poolLocal	// 本地固定大小的per-P池，实际类型为[P]poolLocal
	localSize uintptr        // size of the local array										// 本地数组的大小

	/*
		GC的时候，先将local中每个处理器（P）对应的poolLocal赋给victim，
		然后清空local，所以victim就是缓存GC前的local
	*/
	victim     unsafe.Pointer // local from previous cycle	// 上一周期的局部
	victimSize uintptr        // size of victims array

	// New optionally specifies a function to generate
	// a value when Get would otherwise return nil.
	// It may not be changed concurrently with calls to Get.
	// New可以选择指定一个函数，在Get会返回nil的情况下生成一个值。
	// 它不能与对Get的调用同时发生变化。
	New func() interface{}
}

// Local per-P Pool appendix.
// 本地per-P池附录。
type poolLocalInternal struct {
	private interface{} // Can be used only by the respective P.			// 只能由各自的P使用。
	shared  poolChain   // Local P can pushHead/popHead; any P can popTail.	// 本地P可以从头部推入/头部推出；任何P都可以推尾。
}

type poolLocal struct {
	poolLocalInternal

	// Prevents false sharing on widespread platforms with
	// 128 mod (cache line size) = 0 .
	// 防止在广泛的平台上出现错误的共享，128 mod (缓存行大小) = 0 .
	pad [128 - unsafe.Sizeof(poolLocalInternal{})%128]byte
}

// from runtime
// 来自运行时
func fastrand() uint32

var poolRaceHash [128]uint64

// poolRaceAddr returns an address to use as the synchronization point
// for race detector logic. We don't use the actual pointer stored in x
// directly, for fear of conflicting with other synchronization on that address.
// Instead, we hash the pointer to get an index into poolRaceHash.
// See discussion on golang.org/cl/31589.
func poolRaceAddr(x interface{}) unsafe.Pointer {
	ptr := uintptr((*[2]unsafe.Pointer)(unsafe.Pointer(&x))[1])
	h := uint32((uint64(uint32(ptr)) * 0x85ebca6b) >> 16)
	return unsafe.Pointer(&poolRaceHash[h%uint32(len(poolRaceHash))])
}

// Put adds x to the pool.
// 将x添加到池中。
func (p *Pool) Put(x interface{}) {
	// 1、如果x为nil，则不加入池中
	if x == nil {
		return
	}
	// 2、资源检测
	if race.Enabled {
		if fastrand()%4 == 0 {
			// Randomly drop x on floor.
			return
		}
		race.ReleaseMerge(poolRaceAddr(x))
		race.Disable()
	}
	// 3、
	l, _ := p.pin()
	if l.private == nil {
		l.private = x
		x = nil
	}
	if x != nil {
		l.shared.pushHead(x)
	}
	runtime_procUnpin()
	if race.Enabled {
		race.Enable()
	}
}

// Get selects an arbitrary item from the Pool, removes it from the
// Pool, and returns it to the caller.
// Get may choose to ignore the pool and treat it as empty.
// Callers should not assume any relation between values passed to Put and
// the values returned by Get.
// Get从池中选择一个任意的项目，将其从池中移除，并将其返回给调用者。
// Get可以选择忽略池子并将其视为空的。调用者不应该假设传递给Put的值和Get返回的值之间有任何关系。
//
// If Get would otherwise return nil and p.New is non-nil, Get returns
// the result of calling p.New.
// 如果Get会返回nil，并且p.New不是nil，Get会返回调用p.New的结果。
func (p *Pool) Get() interface{} {
	if race.Enabled {
		race.Disable()
	}
	l, pid := p.pin()
	x := l.private
	l.private = nil
	if x == nil {
		// Try to pop the head of the local shard. We prefer
		// the head over the tail for temporal locality of
		// reuse.
		x, _ = l.shared.popHead()
		if x == nil {
			x = p.getSlow(pid)
		}
	}
	runtime_procUnpin()
	if race.Enabled {
		race.Enable()
		if x != nil {
			race.Acquire(poolRaceAddr(x))
		}
	}
	// 如果x为空，并且p.New方法不为空，则通过p.New创建一个
	if x == nil && p.New != nil {
		x = p.New()
	}
	return x
}

func (p *Pool) getSlow(pid int) interface{} {
	// See the comment in pin regarding ordering of the loads.
	size := runtime_LoadAcquintptr(&p.localSize) // load-acquire
	locals := p.local                            // load-consume
	// Try to steal one element from other procs.
	for i := 0; i < int(size); i++ {
		l := indexLocal(locals, (pid+i+1)%int(size))
		if x, _ := l.shared.popTail(); x != nil {
			return x
		}
	}

	// Try the victim cache. We do this after attempting to steal
	// from all primary caches because we want objects in the
	// victim cache to age out if at all possible.
	size = atomic.LoadUintptr(&p.victimSize)
	if uintptr(pid) >= size {
		return nil
	}
	locals = p.victim
	l := indexLocal(locals, pid)
	if x := l.private; x != nil {
		l.private = nil
		return x
	}
	for i := 0; i < int(size); i++ {
		l := indexLocal(locals, (pid+i)%int(size))
		if x, _ := l.shared.popTail(); x != nil {
			return x
		}
	}

	// Mark the victim cache as empty for future gets don't bother
	// with it.
	atomic.StoreUintptr(&p.victimSize, 0)

	return nil
}

// pin pins the current goroutine to P, disables preemption and
// returns poolLocal pool for the P and the P's id.
// Caller must call runtime_procUnpin() when done with the pool.
// pin将当前的goroutine引向P，禁止抢占，并返回P的poolLocal pool和P的id。
// 调用者在处理完池子后必须调用runtime_procUnpin()。
func (p *Pool) pin() (*poolLocal, int) {
	/*
		作用：禁止当前goroutine被抢占
		操作：当前goroutine绑定的m的锁加1
		返回：当前goroutine绑定的p的id
	*/
	pid := runtime_procPin()
	// In pinSlow we store to local and then to localSize, here we load in opposite order.
	// Since we've disabled preemption, GC cannot happen in between.
	// Thus here we must observe local at least as large localSize.
	// We can observe a newer/larger local, it is fine (we must observe its zero-initialized-ness).
	// 在pinSlow中，我们先存储到本地，然后再存储到localSize，这里我们以相反的顺序加载。
	// 因为我们已经禁用了抢占，所以GC不能在两者之间发生。
	// 因此在这里我们必须观察到至少和 localSize 一样大的 local。
	// 我们可以观察到一个更新/更大的局部，这很好（我们必须观察其零初始化）。
	s := runtime_LoadAcquintptr(&p.localSize) // load-acquire
	l := p.local                              // load-consume
	if uintptr(pid) < s {
		return indexLocal(l, pid), pid
	}
	return p.pinSlow()
}

func (p *Pool) pinSlow() (*poolLocal, int) {
	// Retry under the mutex.
	// Can not lock the mutex while pinned.
	runtime_procUnpin()
	allPoolsMu.Lock()
	defer allPoolsMu.Unlock()
	pid := runtime_procPin()
	// poolCleanup won't be called while we are pinned.
	s := p.localSize
	l := p.local
	if uintptr(pid) < s {
		return indexLocal(l, pid), pid
	}
	if p.local == nil {
		allPools = append(allPools, p)
	}
	// If GOMAXPROCS changes between GCs, we re-allocate the array and lose the old one.
	size := runtime.GOMAXPROCS(0)
	local := make([]poolLocal, size)
	atomic.StorePointer(&p.local, unsafe.Pointer(&local[0])) // store-release
	runtime_StoreReluintptr(&p.localSize, uintptr(size))     // store-release
	return &local[pid], pid
}

func poolCleanup() {
	// This function is called with the world stopped, at the beginning of a garbage collection.
	// It must not allocate and probably should not call any runtime functions.
	// 这个函数是 在世界停止的情况下 被调用的，在垃圾回收开始前。
	// 它不能分配，也不应该调用任何运行时函数。

	// Because the world is stopped, no pool user can be in a
	// pinned section (in effect, this has all Ps pinned).
	// 因为世界已经停止了，没有一个池子里的用户可以在一个pinned的部分（实际上，这已经把所有的Ps pinned）。

	// Drop victim caches from all pools.
	for _, p := range oldPools {
		p.victim = nil
		p.victimSize = 0
	}

	// Move primary cache to victim cache.
	for _, p := range allPools {
		p.victim = p.local
		p.victimSize = p.localSize
		p.local = nil
		p.localSize = 0
	}

	// The pools with non-empty primary caches now have non-empty
	// victim caches and no pools have primary caches.
	oldPools, allPools = allPools, nil
}

var (
	allPoolsMu Mutex

	// allPools is the set of pools that have non-empty primary
	// caches. Protected by either 1) allPoolsMu and pinning or 2)
	// STW.
	// allPools是拥有非空主缓存的池的集合。由1）allPoolsMu和pinning或2）STW来保护。
	allPools []*Pool

	// oldPools is the set of pools that may have non-empty victim
	// caches. Protected by STW.
	// oldPools是可能有非空的受害者缓存的池的集合。受到STW的保护。
	oldPools []*Pool
)

func init() {
	runtime_registerPoolCleanup(poolCleanup)
}

func indexLocal(l unsafe.Pointer, i int) *poolLocal {
	lp := unsafe.Pointer(uintptr(l) + uintptr(i)*unsafe.Sizeof(poolLocal{}))
	return (*poolLocal)(lp)
}

// Implemented in runtime.
func runtime_registerPoolCleanup(cleanup func())
func runtime_procPin() int
func runtime_procUnpin()

// The below are implemented in runtime/internal/atomic and the
// compiler also knows to intrinsify the symbol we linkname into this
// package.

//go:linkname runtime_LoadAcquintptr runtime/internal/atomic.LoadAcquintptr
func runtime_LoadAcquintptr(ptr *uintptr) uintptr

//go:linkname runtime_StoreReluintptr runtime/internal/atomic.StoreReluintptr
func runtime_StoreReluintptr(ptr *uintptr, val uintptr) uintptr
