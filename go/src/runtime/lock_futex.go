// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build dragonfly || freebsd || linux
// +build dragonfly freebsd linux

package runtime

import (
	"runtime/internal/atomic"
	"unsafe"
)

/*
计算机知识：
在开发中，我们会接触或碰到各种锁，但最底层的两种是 互斥锁（这边的互斥锁不是go中sync.Mutex） 和 自旋锁，而其他锁都是基于这两种之上实现的

互斥锁与自旋锁的本质区别：一般互斥锁会在等待期间放弃cpu，自旋锁（spinlock）则是不断循环并测试锁的状态，这样会一直占着cpu.线程在申请自旋锁的时候，线程不会被挂起，而是处于忙等的状态。

* 互斥锁：用于保护临界区，确保同一时间只有一个线程访问数据。堆共享资源的访问，先对互斥量进行加锁，如果互斥量已经上锁，调用线程会阻塞，直到互斥量被解锁。堆完成了对共享资源的访问后，要对互斥量进行解锁。
* 自旋锁：与互斥锁类似，它不是通过休眠使进程阻塞，而是在获取锁之前一直处于忙等(自旋)阻塞状态。用在以下情况：锁持有的时间短，而且线程并不希望在重新调度上花太多的成本。
* 信号量：信号量是一个计数器，可以用来控制多个进程对共享资源的访问。它常作为一种锁机制，防止某进程正在访问共享资源时，其他进程也访问该资源。因此，主要作为进程间以及同一进程内不同线程之间的同步手段。

golang中的mutex是一个混合锁，优先判断是否自旋，不行才会使用互斥锁。
*/

// This implementation depends on OS-specific implementations of
// 这个实现取决于操作系统对以下内容的具体实现
//
//	futexsleep(addr *uint32, val uint32, ns int64)
//		Atomically,
// 		是原子操作，
//			if *addr == val { sleep }
//		Might be woken up spuriously; that's allowed.
// 		可能会被假性唤醒；这是被允许的。
//		Don't sleep longer than ns; ns < 0 means forever.
// 		不要让睡眠时间超过ns；ns < 0意味着永远。
//
//	futexwakeup(addr *uint32, cnt uint32)
//		If any procs are sleeping on addr, wake up at most cnt.
// 		如果有程序在addr上sleeping，最多可以唤醒cnt。

const (
	mutex_unlocked = 0
	mutex_locked   = 1
	mutex_sleeping = 2

	active_spin     = 4  // 最大自旋4次
	active_spin_cnt = 30 // 每次30个cpu时钟周期
	passive_spin    = 1
)

// Possible lock states are mutex_unlocked, mutex_locked and mutex_sleeping.
// mutex_sleeping means that there is presumably at least one sleeping thread.
// Note that there can be spinning threads during all states - they do not
// affect mutex's state.
// 可能的锁定状态是mutex_unlocked、mutex_locked和mutex_sleeping。
// mutex_sleeping意味着可能至少有一个睡眠线程。
// 注意，在所有状态下都可以有旋转的线程--它们不影响mutex的状态。

// We use the uintptr mutex.key and note.key as a uint32.
// 我们使用uintptr mutex.key和note.key作为一个uint32。
//go:nosplit
func key32(p *uintptr) *uint32 {
	return (*uint32)(unsafe.Pointer(p))
}

func lock(l *mutex) {
	lockWithRank(l, getLockRank(l))
}

func lock2(l *mutex) {
	// 获取当前运行的协程
	gp := getg()

	if gp.m.locks < 0 {
		throw("runtime·lock: lock count") // 运行时锁定：锁定计数
	}
	/*
		当前m的锁加一
	*/
	gp.m.locks++

	// Speculative grab for lock.
	// 投机性地抢夺锁。
	/*
		func Xchg(ptr *uint32, new uint32) uint32
		将new值赋值给ptr所指向变量，并返回赋值前的ptr所指向变量值
	*/
	v := atomic.Xchg(key32(&l.key), mutex_locked)
	/*
		如果l.key是从未锁变成锁定，则直接返回
	*/
	if v == mutex_unlocked {
		return
	}

	/*
		上面的表示获取锁成功，如果没有获取成功，可能是因为有协程已经获取到锁，或在sleeping状态
		也就是：这个锁处于，MUTEX_LOCKED or MUTEX_SLEEPING
	*/
	// wait is either MUTEX_LOCKED or MUTEX_SLEEPING
	// depending on whether there is a thread sleeping
	// on this mutex. If we ever change l->key from
	// MUTEX_SLEEPING to some other value, we must be
	// careful to change it back to MUTEX_SLEEPING before
	// returning, to ensure that the sleeping thread gets
	// its wakeup call.
	// wait是MUTEX_LOCKED或MUTEX_SLEEPING，这取决于是否有线程在这个互斥上睡眠。
	// 如果我们把l->key从MUTEX_SLEEPING改成其他的值，我们必须注意在返回之前把它
	// 改回MUTEX_SLEEPING，以确保睡眠线程得到它的唤醒。
	wait := v

	// On uniprocessors, no point spinning.
	// On multiprocessors, spin for ACTIVE_SPIN attempts.
	// 在单核处理器上，没有必要进行旋转。
	// 在多处理器上，为ACTIVE_SPIN的尝试而旋转
	spin := 0
	if ncpu > 1 {
		spin = active_spin
	}
	for {
		// Try for lock, spinning.
		// 尝试锁定，旋转。
		for i := 0; i < spin; i++ {
			for l.key == mutex_unlocked {
				if atomic.Cas(key32(&l.key), mutex_unlocked, wait) {
					return
				}
			}
			procyield(active_spin_cnt)
		}

		// Try for lock, rescheduling.
		// 尝试锁定，重新安排时间。
		for i := 0; i < passive_spin; i++ {
			/*
				如果未锁定，尝试
			*/
			for l.key == mutex_unlocked {
				/*
					尝试锁定，如果锁定成功，则直接返回
				*/
				if atomic.Cas(key32(&l.key), mutex_unlocked, wait) {
					return
				}
			}
			/*
				尝试重新调度
			*/
			osyield()
		}

		// Sleep.
		/*
			进入sleep

			在之前，经历了 尝试抢锁，尝试自旋，尝试CAS，osyield()都没有获取到锁，则进入休眠状态
		*/
		v = atomic.Xchg(key32(&l.key), mutex_sleeping)
		if v == mutex_unlocked {
			return
		}
		wait = mutex_sleeping
		futexsleep(key32(&l.key), mutex_sleeping, -1)
	}
}

