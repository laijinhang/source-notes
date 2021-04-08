// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"runtime/internal/atomic"
	"unsafe"
)

// Per-thread (in Go, per-P) cache for small objects.
// This includes a small object cache and local allocation stats.
// No locking needed because it is per-thread (per-P).
//
// mcaches are allocated from non-GC'd memory, so any heap pointers
// must be specially handled.
//
// mcache是一个Per-P的缓存，因此每个线程都只访问自身的mache，因此也就不会出现并发，也就省去了对其进行加锁步骤。
//go:notinheap
type mcache struct {
	// The following members are accessed on every malloc,
	// so they are grouped here for better caching.
	// 下面的成员在每次malloc时都会被访问。
	// 因此将它们放到一起来利用缓存的局部性原理。
	nextSample uintptr // trigger heap sample after allocating this many bytes，分配这么多字节后触发堆样本
	scanAlloc  uintptr // bytes of scannable heap allocated，分配的可扫描堆的字节数

	// Allocator cache for tiny objects w/o pointers.
	// See "Tiny allocator" comment in malloc.go.

	// tiny points to the beginning of the current tiny block, or
	// nil if there is no current tiny block.
	//
	// tiny is a heap pointer. Since mcache is in non-GC'd memory,
	// we handle it by clearing it in releaseAll during mark
	// termination.
	//
	// tinyAllocs is the number of tiny allocations performed
	// by the P that owns this mcache.
	// 没有指针的微小对象的内存分配器缓存。
	// 请参考 malloc.go 中的”小型分配器“注释。
	//
	// tiny指向当前tiny块的起始位置，或没有tiny块时为nil
	// tiny是一个堆指针。由于mcache在非GC内存中，我们通常在
	// make termination期间在releaseAll中清除它来处理它。
	tiny       uintptr
	tinyoffset uintptr
	tinyAllocs uintptr

	// The rest is not accessed on every malloc.
	// 下面的不在每个malloc时被访问

	// 用来分配的spans，由spanClass索引
	alloc [numSpanClasses]*mspan // spans to allocate from, indexed by spanClass

	stackcache [_NumStackOrders]stackfreelist

	// flushGen indicates the sweepgen during which this mcache
	// was last flushed. If flushGen != mheap_.sweepgen, the spans
	// in this mcache are stale and need to the flushed so they
	// can be swept. This is done in acquirep.
	flushGen uint32
}

// A gclink is a node in a linked list of blocks, like mlink,
// but it is opaque to the garbage collector.
// The GC does not trace the pointers during collection,
// and the compiler does not emit write barriers for assignments
// of gclinkptr values. Code should store references to gclinks
// as gclinkptr, not as *gclink.
type gclink struct {
	next gclinkptr
}

// A gclinkptr is a pointer to a gclink, but it is opaque
// to the garbage collector.
// gclinkptr是指向gclink的指针，但对垃圾收集器不透明。
type gclinkptr uintptr

// ptr returns the *gclink form of p.
// The result should be used for accessing fields, not stored
// in other data structures.
// ptr返回p的*gclink形式。结果应该用于访问字段，
// 而不是存储在其他数据结构中。
func (p gclinkptr) ptr() *gclink {
	return (*gclink)(unsafe.Pointer(p))
}

type stackfreelist struct {
	list gclinkptr // linked list of free stacks.空闲堆栈的链表
	size uintptr   // total size of stacks in list，链表中的堆栈大小
}

// dummy mspan that contains no free objects.
// 虚拟的MSpan，不包含任何对象。
var emptymspan mspan

func allocmcache() *mcache {
	var c *mcache
	systemstack(func() {
		lock(&mheap_.lock)
		c = (*mcache)(mheap_.cachealloc.alloc())
		c.flushGen = mheap_.sweepgen
		unlock(&mheap_.lock)
	})
	for i := range c.alloc {
		c.alloc[i] = &emptymspan // 暂时指向虚拟内存
	}
	// 返回下一个采样点，是服从泊松过程的随机数
	c.nextSample = nextSample()
	return c
}

