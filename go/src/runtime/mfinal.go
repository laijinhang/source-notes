// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Garbage collector: finalizers and block profiling.

package runtime

import (
	"internal/abi"
	"runtime/internal/atomic"
	"runtime/internal/sys"
	"unsafe"
)

// finblock is an array of finalizers to be executed. finblocks are
// arranged in a linked list for the finalizer queue.
//
// finblock is allocated from non-GC'd memory, so any heap pointers
// must be specially handled. GC currently assumes that the finalizer
// queue does not grow during marking (but it can shrink).
//
//go:notinheap
type finblock struct {
	alllink *finblock
	next    *finblock
	cnt     uint32
	_       int32
	fin     [(_FinBlockSize - 2*sys.PtrSize - 2*4) / unsafe.Sizeof(finalizer{})]finalizer
}

var finlock mutex // protects the following variables
// 用于运行finalizer的g，只有一个g，不用的时候休眠，需要的时候再换线
var fing *g // goroutine that runs finalizers
// finalizer的全局队列，这里是已经设置的finalizer串成的链表
var finq *finblock // list of finalizers that are to be executed
// 已经释放的finblock的链表，用于finc缓存起来，以后需要使用的时候可以直接取走，避免再走一遍内存分配了
var finc *finblock // cache of free blocks
var finptrmask [_FinBlockSize / sys.PtrSize / 8]byte

/* fing的标志为，通过fingwait和fingwake，来确定释放需要唤醒fing */
var fingwait bool
var fingwake bool

// 所有的blocks串成的链表
var allfin *finblock // list of all blocks

// NOTE: Layout known to queuefinalizer.
type finalizer struct {
	fn   *funcval       // function to call (may be a heap pointer)
	arg  unsafe.Pointer // ptr to object (may be a heap pointer)
	nret uintptr        // bytes of return values from fn
	fint *_type         // type of first argument of fn
	ot   *ptrtype       // type of ptr to object (may be a heap pointer)
}

var finalizer1 = [...]byte{
	// Each Finalizer is 5 words, ptr ptr INT ptr ptr (INT = uintptr here)
	// Each byte describes 8 words.
	// Need 8 Finalizers described by 5 bytes before pattern repeats:
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	// aka
	//
	//	ptr ptr INT ptr ptr ptr ptr INT
	//	ptr ptr ptr ptr INT ptr ptr ptr
	//	ptr INT ptr ptr ptr ptr INT ptr
	//	ptr ptr ptr INT ptr ptr ptr ptr
	//	INT ptr ptr ptr ptr INT ptr ptr
	//
	// Assumptions about Finalizer layout checked below.
	1<<0 | 1<<1 | 0<<2 | 1<<3 | 1<<4 | 1<<5 | 1<<6 | 0<<7,
	1<<0 | 1<<1 | 1<<2 | 1<<3 | 0<<4 | 1<<5 | 1<<6 | 1<<7,
	1<<0 | 0<<1 | 1<<2 | 1<<3 | 1<<4 | 1<<5 | 0<<6 | 1<<7,
	1<<0 | 1<<1 | 1<<2 | 0<<3 | 1<<4 | 1<<5 | 1<<6 | 1<<7,
	0<<0 | 1<<1 | 1<<2 | 1<<3 | 1<<4 | 0<<5 | 1<<6 | 1<<7,
}