func unlock(l *mutex) {
	unlockWithRank(l)
}

func unlock2(l *mutex) {
	v := atomic.Xchg(key32(&l.key), mutex_unlocked)
	if v == mutex_unlocked {
		throw("unlock of unlocked lock")
	}
	if v == mutex_sleeping {
		futexwakeup(key32(&l.key), 1)
	}

	gp := getg()
	gp.m.locks--
	if gp.m.locks < 0 {
		throw("runtime·unlock: lock count")
	}
	if gp.m.locks == 0 && gp.preempt { // restore the preemption request in case we've cleared it in newstack
		gp.stackguard0 = stackPreempt
	}
}

// One-time notifications.
// 一次性通知。
func noteclear(n *note) {
	n.key = 0
}

func notewakeup(n *note) {
	old := atomic.Xchg(key32(&n.key), 1)
	if old != 0 {
		print("notewakeup - double wakeup (", old, ")\n")
		throw("notewakeup - double wakeup")
	}
	futexwakeup(key32(&n.key), 1)
}

func notesleep(n *note) {
	gp := getg()
	if gp != gp.m.g0 {
		throw("notesleep not on g0")
	}
	ns := int64(-1)
	if *cgo_yield != nil {
		// Sleep for an arbitrary-but-moderate interval to poll libc interceptors.
		ns = 10e6
	}
	for atomic.Load(key32(&n.key)) == 0 {
		gp.m.blocked = true
		futexsleep(key32(&n.key), 0, ns)
		if *cgo_yield != nil {
			asmcgocall(*cgo_yield, nil)
		}
		gp.m.blocked = false
	}
}

// May run with m.p==nil if called from notetsleep, so write barriers
// are not allowed.
// 如果从notetsleep调用，可能会在m.p==nil的情况下运行，所以不允许写障碍。
//
//go:nosplit
//go:nowritebarrier
func notetsleep_internal(n *note, ns int64) bool {
	gp := getg()

	if ns < 0 {
		if *cgo_yield != nil {
			// Sleep for an arbitrary-but-moderate interval to poll libc interceptors.
			ns = 10e6
		}
		for atomic.Load(key32(&n.key)) == 0 {
			gp.m.blocked = true
			futexsleep(key32(&n.key), 0, ns)
			if *cgo_yield != nil {
				asmcgocall(*cgo_yield, nil)
			}
			gp.m.blocked = false
		}
		return true
	}

	if atomic.Load(key32(&n.key)) != 0 {
		return true
	}

	deadline := nanotime() + ns
	for {
		if *cgo_yield != nil && ns > 10e6 {
			ns = 10e6
		}
		gp.m.blocked = true
		futexsleep(key32(&n.key), 0, ns)
		if *cgo_yield != nil {
			asmcgocall(*cgo_yield, nil)
		}
		gp.m.blocked = false
		if atomic.Load(key32(&n.key)) != 0 {
			break
		}
		now := nanotime()
		if now >= deadline {
			break
		}
		ns = deadline - now
	}
	return atomic.Load(key32(&n.key)) != 0
}

func notetsleep(n *note, ns int64) bool {
	gp := getg()
	if gp != gp.m.g0 && gp.m.preemptoff != "" {
		throw("notetsleep not on g0")
	}

	return notetsleep_internal(n, ns)
}

// same as runtime·notetsleep, but called on user g (not g0)
// calls only nosplit functions between entersyscallblock/exitsyscall
func notetsleepg(n *note, ns int64) bool {
	gp := getg()
	if gp == gp.m.g0 {
		throw("notetsleepg on g0")
	}

	entersyscallblock()
	ok := notetsleep_internal(n, ns)
	exitsyscall()
	return ok
}

func beforeIdle(int64, int64) (*g, bool) {
	return nil, false
}

func checkTimeouts() {}
