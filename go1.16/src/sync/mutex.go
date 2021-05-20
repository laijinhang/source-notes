// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sync provides basic synchronization primitives such as mutual
// exclusion locks. Other than the Once and WaitGroup types, most are intended
// for use by low-level library routines. Higher-level synchronization is
// better done via channels and communication.
//
// Values containing the types defined in this package should not be copied.
package sync

import (
	"internal/race"
	"sync/atomic"
	"unsafe"
)

func throw(string) // provided by runtime

// Mutex是互斥锁。
// 互斥锁的零值是未锁定的互斥锁。
//
// 首次使用后不得复制Mutex。
type Mutex struct {
	// state是一个共用的字段
	// 从左到右的 第 0个bit位 标记 mutex 是否被某个协程占用，也就是有没有加锁
	// 从左到右的 第 1个bit位 标记 mutex 是否被唤醒，就是某个被唤醒的mutex尝试去获取锁
	// 从左到右的 第 2个bit位 标记 mutex 是否饥饿状态
	// 剩下的bit位表示 waiter 的个数，最大允许记录 1<<(32-3) 个协程
	// [阻塞的goroutine个数, starving标识, woken标识, locked标识]
	state int32

	sema uint32
}

// A Locker represents an object that can be locked and unlocked.
type Locker interface {
	Lock()
	Unlock()
}

const (
	mutexLocked   = 1 << iota // mutex is locked	锁定状态
	mutexWoken                // 2	唤醒状态
	mutexStarving             // 4	饥饿状态
	// 偏移量，用于计算等待获取锁的协程数，用法是state>>=mutexWaiterShift之后，
	// state的值就表示当前阻塞等待锁的goroutine个数。最多可以阻塞2^29个goroutine。
	mutexWaiterShift = iota // 3
	// 互斥公平.
	//
	// Mutex有2种操作模式: 正常模式 和 饥饿模式.
	// 在正常模式下，等待着waiter会进入到一个FIFO队列，在获取锁时waiter会按照先进先出的顺序获取。
	// 当唤醒一个waiter时，它并不会立即获取锁，而是继续与新来的协程竞争，这种情况下新来的协程比较有优势，
	// 主要是因为它已经运行在CPU，可能这种的数量还不少，所以waiter大概率下获取不到锁。在这种waiter获取
	// 不到锁的情况下，waiter会被添加到队列的前面。如果waiter获取不到锁的时间超过1毫秒，它将被切换到饥饿模式。
	// 这里的waiter是指新来的协程尝试一次获取锁，如果获取不到我们就视其为waiter，并将其添加到FIFO队列里。
	//
	// 在饥饿模式下，锁将直接交给队列最前面的waiter。新来的协程即使在锁未被挟持情况下也不会参与竞争锁，
	// 同时也不会进行自旋，而直接将其添加到队列的尾部。
	//
	// 如果拥有锁的waiter发现有以下两种情况，它将切回到正常模式：
	// 1.它是队列里的最后一个waiter，再也没有其它waiter
	// 2.等待时间小于1毫秒
	//
	// 正常模式拥有更好的性能，因为即使有等待抢锁的waiter，协程也可以连续多次获取到锁。
	// 饥饿模式锁公平性和性能的一种平衡，它避免了某些协程长时间的等待锁。
	// 在饥饿模式下，优先处理的是那些在一直等待的waiter。饥饿模式在一定时机会切换回正常模式。
	starvationThresholdNs = 1e6 // 1毫秒，用来与waiter的等待时间做比较
)