func queuefinalizer(p unsafe.Pointer, fn *funcval, nret uintptr, fint *_type, ot *ptrtype) {
	if gcphase != _GCoff {
		// Currently we assume that the finalizer queue won't
		// grow during marking so we don't have to rescan it
		// during mark termination. If we ever need to lift
		// this assumption, we can do it by adding the
		// necessary barriers to queuefinalizer (which it may
		// have automatically).
		throw("queuefinalizer during GC")
	}

	lock(&finlock)
	if finq == nil || finq.cnt == uint32(len(finq.fin)) {
		if finc == nil {
			finc = (*finblock)(persistentalloc(_FinBlockSize, 0, &memstats.gcMiscSys))
			finc.alllink = allfin
			allfin = finc
			if finptrmask[0] == 0 {
				// Build pointer mask for Finalizer array in block.
				// Check assumptions made in finalizer1 array above.
				if (unsafe.Sizeof(finalizer{}) != 5*sys.PtrSize ||
					unsafe.Offsetof(finalizer{}.fn) != 0 ||
					unsafe.Offsetof(finalizer{}.arg) != sys.PtrSize ||
					unsafe.Offsetof(finalizer{}.nret) != 2*sys.PtrSize ||
					unsafe.Offsetof(finalizer{}.fint) != 3*sys.PtrSize ||
					unsafe.Offsetof(finalizer{}.ot) != 4*sys.PtrSize) {
					throw("finalizer out of sync")
				}
				for i := range finptrmask {
					finptrmask[i] = finalizer1[i%len(finalizer1)]
				}
			}
		}
		block := finc
		finc = block.next
		block.next = finq
		finq = block
	}
	f := &finq.fin[finq.cnt]
	atomic.Xadd(&finq.cnt, +1) // Sync with markroots
	f.fn = fn
	f.nret = nret
	f.fint = fint
	f.ot = ot
	f.arg = p
	fingwake = true
	unlock(&finlock)
}

//go:nowritebarrier
func iterate_finq(callback func(*funcval, unsafe.Pointer, uintptr, *_type, *ptrtype)) {
	for fb := allfin; fb != nil; fb = fb.alllink {
		for i := uint32(0); i < fb.cnt; i++ {
			f := &fb.fin[i]
			callback(f.fn, f.arg, f.nret, f.fint, f.ot)
		}
	}
}

func wakefing() *g {
	var res *g
	lock(&finlock)
	if fingwait && fingwake {
		fingwait = false
		fingwake = false
		res = fing
	}
	unlock(&finlock)
	return res
}

var (
	fingCreate  uint32
	fingRunning bool
)

/*
这个函数是保证，创建了finalizer之后，有一个goroutine去运行，这里只运行一次，这个goroutine会由全局变量 fing 记录
*/
func createfing() {
	// start the finalizer goroutine exactly once
	// 创建一个goroutine，进行时刻监控运行
	if fingCreate == 0 && atomic.Cas(&fingCreate, 0, 1) {
		// 开启一个goroutine运行
		go runfinq()
	}
}

