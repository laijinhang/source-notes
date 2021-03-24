// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Fixed-size object allocator. Returned memory is not zeroed.
//
// See malloc.go for overview.

package runtime

import "unsafe"

// FixAlloc is a simple free-list allocator for fixed size objects.
// Malloc uses a FixAlloc wrapped around sysAlloc to manage its
// mcache and mspan objects.
//
// Memory returned by fixalloc.alloc is zeroed by default, but the
// caller may take responsibility for zeroing allocations by setting
// the zero flag to false. This is only safe if the memory never
// contains heap pointers.
//
// The caller is responsible for locking around FixAlloc calls.
// Callers can keep state in the object but the first word is
// smashed by freeing and reallocating.
//
// Consider marking fixalloc'd types go:notinheap.
//
// FixAlloc是一个简单的固定大小对象的自由表内存分配器。
// Malloc使用围绕sysAlloc的fixalloc来管理其MCache和MSpan对象。
//
// fixalloc.alloc返回的内存默认为零，但调用者可以通过将zero标记为false
// 来自行负责将分配归零。这部分内存永远不包含堆指针，则这样的操作是安全的。
//
// 负责方负责锁定fixalloc调用。调用方可以在对象中保持状态，
// 但当释放和重新分配时第一个字会被破坏。‘
//
// 考虑使fixalloc的类型变为 go:notinheap
type fixalloc struct {
	size uintptr
	// 首次调用时，返回p
	first func(arg, p unsafe.Pointer) // called first time p is returned
	arg   unsafe.Pointer
	list  *mlink
	// 使用uintptr而非unsafe.Pointer来避免write barriers
	chunk  uintptr // use uintptr instead of unsafe.Pointer to avoid write barriers
	nchunk uint32
	// 正在使用的字节
	inuse uintptr // in-use bytes now
	stat  *sysMemStat
	// 归零的分配
	zero bool // zero allocations
}

// A generic linked list of blocks.  (Typically the block is bigger than sizeof(MLink).)
// Since assignments to mlink.next will result in a write barrier being performed
// this cannot be used by some of the internal GC structures. For example when
// the sweeper is placing an unmarked object on the free list it does not want the
// write barrier to be called since that could result in the object being reachable.
//
//go:notinheap
type mlink struct {
	next *mlink
}

// Initialize f to allocate objects of the given size,
// using the allocator to obtain chunks of memory.
// 初始化 f 来分配给定大小的对象。
// 使用分配器来按chunk获取
func (f *fixalloc) init(size uintptr, first func(arg, p unsafe.Pointer), arg unsafe.Pointer, stat *sysMemStat) {
	f.size = size
	f.first = first
	f.arg = arg
	f.list = nil
	f.chunk = 0
	f.nchunk = 0
	f.inuse = 0
	f.stat = stat
	f.zero = true
}

// 分配
// fixalloc基于自由表策略进行实现，分为两种情况：
// 1.存在被释放、可复用的内存
// 2.不存在可复用的内存
//
// 对于第一种情况，也就是在运行时内存被释放，但这部分内存并不会被立即回收给操作系统，
// 我们直接从自由表中获得即可，但需要注意按需将这部分内存进行清零操作。
// 对于第二种情况，我们直接向操作系统申请固定大小的内存，然后扣除分配的内存即可。
func (f *fixalloc) alloc() unsafe.Pointer {
	// fixalloc的字段必须先被init
	if f.size == 0 {
		print("runtime: use of FixAlloc_Alloc before FixAlloc_Init\n")
		throw("runtime: internal error")
	}

	// 如果f.list不是nil，则说明还存在已经释放，可复用的内存，直接将其分配
	if f.list != nil {
		// 取出f.list
		v := unsafe.Pointer(f.list)
		//并将其指向下一段区域
		f.list = f.list.next
		// 增加使用的（分配）大小
		f.inuse += f.size
		// 如果需要堆内存清零，则对取出的内存执行初始化
		if f.zero {
			memclrNoHeapPointers(v, f.size)
		}
		// 返会分配的内存
		return v
	}
	// f.list中没有可复用的内存

	// 如果此时nchunk不足以分配一个size
	if uintptr(f.nchunk) < f.size {
		// 则向操作系统申请内存，大小为 16 << 10 pow(2,14)
		f.chunk = uintptr(persistentalloc(_FixAllocChunk, 0, f.stat))
		f.nchunk = _FixAllocChunk
	}

	// 指向申请好的内存
	v := unsafe.Pointer(f.chunk)
	if f.first != nil { // first只有在fixalloc作为spanalloc时候，才会被设置为recordspan
		f.first(f.arg, v) // 用于为heap.allspans添加新的span
	}
	// 扣除并保留size大小的空间
	f.chunk = f.chunk + f.size
	f.nchunk -= uint32(f.size)
	f.inuse += f.size // 记录已使用的大小
	return v
}

// 回收
func (f *fixalloc) free(p unsafe.Pointer) {
	// 减少使用的字节数
	f.inuse -= f.size
	// 将要释放的内存地址作为mlink指针插入到f.list内，完成回收
	v := (*mlink)(p)
	v.next = f.list
	f.list = v
}