// Lock locks m.
// If the lock is already in use, the calling goroutine
// blocks until the mutex is available.
func (m *Mutex) Lock() {
	// Fast path: grab unlocked mutex.
	// 如果mutex的state没有被锁，也没有等待/唤醒的协程，锁处于正常状态（未上锁），那么获得锁，返回
	// 原子操作，如果m.state未上锁，也就是值为0，则上锁，并返回true/false（设置成功，则true，否则false）

	/*
		如果这边能直接拿到锁，则在这边拿到，拿不到的话，就执行后面的m.lockSlow()方法尝试拿锁
	*/
	if atomic.CompareAndSwapInt32(&m.state, 0, mutexLocked) {
		// 看里面内容，下面三行代码什么也没做，估计是待实现内容
		if race.Enabled {
			race.Acquire(unsafe.Pointer(m))
		}
		return
	}
	// Slow path (outlined so that the fast path can be inlined)
	// 尝试自旋竞争或饥饿状态下饥饿协程竞争
	m.lockSlow()
}

func (m *Mutex) lockSlow() {
	var waitStartTime int64 // 当前 waiter开始等待时间
	starving := false       // 当前饥饿状态
	awoke := false          // 当前唤醒状态
	iter := 0               // 当前自旋次数
	old := m.state          // 当前锁的状态
	for {
		// 在饥饿模式下，直接将锁移交给waiter（队列头部的waiter）因此新来的协程永远也不会获取锁
		// 在正常模式，锁被其他协程持有，如果允spinning，则尝试自旋
		/*
			如果当前锁的状态是锁定或饥饿的话，也就是锁定状态时，则尝试自旋
		*/
		if old&(mutexLocked|mutexStarving) == mutexLocked && runtime_canSpin(iter) {
			// Active spinning makes sense.
			// Try to set mutexWoken flag to inform Unlock
			// to not wake other blocked goroutines.
			if !awoke && old&mutexWoken == 0 && old>>mutexWaiterShift != 0 &&
				atomic.CompareAndSwapInt32(&m.state, old, old|mutexWoken) {
				awoke = true // 设置当前协程唤醒成功
			}
			runtime_doSpin() // 自旋
			iter++           // 当前自旋次数加1
			old = m.state    // 当前协程再次获取锁的状态，之后会检查是否锁被释放了
			continue
		}
		// 当前协程的m.state新状态
		new := old
		// Don't try to acquire starving mutex, new arriving goroutines must queue.
		// 如果处于正常模式，则加锁
		if old&mutexStarving == 0 {
			new |= mutexLocked
		}
		// 当前如果是处于饥饿模式，则更新waiters数量 +1
		if old&(mutexLocked|mutexStarving) != 0 {
			// 从左到右三位分别被 是否上锁 是否唤醒 是否处于饥饿状态 标识位所占用
			new += 1 << mutexWaiterShift
		}
		// The current goroutine switches mutex to starvation mode.
		// But if the mutex is currently unlocked, don't do the switch.
		// Unlock expects that starving mutex has waiters, which will not
		// be true in this case.
		// 当前协程处于饥饿状态，且锁被其它协程持有，新状态则更新锁为饥饿模式
		if starving && old&mutexLocked != 0 {
			new |= mutexStarving
		}
		// 如果当前协程的waiter被唤醒，则重置flag
		if awoke {
			// The goroutine has been woken from sleep,
			// so we need to reset the flag in either case.
			// 唤醒状态不一致，直接抛出异常
			if new&mutexWoken == 0 {
				throw("sync: inconsistent mutex state")
			}
			// 新状态清除唤醒标记
			new &^= mutexWoken
		}
		if atomic.CompareAndSwapInt32(&m.state, old, new) {
			// 1、锁已经被释放且为正常模式
			if old&(mutexLocked|mutexStarving) == 0 {
				// 通过 CSA 获取了锁，直接终止，返回
				break // locked the mutex with CAS
			}
			// If we were already waiting before, queue at the front of the queue.
			// 如果我们之前已经在等待，就在队列的前面排队。
			// 2、到这里是没有获取到锁，判断一下等待时长是否不为0
			//	如果不为0，那么加入到队列头部
			queueLifo := waitStartTime != 0
			// 3、如果等待时间为0，那么初始化等待时间
			if waitStartTime == 0 {
				// 获取运行时时钟的当前值，单位为纳秒
				waitStartTime = runtime_nanotime()
			}
			// 4、阻塞等待
			runtime_SemacquireMutex(&m.sema, queueLifo, 1)
			// 5、唤醒之后检查锁是否应该处于饥饿状态
			starving = starving || runtime_nanotime()-waitStartTime > starvationThresholdNs
			old = m.state
			// 6、判断是否已经处于饥饿状态
			if old&mutexStarving != 0 {
				// If this goroutine was woken and mutex is in starvation mode,
				// ownership was handed off to us but mutex is in somewhat
				// inconsistent state: mutexLocked is not set and we are still
				// accounted as waiter. Fix that.
				if old&(mutexLocked|mutexWoken) != 0 || old>>mutexWaiterShift == 0 {
					throw("sync: inconsistent mutex state")
				}
				// 7、加锁并且将waiter数减1
				delta := int32(mutexLocked - 1<<mutexWaiterShift)
				if !starving || old>>mutexWaiterShift == 1 {
					// Exit starvation mode.
					// Critical to do it here and consider wait time.
					// Starvation mode is so inefficient, that two goroutines
					// can go lock-step infinitely once they switch mutex
					// to starvation mode.
					// 8、如果当前goroutine不是饥饿状态，则从饥饿模式切换回正常模式
					delta -= mutexStarving
				}
				// 9、设置状态
				atomic.AddInt32(&m.state, delta)
				break
			}
			awoke = true
			iter = 0
		} else {
			old = m.state
		}
	}

	if race.Enabled {
		race.Acquire(unsafe.Pointer(m))
	}
}

