// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"internal/race"
	"sync/atomic"
	"unsafe"
)

// There is a modified copy of this file in runtime/rwmutex.go.
// If you make any changes here, see if you should make them there.
// 在runtime/rwmutex.go中有这个文件的一个修改副本。
// 如果你在这里做了任何修改，看看你是否应该在那里做。

// A RWMutex is a reader/writer mutual exclusion lock.
// The lock can be held by an arbitrary number of readers or a single writer.
// The zero value for a RWMutex is an unlocked mutex.
// 一个RWMutex是一个读写互斥锁。
// 该锁可以由任意数量的读者或单一的写者持有。
// RWMutex的零值是一个解锁的mutex。
//
// A RWMutex must not be copied after first use.
// 一个RWMutex在第一次使用后不能被复制。
//
// If a goroutine holds a RWMutex for reading and another goroutine might
// call Lock, no goroutine should expect to be able to acquire a read lock
// until the initial read lock is released. In particular, this prohibits
// recursive read locking. This is to ensure that the lock eventually becomes
// available; a blocked Lock call excludes new readers from acquiring the
// lock.
// 如果一个goroutine持有一个RWMutex用于读取，而另一个goroutine可能会调用Lock，
// 那么任何goroutine都不应该期望能够获得一个读锁，直到最初的读锁被释放。特别是，
// 这禁止了递归读锁。这是为了确保锁最终是可用的；一个阻塞的Lock调用排除了新的读获取锁的可能性。
type RWMutex struct {
	w           Mutex  // held if there are pending writers						// 如果有待处理的写，则保留。
	writerSem   uint32 // semaphore for writers to wait for completing readers	// 写等待 读完成的信号。
	readerSem   uint32 // semaphore for readers to wait for completing writers	// 读等待 写完成的信号。
	readerCount int32  // number of pending readers								// 待处理的读数量
	readerWait  int32  // number of departing readers							// 离开的读数量
}

const rwmutexMaxReaders = 1 << 30

// Happens-before relationships are indicated to the race detector via:
// Happens-before关系通过以下方式表示给竞赛检测器。
// - Unlock  -> Lock:  readerSem
// - Unlock  -> RLock: readerSem
// - RUnlock -> Lock:  writerSem
//
// The methods below temporarily disable handling of race synchronization
// events in order to provide the more precise model above to the race
// detector.
// 下面的方法暂时禁止处理竞赛同步事件，以便向竞赛检测器提供上述更精确的模型。
//
// For example, atomic.AddInt32 in RLock should not appear to provide
// acquire-release semantics, which would incorrectly synchronize racing
// readers, thus potentially missing races.
// 例如，RLock中的atomic.AddInt32不应该出现提供获取-释放的语义，这将不正确地同步赛车的读，从而有可能错过races。

// RLock locks rw for reading.
// RLock锁住rw进行读取。
//
// It should not be used for recursive read locking; a blocked Lock
// call excludes new readers from acquiring the lock. See the
// documentation on the RWMutex type.
// 它不应该被用于递归读锁；一个阻塞的Lock调用排除了新的读者获取锁。参见RWMutex类型的文档。
/*
	读锁：
	1、
*/
func (rw *RWMutex) RLock() {
	if race.Enabled {
		_ = rw.w.state
		race.Disable()
	}
	if atomic.AddInt32(&rw.readerCount, 1) < 0 {
		// A writer is pending, wait for it.
		// 如果readerCount小于-1则通过同步原语阻塞住，否则将readerCount加1后即返回
		// A writer is pending, wait for it.
		// 一个writer正在等待，请等待。
		runtime_SemacquireMutex(&rw.readerSem, false, 0)
	}
	if race.Enabled {
		race.Enable()
		race.Acquire(unsafe.Pointer(&rw.readerSem))
	}
}

// RUnlock undoes a single RLock call;
// it does not affect other simultaneous readers.
// It is a run-time error if rw is not locked for reading
// on entry to RUnlock.
// RUnlock撤消单个RLock调用；它不影响其他同时进行的读。
// 如果rw在进入RUnlock时没有被锁定用于读取，则是一个运行时错误。
func (rw *RWMutex) RUnlock() {
	// 是否开启检测race
	if race.Enabled {
		_ = rw.w.state
		race.ReleaseMerge(unsafe.Pointer(&rw.writerSem))
		race.Disable()
	}
	if r := atomic.AddInt32(&rw.readerCount, -1); r < 0 {
		// Outlined slow-path to allow the fast-path to be inlined
		// 如果readerCount减1后小于0，则调用rUnlockSlow方法，将这个方法剥离出来是为了RUnlock可以内联，这样能进一步提升读操作时的取锁性能
		rw.rUnlockSlow(r)
	}
	// 是否开启检测race
	if race.Enabled {
		race.Enable()
	}
}