// freemcache releases resources associated with this
// mcache and puts the object onto a free list.
//
// In some cases there is no way to simply release
// resources, such as statistics, so donate them to
// a different mcache (the recipient).
func freemcache(c *mcache) {
	systemstack(func() {
		// 归还span
		c.releaseAll()
		// 释放stack
		stackcache_clear(c)

		// NOTE(rsc,rlh): If gcworkbuffree comes back, we need to coordinate
		// with the stealing of gcworkbufs during garbage collection to avoid
		// a race where the workbuf is double-freed.
		// gcworkbuffree(c.gcworkbuf)

		lock(&mheap_.lock)
		// 将mcache释放
		mheap_.cachealloc.free(unsafe.Pointer(c))
		unlock(&mheap_.lock)
	})
}

// getMCache is a convenience function which tries to obtain an mcache.
//
// Returns nil if we're not bootstrapping or we don't have a P. The caller's
// P must not change, so we must be in a non-preemptible state.
func getMCache() *mcache {
	// Grab the mcache, since that's where stats live.
	pp := getg().m.p.ptr()
	var c *mcache
	if pp == nil {
		// We will be called without a P while bootstrapping,
		// in which case we use mcache0, which is set in mallocinit.
		// mcache0 is cleared when bootstrapping is complete,
		// by procresize.
		c = mcache0
	} else {
		c = pp.mcache
	}
	return c
}

// refill acquires a new span of span class spc for c. This span will
// have at least one free object. The current span in c must be full.
//
// Must run in a non-preemptible context since otherwise the owner of
// c could change.
func (c *mcache) refill(spc spanClass) {
	// Return the current cached span to the central lists.
	s := c.alloc[spc]

	if uintptr(s.allocCount) != s.nelems {
		throw("refill of span with free space remaining")
	}
	if s != &emptymspan {
		// Mark this span as no longer cached.
		if s.sweepgen != mheap_.sweepgen+3 {
			throw("bad sweepgen in refill")
		}
		mheap_.central[spc].mcentral.uncacheSpan(s)
	}

	// Get a new cached span from the central lists.
	s = mheap_.central[spc].mcentral.cacheSpan()
	if s == nil {
		throw("out of memory")
	}

	if uintptr(s.allocCount) == s.nelems {
		throw("span has no free space")
	}

	// Indicate that this span is cached and prevent asynchronous
	// sweeping in the next sweep phase.
	s.sweepgen = mheap_.sweepgen + 3

	// Assume all objects from this span will be allocated in the
	// mcache. If it gets uncached, we'll adjust this.
	stats := memstats.heapStats.acquire()
	atomic.Xadduintptr(&stats.smallAllocCount[spc.sizeclass()], uintptr(s.nelems)-uintptr(s.allocCount))
	memstats.heapStats.release()

	// Update heap_live with the same assumption.
	usedBytes := uintptr(s.allocCount) * s.elemsize
	atomic.Xadd64(&memstats.heap_live, int64(s.npages*pageSize)-int64(usedBytes))

	// Flush tinyAllocs.
	if spc == tinySpanClass {
		atomic.Xadd64(&memstats.tinyallocs, int64(c.tinyAllocs))
		c.tinyAllocs = 0
	}

	// While we're here, flush scanAlloc, since we have to call
	// revise anyway.
	atomic.Xadd64(&memstats.heap_scan, int64(c.scanAlloc))
	c.scanAlloc = 0

	if trace.enabled {
		// heap_live changed.
		traceHeapAlloc()
	}
	if gcBlackenEnabled != 0 {
		// heap_live and heap_scan changed.
		gcController.revise()
	}

	c.alloc[spc] = s
}