// This is the goroutine that runs all of the finalizers
func runfinq() {
	var (
		frame    unsafe.Pointer
		framecap uintptr
		argRegs  int
	)

	for {
		lock(&finlock)
		// 获取 finq 全局队列，并清空全局队列
		fb := finq
		finq = nil
		if fb == nil {
			// 如果全局队列为空，休眠当前g，等待被唤醒
			gp := getg()
			fing = gp
			// 设置fing的状态标志位
			fingwait = true
			goparkunlock(&finlock, waitReasonFinalizerWait, traceEvGoBlock, 1)
			continue
		}
		argRegs = intArgRegs
		unlock(&finlock)
		// 循环执行runq链表的fin数组
		if raceenabled {
			racefingo()
		}
		for fb != nil {
			for i := fb.cnt; i > 0; i-- {
				f := &fb.fin[i-1]

				// 获取存储当前finalizer的返回数据的大小，如果比之前大，则分配
				var regs abi.RegArgs
				var framesz uintptr
				if argRegs > 0 {
					// The args can always be passed in registers if they're
					// available, because platforms we support always have no
					// argument registers available, or more than 2.
					//
					// But unfortunately because we can have an arbitrary
					// amount of returns and it would be complex to try and
					// figure out how many of those can get passed in registers,
					// just conservatively assume none of them do.
					framesz = f.nret
				} else {
					// Need to pass arguments on the stack too.
					framesz = unsafe.Sizeof((interface{})(nil)) + f.nret
				}
				if framecap < framesz {
					// The frame does not contain pointers interesting for GC,
					// all not yet finalized objects are stored in finq.
					// If we do not mark it as FlagNoScan,
					// the last finalized object is not collected.
					frame = mallocgc(framesz, nil, true)
					framecap = framesz
				}

				if f.fint == nil {
					throw("missing type in runfinq")
				}
				r := frame
				if argRegs > 0 {
					r = unsafe.Pointer(&regs.Ints)
				} else {
					// frame is effectively uninitialized
					// memory. That means we have to clear
					// it before writing to it to avoid
					// confusing the write barrier.
					// 清空frame内存存储
					*(*[2]uintptr)(frame) = [2]uintptr{}
				}
				switch f.fint.kind & kindMask {
				case kindPtr:
					// direct use of pointer
					*(*unsafe.Pointer)(r) = f.arg
				case kindInterface:
					ityp := (*interfacetype)(unsafe.Pointer(f.fint))
					// set up with empty interface
					(*eface)(r)._type = &f.ot.typ
					(*eface)(r).data = f.arg
					if len(ityp.mhdr) != 0 {
						// convert to interface with methods
						// this conversion is guaranteed to succeed - we checked in SetFinalizer
						(*iface)(r).tab = assertE2I(ityp, (*eface)(r)._type)
					}
				default:
					throw("bad kind in runfinq")
				}
				// 调用finalizer函数
				fingRunning = true
				reflectcall(nil, unsafe.Pointer(f.fn), frame, uint32(framesz), uint32(framesz), uint32(framesz), &regs)
				fingRunning = false

				// Drop finalizer queue heap references
				// before hiding them from markroot.
				// This also ensures these will be
				// clear if we reuse the finalizer.
				// 清空finalizer的属性
				f.fn = nil
				f.arg = nil
				f.ot = nil
				atomic.Store(&fb.cnt, i-1)
			}
			// 将已经完成的finalizer放入finc以作缓存，避免再次分配内存
			next := fb.next
			lock(&finlock)
			fb.next = finc
			finc = fb
			unlock(&finlock)
			fb = next
		}
	}
}