// Unlock unlocks m.
// It is a run-time error if m is not locked on entry to Unlock.
//
// A locked Mutex is not associated with a particular goroutine.
// It is allowed for one goroutine to lock a Mutex and then
// arrange for another goroutine to unlock it.
func (m *Mutex) Unlock() {
	if race.Enabled {
		_ = m.state
		race.Release(unsafe.Pointer(m))
	}

	// Fast path: drop lock bit.
	// 解锁
	new := atomic.AddInt32(&m.state, -mutexLocked)
	if new != 0 {
		// Outlined slow path to allow inlining the fast path.
		// To hide unlockSlow during tracing we skip one extra frame when tracing GoUnblock.
		// 如果还存在 唤醒、饥饿、等待锁 的协程
		m.unlockSlow(new)
	}
}

func (m *Mutex) unlockSlow(new int32) {
	if (new+mutexLocked)&mutexLocked == 0 {
		throw("sync: unlock of unlocked mutex")
	}
	// 如果不存在饥饿状态的等待锁的
	if new&mutexStarving == 0 {
		old := new
		for {
			// If there are no waiters or a goroutine has already
			// been woken or grabbed the lock, no need to wake anyone.
			// In starvation mode ownership is directly handed off from unlocking
			// goroutine to the next waiter. We are not part of this chain,
			// since we did not observe mutexStarving when we unlocked the mutex above.
			// So get off the way.
			/*
				如果没有 waiter（等待获取锁的），或者已经有在处理（拿到了锁，唤醒状态、饥饿状态）的情况，直接返回
			 */
			if old>>mutexWaiterShift == 0 || old&(mutexLocked|mutexWoken|mutexStarving) != 0 {
				return
			}
			// Grab the right to wake someone.
			// waiter数减1，mutexWoken标识设置上，通过CAS更新state的值
			new = (old - 1<<mutexWaiterShift) | mutexWoken
			if atomic.CompareAndSwapInt32(&m.state, old, new) {
				// 直接唤醒等待队列中的 waiter
				runtime_Semrelease(&m.sema, false, 1)
				return
			}
			old = m.state
		}
	} else {
		// Starving mode: handoff mutex ownership to the next waiter, and yield
		// our time slice so that the next waiter can start to run immediately.
		// Note: mutexLocked is not set, the waiter will set it after wakeup.
		// But mutex is still considered locked if mutexStarving is set,
		// so new coming goroutines won't acquire it.
		/*
			饥饿模式
			直接唤醒等待队列中的
		 */
		runtime_Semrelease(&m.sema, true, 1)
	}
}