func (rw *RWMutex) rUnlockSlow(r int32) {
	if r+1 == 0 || r+1 == -rwmutexMaxReaders {
		race.Enable()
		throw("sync: RUnlock of unlocked RWMutex")
	}
	// A writer is pending.
	// 一个读正在等待
	if atomic.AddInt32(&rw.readerWait, -1) == 0 {
		// The last reader unblocks the writer.
		// 最后一个读解除对写的锁定。
		runtime_Semrelease(&rw.writerSem, false, 1)
	}
}

// Lock locks rw for writing.
// If the lock is already locked for reading or writing,
// Lock blocks until the lock is available.
// Lock锁住rw进行写入。
// 如果该锁已经被锁定用于读或写，Lock就会阻止，直到该锁可用。
/*
	写锁上锁：
	1、阻塞新来的写操作（尝试上互斥锁，如果处于等待，则说明正在写操作）
	2、阻塞新来的读操作
	3、等待之前的读操作完成
*/
func (rw *RWMutex) Lock() {
	// 是否开启检测race
	if race.Enabled {
		_ = rw.w.state
		race.Disable()
	}
	// First, resolve competition with other writers.
	// 首先，解决与其他写的竞争。
	rw.w.Lock()
	// Announce to readers there is a pending writer.
	/*
		    将readerCount减去一个最大数（2的30次方，RWMutex能支持的最大同时读操作数），这样readerCount将变成一个小于0的很小的数，
		    后续再调RLock方法时将会因为readerCount<0而阻塞住，这样也就阻塞住了新来的读请求
			理解，假设现在有一个写操作进来，在执行AddInt32那一刻rw.readerCount=10，执行完r=10，rw.readerCount=-1<<30 + 10，
			此时 r != 0 && atomic.AddInt32(&rw.readerWait, r) != 0 为false，那么就会等待已读的完成
			又因为rw.readerCount=-1<<30 + 10会远小于0，此刻想读的在执行RLock方法时，被阻塞住，也就是实现了阻塞新来的读请求
	*/
	r := atomic.AddInt32(&rw.readerCount, -rwmutexMaxReaders) + rwmutexMaxReaders
	// Wait for active readers.
	// 等待之前的读操作完成
	if r != 0 && atomic.AddInt32(&rw.readerWait, r) != 0 {
		runtime_SemacquireMutex(&rw.writerSem, false, 0)
	}
	// 是否开启检测race
	if race.Enabled {
		race.Enable()
		race.Acquire(unsafe.Pointer(&rw.readerSem))
		race.Acquire(unsafe.Pointer(&rw.writerSem))
	}
}

// Unlock unlocks rw for writing. It is a run-time error if rw is
// not locked for writing on entry to Unlock.
// Unlock 解除对rw的写入锁定。如果rw在进入Unlock时没有被锁定以便写入，则是一个运行时错误。
//
// As with Mutexes, a locked RWMutex is not associated with a particular
// goroutine. One goroutine may RLock (Lock) a RWMutex and then
// arrange for another goroutine to RUnlock (Unlock) it.
// 与Mutexes一样，一个被锁定的RWMutex并不与一个特定的goroutine相关联。
// 一个goroutine可以RLock（锁定）一个RWMutex，然后安排另一个goroutine来RUnlock（解锁）它。
func (rw *RWMutex) Unlock() {
	// 是否开启检测race
	if race.Enabled {
		_ = rw.w.state
		race.Release(unsafe.Pointer(&rw.readerSem))
		race.Disable()
	}

	// Announce to readers there is no active writer.
	r := atomic.AddInt32(&rw.readerCount, rwmutexMaxReaders)
	if r >= rwmutexMaxReaders {
		race.Enable()
		throw("sync: Unlock of unlocked RWMutex")
	}
	// Unblock blocked readers, if any.
	for i := 0; i < int(r); i++ {
		runtime_Semrelease(&rw.readerSem, false, 0)
	}
	// Allow other writers to proceed.
	// 释放互斥锁
	rw.w.Unlock()
	// 是否开启检测race
	if race.Enabled {
		race.Enable()
	}
}

// RLocker returns a Locker interface that implements
// the Lock and Unlock methods by calling rw.RLock and rw.RUnlock.
// RLocker返回一个Locker接口，通过调用rw.RLock和rw.RUnlock实现锁定和解锁方法。
func (rw *RWMutex) RLocker() Locker {
	return (*rlocker)(rw)
}

type rlocker RWMutex

func (r *rlocker) Lock()   { (*RWMutex)(r).RLock() }
func (r *rlocker) Unlock() { (*RWMutex)(r).RUnlock() }