// SetFinalizer sets the finalizer associated with obj to the provided
// finalizer function. When the garbage collector finds an unreachable block
// with an associated finalizer, it clears the association and runs
// finalizer(obj) in a separate goroutine. This makes obj reachable again,
// but now without an associated finalizer. Assuming that SetFinalizer
// is not called again, the next time the garbage collector sees
// that obj is unreachable, it will free obj.
// SetFinalizer将与obj关联的终结器设置为提供的终结器函数。
// 当垃圾收集器找到带有关联的终结器的无法访问的块时，它将清除该关联并在单独的goroutine中运行finalizer（obj）。
// 这使得obj可以再次访问，但是现在没有关联的终结器。
// 假设没有再次调用SetFinalizer，则下次垃圾回收器看到obj无法访问时，它将释放obj。
//
// SetFinalizer(obj, nil) clears any finalizer associated with obj.
//
// The argument obj must be a pointer to an object allocated by calling
// new, by taking the address of a composite literal, or by taking the
// address of a local variable.
// The argument finalizer must be a function that takes a single argument
// to which obj's type can be assigned, and can have arbitrary ignored return
// values. If either of these is not true, SetFinalizer may abort the
// program.
//
// Finalizers are run in dependency order: if A points at B, both have
// finalizers, and they are otherwise unreachable, only the finalizer
// for A runs; once A is freed, the finalizer for B can run.
// If a cyclic structure includes a block with a finalizer, that
// cycle is not guaranteed to be garbage collected and the finalizer
// is not guaranteed to run, because there is no ordering that
// respects the dependencies.
//
// The finalizer is scheduled to run at some arbitrary time after the
// program can no longer reach the object to which obj points.
// There is no guarantee that finalizers will run before a program exits,
// so typically they are useful only for releasing non-memory resources
// associated with an object during a long-running program.
// For example, an os.File object could use a finalizer to close the
// associated operating system file descriptor when a program discards
// an os.File without calling Close, but it would be a mistake
// to depend on a finalizer to flush an in-memory I/O buffer such as a
// bufio.Writer, because the buffer would not be flushed at program exit.
//
// It is not guaranteed that a finalizer will run if the size of *obj is
// zero bytes.
//
// It is not guaranteed that a finalizer will run for objects allocated
// in initializers for package-level variables. Such objects may be
// linker-allocated, not heap-allocated.
//
// A finalizer may run as soon as an object becomes unreachable.
// In order to use finalizers correctly, the program must ensure that
// the object is reachable until it is no longer required.
// Objects stored in global variables, or that can be found by tracing
// pointers from a global variable, are reachable. For other objects,
// pass the object to a call of the KeepAlive function to mark the
// last point in the function where the object must be reachable.
//
// For example, if p points to a struct, such as os.File, that contains
// a file descriptor d, and p has a finalizer that closes that file
// descriptor, and if the last use of p in a function is a call to
// syscall.Write(p.d, buf, size), then p may be unreachable as soon as
// the program enters syscall.Write. The finalizer may run at that moment,
// closing p.d, causing syscall.Write to fail because it is writing to
// a closed file descriptor (or, worse, to an entirely different
// file descriptor opened by a different goroutine). To avoid this problem,
// call runtime.KeepAlive(p) after the call to syscall.Write.
//
// A single goroutine runs all finalizers for a program, sequentially.
// If a finalizer must run for a long time, it should do so by starting
// a new goroutine.
// 即使程序正常结束或者发生错误，但是在对象被 gc 选中并被回收之前，
// SetFinalizer 都不会执行，所以不要在SetFinalizer中执行将内存中的内容flush到磁盘这种操作
//
// SetFinalizer 最大的问题是延长了对象生命周期。
// 在第一次回收时执行 Finalizer 函数，且目标对象重新变成可达状态，直到第二次才真正 “销毁”。
// 这对于有大量对象分配的高并发算法，可能会造成很大麻烦
//
// 指针构成的 "循环引⽤" 加上 runtime.SetFinalizer 会导致内存泄露
func SetFinalizer(obj interface{}, finalizer interface{}) {
	if debug.sbrk != 0 {
		// debug.sbrk never frees memory, so no finalizers run
		// (and we don't have the data structures to record them).
		// debug.sbrk永远不会释放内存，因此不会运行终结器（并且我们没有记录它们的数据结构）。
		return
	}
	e := efaceOf(&obj)
	etyp := e._type
	// 第一个参数是nil
	if etyp == nil {
		throw("runtime.SetFinalizer: first argument is nil")
	}
	// 第一个参数不是一个指针
	if etyp.kind&kindMask != kindPtr {
		throw("runtime.SetFinalizer: first argument is " + etyp.string() + ", not pointer")
	}
	ot := (*ptrtype)(unsafe.Pointer(etyp))
	if ot.elem == nil {
		throw("nil elem type!")
	}

	// find the containing object
	// 找到包含的对象
	// 在内存中找不到分配的地址时 base==0，setFinalizer 是在内存回收的时候调用，没有分配就不会回收
	base, _, _ := findObject(uintptr(e.data), 0, 0)

	if base == 0 {
		// 0-length objects are okay.
		if e.data == unsafe.Pointer(&zerobase) {
			return
		}

		// Global initializers might be linker-allocated.
		//	var Foo = &Object{}
		//	func main() {
		//		runtime.SetFinalizer(Foo, nil)
		//	}
		// The relevant segments are: noptrdata, data, bss, noptrbss.
		// We cannot assume they are in any order or even contiguous,
		// due to external linking.
		for datap := &firstmoduledata; datap != nil; datap = datap.next {
			if datap.noptrdata <= uintptr(e.data) && uintptr(e.data) < datap.enoptrdata ||
				datap.data <= uintptr(e.data) && uintptr(e.data) < datap.edata ||
				datap.bss <= uintptr(e.data) && uintptr(e.data) < datap.ebss ||
				datap.noptrbss <= uintptr(e.data) && uintptr(e.data) < datap.enoptrbss {
				return
			}
		}
		throw("runtime.SetFinalizer: pointer not in allocated block")
	}

	if uintptr(e.data) != base {
		// As an implementation detail we allow to set finalizers for an inner byte
		// of an object if it could come from tiny alloc (see mallocgc for details).
		if ot.elem == nil || ot.elem.ptrdata != 0 || ot.elem.size >= maxTinySize {
			throw("runtime.SetFinalizer: pointer not at beginning of allocated block")
		}
	}

	f := efaceOf(&finalizer)
	ftyp := f._type
	// 如果 finalizer type == nil，尝试移除（没有的话，就不需要移除了）
	if ftyp == nil {
		// switch to system stack and remove finalizer
		systemstack(func() {
			removefinalizer(e.data)
		})
		return
	}

	if ftyp.kind&kindMask != kindFunc {
		throw("runtime.SetFinalizer: second argument is " + ftyp.string() + ", not a function")
	}
	ft := (*functype)(unsafe.Pointer(ftyp))
	if ft.dotdotdot() {
		throw("runtime.SetFinalizer: cannot pass " + etyp.string() + " to finalizer " + ftyp.string() + " because dotdotdot")
	}
	if ft.inCount != 1 {
		throw("runtime.SetFinalizer: cannot pass " + etyp.string() + " to finalizer " + ftyp.string())
	}
	fint := ft.in()[0]
	switch {
	case fint == etyp:
		// ok - same type
		goto okarg
	case fint.kind&kindMask == kindPtr:
		if (fint.uncommon() == nil || etyp.uncommon() == nil) && (*ptrtype)(unsafe.Pointer(fint)).elem == ot.elem {
			// ok - not same type, but both pointers,
			// one or the other is unnamed, and same element type, so assignable.
			goto okarg
		}
	case fint.kind&kindMask == kindInterface:
		ityp := (*interfacetype)(unsafe.Pointer(fint))
		if len(ityp.mhdr) == 0 {
			// ok - satisfies empty interface
			goto okarg
		}
		if iface := assertE2I2(ityp, *efaceOf(&obj)); iface.tab != nil {
			goto okarg
		}
	}
	throw("runtime.SetFinalizer: cannot pass " + etyp.string() + " to finalizer " + ftyp.string())
okarg:
	// compute size needed for return parameters
	// 计算返回参数的大小并进行对齐
	nret := uintptr(0)
	for _, t := range ft.out() {
		nret = alignUp(nret, uintptr(t.align)) + uintptr(t.size)
	}
	nret = alignUp(nret, sys.PtrSize)

	// make sure we have a finalizer goroutine
	// 确保 finalizer 有一个 goroutine
	createfing()

	systemstack(func() {
		// 却换到g0，添加finalizer，并且不能重复设置
		if !addfinalizer(e.data, (*funcval)(f.data), nret, fint, ot) {
			throw("runtime.SetFinalizer: finalizer already set")
		}
	})
}

