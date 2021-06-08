# 一、结构
### 1、defer数据结构
runtime/panic.go
```go
type _defer struct {
	siz int32       // 参数的大小
	started bool    // 是否执行过了
	heap    bool
	openDefer bool
	sp        uintptr  
	pc        uintptr  
	fn        *funcval 
	// defer中的panic
	_panic *_panic
	// defer链表，函数执行流程中的defer，会通过 link 这个属性进行串联
	link *_defer

	fd   unsafe.Pointer 
	varp uintptr        
	framepc uintptr
}
```
### 2、panic数据结构
```go
/*
在panic中使用 _panic 作为其基础单元，每执行一次 panic 语句，都会创建一个 _panic 对象。
它包含了一些基础的字段用于存储当前的panic调用情况，涉及的字段如下：
1. argp：指向 defer 延迟调用的参数的指针
2. arg：panic的原因，也就是调用 panic 时传入的参数
3. link：指向上一个调用的 _panic，这里说明panci也是一个链表
4. recovered：panic是否已经被处理过，也就是是否被recover接收掉了
5. aborted：panic是否被终止
*/
type _panic struct {
	argp      unsafe.Pointer // pointer to arguments of deferred call run during panic; cannot move - known to liblink
	arg       interface{}    // argument to panic
	link      *_panic        // link to earlier panic
	pc        uintptr        // where to return to in runtime if this panic is bypassed
	sp        unsafe.Pointer // where to return to in runtime if this panic is bypassed
	recovered bool           // whether this panic is over
	aborted   bool           // the panic was aborted
	goexit    bool
}
```
### 3、G
defer和panic都是绑定在 所运行的g上的
```go
// g中与defer、panic相关属性
type g struct {
_panic         *_panic // panic组成的链表
_defer         *_defer // defer组成的先进后出的链表，同栈
}
```
# 二、源码分析
通过一段简单的代码进行分析
```go
package main

func main() {
	defer func() {
		recover()
	}()
	panic("error")
}
```
* go build -gcflags=all="-N -l" main.go
* go tool objdump -s "main.main" main
```go
go tool objdump -s "main\.main" main | grep CALL

main.go:4             0x46895a                e8c192fcff              CALL runtime.deferprocStack(SB)
main.go:7             0x468980                e81ba6fcff              CALL runtime.gopanic(SB)
main.go:4             0x468986                e8b59afcff              CALL runtime.deferreturn(SB)
main.go:3             0x468995                e8e6afffff              CALL runtime.morestack_noctxt(SB)
main.go:5             0x4689c6                e895aefcff              CALL runtime.gorecover(SB)
main.go:4             0x4689d5                e8a6afffff              CALL runtime.morestack_noctxt(SB) 
```
通过反编译结果可以看出，
* 与defer相关的调用`runtime.deferprocStack(SB)`和`runtime.deferreturn(SB)`
* 与panic相关的调用`runtime.gopanic(SB)`
* 与recover相关的调用`runtime.gorecover(SB)`

