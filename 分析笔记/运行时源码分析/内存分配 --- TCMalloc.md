TCMalloc全称Thread-Cacheing Malloc，即线程缓存的Malloc，它是谷歌实现的一套用于内存分配管理

# 一、组成
* cache
* central
* heap

malloc.go源码注释   // Large objects (> 32 kB) are allocated straight from the heap.
当分配对象大于（ > 32KB）直接从堆mhead中分配
当分配对象小于（<= 32KB）直接通过mcache分配

**初始化malloc：**runtime/malloc.go mallocinit()

**初始化heap：**runtime/malloc.go mheap_.init() -> runtime/mheap.go func (h *mheap) init()

# 二、内存布局
go管理的内存由三部分组成：
* spans：512M，存放 mspan 的指针，每个指针对应一页，所以 spans 区域的大小就是 512G/8KB*8B = 512MB
* bitmap：16G，标识 arena 区域哪些地址保存了对象，并且用 4bit 标志位表示对象 是否包含指针、GC标记信息
* arena：512G，这个就是堆区，Go动态分配的内存都是在这个区域，它把内存分割成 8KB 大小的页，一些页组合起来称为 mspan

mspan在go源码中的定义：
runtime/mheap.go
```go
type mspan struct {
	// 链表的下一个节点，如果下一个节点为空，则为nil
	next *mspan // next span in list, or nil if none
	// 链表的上一个节点，如果上一个节点为空，则为nil
	prev *mspan // previous span in list, or nil if none
	// 链表地址
	list *mSpanList // For debugging. TODO: Remove.

	// 该span在arena区域的起始地址
	startAddr uintptr // address of first byte of span aka s.base()
	// 该span占用arena区域page的数量
	npages uintptr // number of pages in span

	// 空闲对象列表
	manualFreeList gclinkptr // list of free objects in mSpanManual spans

	// freeindex is the slot index between 0 and nelems at which to begin scanning
	// for the next free object in this span.
	// Each allocation scans allocBits starting at freeindex until it encounters a 0
	// indicating a free object. freeindex is then adjusted so that subsequent scans begin
	// just past the newly discovered free object.
	//
	// If freeindex == nelem, this span has no free objects.
	//
	// allocBits is a bitmap of objects in this span.
	// If n >= freeindex and allocBits[n/8] & (1<<(n%8)) is 0
	// then object n is free;
	// otherwise, object n is allocated. Bits starting at nelem are
	// undefined and should never be referenced.
	//
	// Object n starts at address n*elemsize + (start << pageShift).
	freeindex uintptr
	// TODO: Look up nelems from sizeclass and remove this field if it
	// helps performance.
	// 管理的对象数
	nelems uintptr // number of object in the span.

	// Cache of the allocBits at freeindex. allocCache is shifted
	// such that the lowest bit corresponds to the bit freeindex.
	// allocCache holds the complement of allocBits, thus allowing
	// ctz (count trailing zero) to use it directly.
	// allocCache may contain bits beyond s.nelems; the caller must ignore
	// these.
	// 从freeindex开始的标记位
	allocCache uint64

	// allocBits and gcmarkBits hold pointers to a span's mark and
	// allocation bits. The pointers are 8 byte aligned.
	// There are three arenas where this data is held.
	// free: Dirty arenas that are no longer accessed
	//       and can be reused.
	// next: Holds information to be used in the next GC cycle.
	// current: Information being used during this GC cycle.
	// previous: Information being used during the last GC cycle.
	// A new GC cycle starts with the call to finishsweep_m.
	// finishsweep_m moves the previous arena to the free arena,
	// the current arena to the previous arena, and
	// the next arena to the current arena.
	// The next arena is populated as the spans request
	// memory to hold gcmarkBits for the next GC cycle as well
	// as allocBits for newly allocated spans.
	//
	// The pointer arithmetic is done "by hand" instead of using
	// arrays to avoid bounds checks along critical performance
	// paths.
	// The sweep will free the old allocBits and set allocBits to the
	// gcmarkBits. The gcmarkBits are replaced with a fresh zeroed
	// out memory.
	allocBits  *gcBits // 已分配的对象的个数
	gcmarkBits *gcBits // span分类

	// sweep generation:
	// if sweepgen == h->sweepgen - 2, the span needs sweeping
	// if sweepgen == h->sweepgen - 1, the span is currently being swept
	// if sweepgen == h->sweepgen, the span is swept and ready to use
	// if sweepgen == h->sweepgen + 1, the span was cached before sweep began and is still cached, and needs sweeping
	// if sweepgen == h->sweepgen + 3, the span was swept and then cached and is still cached
	// h->sweepgen is incremented by 2 after every GC

	sweepgen   uint32
	divMul     uint32        // for divide by elemsize
	allocCount uint16        // number of allocated objects
	spanclass  spanClass     // size class and noscan (uint8)
	state      mSpanStateBox // mSpanInUse etc; accessed atomically (get/set methods)
	needzero   uint8         // needs to be zeroed before allocation
	elemsize   uintptr       // computed from sizeclass or from npages
	// 申请大对象内存块会用到，mspan的数据截止位置
	limit       uintptr // end of data in span
	speciallock mutex   // guards specials list
	/*
		当前span上所有对象的special串成链表
		special中有个offset，就是数据对象在span上的offset，通过offset，将数据对象和special关联起来
	*/
	specials *special // linked list of special records sorted by offset.
}
```