// Mark KeepAlive as noinline so that it is easily detectable as an intrinsic.
//go:noinline

// KeepAlive marks its argument as currently reachable.
// This ensures that the object is not freed, and its finalizer is not run,
// before the point in the program where KeepAlive is called.
//
// A very simplified example showing where KeepAlive is required:
// 	type File struct { d int }
// 	d, err := syscall.Open("/file/path", syscall.O_RDONLY, 0)
// 	// ... do something if err != nil ...
// 	p := &File{d}
// 	runtime.SetFinalizer(p, func(p *File) { syscall.Close(p.d) })
// 	var buf [10]byte
// 	n, err := syscall.Read(p.d, buf[:])
// 	// Ensure p is not finalized until Read returns.
// 	runtime.KeepAlive(p)
// 	// No more uses of p after this point.
//
// Without the KeepAlive call, the finalizer could run at the start of
// syscall.Read, closing the file descriptor before syscall.Read makes
// the actual system call.
// 保留一个引用只需要产生一个参数传递，而这里针对 cgo 做了特殊处理，
// 即 产生了一个 println 调用来保证编译器不会将其优化掉。
func KeepAlive(x interface{}) {
	// Introduce a use of x that the compiler can't eliminate.
	// This makes sure x is alive on entry. We need x to be alive
	// on entry for "defer runtime.KeepAlive(x)"; see issue 21402.
	if cgoAlwaysFalse {
		println(x)
	}
}