### 1、deferprocStack
```go
func deferprocStack(d *_defer) {
	gp := getg()
	if gp.m.curg != gp {
		throw("defer on system stack")
	}
	if goexperiment.RegabiDefer && d.siz != 0 {
		throw("defer with non-empty frame")
	}
	d.started = false
	d.heap = false
	d.openDefer = false
	d.sp = getcallersp()
	d.pc = getcallerpc()
	d.framepc = 0
	d.varp = 0

	*(*uintptr)(unsafe.Pointer(&d._panic)) = 0
	*(*uintptr)(unsafe.Pointer(&d.fd)) = 0
	*(*uintptr)(unsafe.Pointer(&d.link)) = uintptr(unsafe.Pointer(gp._defer))
	*(*uintptr)(unsafe.Pointer(&gp._defer)) = uintptr(unsafe.Pointer(d))

	return0()
}
```
### 2、deferreturn
```go
func deferreturn() {
	gp := getg()
	d := gp._defer
	if d == nil {
		return
	}
	sp := getcallersp()
	if d.sp != sp {
		return
	}
	if d.openDefer {
		done := runOpenDeferFrame(gp, d)
		if !done {
			throw("unfinished open-coded defers in deferreturn")
		}
		gp._defer = d.link
		freedefer(d)
		return
	}

	argp := getcallersp() + sys.MinFrameSize
	switch d.siz {
	case 0:
		// Do nothing.
	case sys.PtrSize:
		*(*uintptr)(unsafe.Pointer(argp)) = *(*uintptr)(deferArgs(d))
	default:
		memmove(unsafe.Pointer(argp), deferArgs(d), uintptr(d.siz))
	}
	fn := d.fn
	d.fn = nil
	gp._defer = d.link
	freedefer(d)

	_ = fn.fn
	jmpdefer(fn, argp)
}
```
### 3、gopanic
```go
/*
gopanic函数的操作：
1. 获取指向当前 Goroutine 的指针
2. 初始化一个 panic 的基本单位 _panic，并将这个panic头插入到goroutine的panic链表中
3. 获取当前Goroutine上挂载的 _defer
4. 若当前存在defer调用，则调用reflectcall方法去指向先前defer中延迟执行的代码。
refectcall方法若在执行过程中需要运行 recover 将会调用 gorecover 方法。
5. 结束前，使用preprintpanics方法打印所涉及的 panic 消息
6. 最后调用 fatalpanic 中止应用程序，实际是执行 exit(2) 进行最终退出行为。
也就是处理当前 Gorougine(g) 上所挂载的 ._panic 链表（所以无法对其 Goroutine 的异常事件响应），
然后对其所属的defer链表和recover进行检测并处理，最后调用退出命名中止应用程序。
*/

// The implementation of the predeclared function panic.
func gopanic(e interface{}) {
	gp := getg()
	if gp.m.curg != gp {
		print("panic: ")
		printany(e)
		print("\n")
		throw("panic on system stack")
	}

	if gp.m.mallocing != 0 {
		print("panic: ")
		printany(e)
		print("\n")
		throw("panic during malloc")
	}
	if gp.m.preemptoff != "" {
		print("panic: ")
		printany(e)
		print("\n")
		print("preempt off reason: ")
		print(gp.m.preemptoff)
		print("\n")
		throw("panic during preemptoff")
	}
	if gp.m.locks != 0 {
		print("panic: ")
		printany(e)
		print("\n")
		throw("panic holding locks")
	}

	var p _panic
	p.arg = e
	p.link = gp._panic
	gp._panic = (*_panic)(noescape(unsafe.Pointer(&p)))

	atomic.Xadd(&runningPanicDefers, 1)

	// By calculating getcallerpc/getcallersp here, we avoid scanning the
	// gopanic frame (stack scanning is slow...)
	addOneOpenDeferFrame(gp, getcallerpc(), unsafe.Pointer(getcallersp()))

	for {
		d := gp._defer
		if d == nil {
			break
		}

		// If defer was started by earlier panic or Goexit (and, since we're back here, that triggered a new panic),
		// take defer off list. An earlier panic will not continue running, but we will make sure below that an
		// earlier Goexit does continue running.
		if d.started {
			if d._panic != nil {
				d._panic.aborted = true
			}
			d._panic = nil
			if !d.openDefer {
				// For open-coded defers, we need to process the
				// defer again, in case there are any other defers
				// to call in the frame (not including the defer
				// call that caused the panic).
				d.fn = nil
				gp._defer = d.link
				freedefer(d)
				continue
			}
		}

		// Mark defer as started, but keep on list, so that traceback
		// can find and update the defer's argument frame if stack growth
		// or a garbage collection happens before executing d.fn.
		d.started = true

		// Record the panic that is running the defer.
		// If there is a new panic during the deferred call, that panic
		// will find d in the list and will mark d._panic (this panic) aborted.
		d._panic = (*_panic)(noescape(unsafe.Pointer(&p)))

		done := true
		if d.openDefer {
			done = runOpenDeferFrame(gp, d)
			if done && !d._panic.recovered {
				addOneOpenDeferFrame(gp, 0, nil)
			}
		} else {
			p.argp = unsafe.Pointer(getargp())

			if goexperiment.RegabiDefer {
				fn := deferFunc(d)
				fn()
			} else {
				// Pass a dummy RegArgs since we'll only take this path if
				// we're not using the register ABI.
				var regs abi.RegArgs
				reflectcall(nil, unsafe.Pointer(d.fn), deferArgs(d), uint32(d.siz), uint32(d.siz), uint32(d.siz), &regs)
			}
		}
		p.argp = nil

		// Deferred function did not panic. Remove d.
		if gp._defer != d {
			throw("bad defer entry in panic")
		}
		d._panic = nil

		// trigger shrinkage to test stack copy. See stack_test.go:TestStackPanic
		//GC()

		pc := d.pc
		sp := unsafe.Pointer(d.sp) // must be pointer so it gets adjusted during stack copy
		if done {
			d.fn = nil
			gp._defer = d.link
			freedefer(d)
		}
		if p.recovered {
			gp._panic = p.link
			if gp._panic != nil && gp._panic.goexit && gp._panic.aborted {
				// A normal recover would bypass/abort the Goexit.  Instead,
				// we return to the processing loop of the Goexit.
				gp.sigcode0 = uintptr(gp._panic.sp)
				gp.sigcode1 = uintptr(gp._panic.pc)
				mcall(recovery)
				throw("bypassed recovery failed") // mcall should not return
			}
			atomic.Xadd(&runningPanicDefers, -1)

			// Remove any remaining non-started, open-coded
			// defer entries after a recover, since the
			// corresponding defers will be executed normally
			// (inline). Any such entry will become stale once
			// we run the corresponding defers inline and exit
			// the associated stack frame.
			d := gp._defer
			var prev *_defer
			if !done {
				// Skip our current frame, if not done. It is
				// needed to complete any remaining defers in
				// deferreturn()
				prev = d
				d = d.link
			}
			for d != nil {
				if d.started {
					// This defer is started but we
					// are in the middle of a
					// defer-panic-recover inside of
					// it, so don't remove it or any
					// further defer entries
					break
				}
				if d.openDefer {
					if prev == nil {
						gp._defer = d.link
					} else {
						prev.link = d.link
					}
					newd := d.link
					freedefer(d)
					d = newd
				} else {
					prev = d
					d = d.link
				}
			}

			gp._panic = p.link
			// Aborted panics are marked but remain on the g.panic list.
			// Remove them from the list.
			for gp._panic != nil && gp._panic.aborted {
				gp._panic = gp._panic.link
			}
			if gp._panic == nil { // must be done with signal
				gp.sig = 0
			}
			// Pass information about recovering frame to recovery.
			gp.sigcode0 = uintptr(sp)
			gp.sigcode1 = pc
			/*
				recovery完成恢复职责：
				1. 判断当前 _panic 中recover是否已被标注为处理
				2. 从 _panic 链表中删除已标注中止的panic事件，也就是删除已被恢复的panic事件
				3. 将相关需要恢复的栈帧信息传递给recovery方法的gp参数（每个栈帧对应着一个未运行完的函数。栈帧中保存了该函数的返回地址和局部变量）
				4. 执行recovery进行恢复动作
			*/
			mcall(recovery)
			throw("recovery failed") // mcall should not return
		}
	}

	// 消耗完所以的defer调用，保守地进行panic
	// 因为在冻结之后调用任意用户代码是不安全的，所以我们调用preprintpanics来调用
	// 所以必要的Error和String方法来在startpanic之前准备panic字符串。
	preprintpanics(gp._panic)

	fatalpanic(gp._panic) // should not return	// 不应该返回
	*(*int)(nil) = 0      // not reached		// 无法触及
}
```
### 4、gorecover
```go
/*
gopanic方法会遍历调用当前 Goroutine 下的defer链表，若refectcall执行中遇到recover就会调用gorecover进行处理：
*/
func gorecover(argp uintptr) interface{} {
	gp := getg()
	p := gp._panic
	if p != nil && !p.goexit && !p.recovered && argp == uintptr(p.argp) {
		p.recovered = true
		return p.arg
	}
	return nil
}

```