// allocLarge allocates a span for a large object.
func (c *mcache) allocLarge(size uintptr, needzero bool, noscan bool) *mspan {
	if size+_PageSize < size {
		throw("out of memory")
	}
	npages := size >> _PageShift
	if size&_PageMask != 0 {
		npages++
	}

	// Deduct credit for this span allocation and sweep if
	// necessary. mHeap_Alloc will also sweep npages, so this only
	// pays the debt down to npage pages.
	deductSweepCredit(npages*_PageSize, npages)

	spc := makeSpanClass(0, noscan)
	s := mheap_.alloc(npages, spc, needzero)
	if s == nil {
		throw("out of memory")
	}
	stats := memstats.heapStats.acquire()
	atomic.Xadduintptr(&stats.largeAlloc, npages*pageSize)
	atomic.Xadduintptr(&stats.largeAllocCount, 1)
	memstats.heapStats.release()

	// Update heap_live and revise pacing if needed.
	atomic.Xadd64(&memstats.heap_live, int64(npages*pageSize))
	if trace.enabled {
		// Trace that a heap alloc occurred because heap_live changed.
		traceHeapAlloc()
	}
	if gcBlackenEnabled != 0 {
		gcController.revise()
	}

	// Put the large span in the mcentral swept list so that it's
	// visible to the background sweeper.
	mheap_.central[spc].mcentral.fullSwept(mheap_.sweepgen).push(s)
	s.limit = s.base() + size
	heapBitsForAddr(s.base()).initSpan(s)
	return s
}

// 释放
// 由于mcache从非GC内存上进行分配，因此出现的任何堆指针都必须进行特殊处理。
// 所以在释放前，需要调用mcache.releaseAll将堆指针进行处理：
func (c *mcache) releaseAll() {
	// Take this opportunity to flush scanAlloc.
	atomic.Xadd64(&memstats.heap_scan, int64(c.scanAlloc))
	c.scanAlloc = 0

	sg := mheap_.sweepgen
	for i := range c.alloc {
		s := c.alloc[i]
		if s != &emptymspan {
			// Adjust nsmallalloc in case the span wasn't fully allocated.
			n := uintptr(s.nelems) - uintptr(s.allocCount)
			stats := memstats.heapStats.acquire()
			atomic.Xadduintptr(&stats.smallAllocCount[spanClass(i).sizeclass()], -n)
			memstats.heapStats.release()
			if s.sweepgen != sg+1 {
				// refill conservatively counted unallocated slots in heap_live.
				// Undo this.
				//
				// If this span was cached before sweep, then
				// heap_live was totally recomputed since
				// caching this span, so we don't do this for
				// stale spans.
				atomic.Xadd64(&memstats.heap_live, -int64(n)*int64(s.elemsize))
			}
			// Release the span to the mcentral.
			mheap_.central[i].mcentral.uncacheSpan(s)
			c.alloc[i] = &emptymspan
		}
	}
	// Clear tinyalloc pool.
	// 清空tinyalloc池
	c.tiny = 0
	c.tinyoffset = 0
	atomic.Xadd64(&memstats.tinyallocs, int64(c.tinyAllocs))
	c.tinyAllocs = 0

	// Updated heap_scan and possible heap_live.
	if gcBlackenEnabled != 0 {
		gcController.revise()
	}
}

// prepareForSweep flushes c if the system has entered a new sweep phase
// since c was populated. This must happen between the sweep phase
// starting and the first allocation from c.
func (c *mcache) prepareForSweep() {
	// Alternatively, instead of making sure we do this on every P
	// between starting the world and allocating on that P, we
	// could leave allocate-black on, allow allocation to continue
	// as usual, use a ragged barrier at the beginning of sweep to
	// ensure all cached spans are swept, and then disable
	// allocate-black. However, with this approach it's difficult
	// to avoid spilling mark bits into the *next* GC cycle.
	sg := mheap_.sweepgen
	if c.flushGen == sg {
		return
	} else if c.flushGen != sg-2 {
		println("bad flushGen", c.flushGen, "in prepareForSweep; sweepgen", sg)
		throw("bad flushGen")
	}
	c.releaseAll()
	stackcache_clear(c)
	atomic.Store(&c.flushGen, mheap_.sweepgen) // Synchronizes with gcStart
}
