// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"internal/race"
	"sync/atomic"
	"unsafe"
)

// A WaitGroup waits for a collection of goroutines to finish.
// The main goroutine calls Add to set the number of
// goroutines to wait for. Then each of the goroutines
// runs and calls Done when finished. At the same time,
// Wait can be used to block until all goroutines have finished.
// WaitGroup用于等待一组 Goroutine 执行完毕。
// 主 Goroutine 调用 Add 来设置需要等待的 Goroutine 的数量
// 然后每个 Goroutine 运行并调用 Done 来确定已经执行完毕
// 同时，Wait 可以用来阻塞并等待所有的 Goroutine 完成。
//
// A WaitGroup must not be copied after first use.
// WaitGroup 在第一次使用后不能被复制
type WaitGroup struct {
	noCopy noCopy

	// 64-bit value: high 32 bits are counter, low 32 bits are waiter count.
	// 64-bit atomic operations require 64-bit alignment, but 32-bit
	// compilers do not ensure it. So we allocate 12 bytes and then use
	// the aligned 8 bytes in them as state, and the other 4 as storage
	// for the sema.
	// 64-bit 值：高 32 位用于计数，低 32 位用于等待计数
	// 64-bit 的原子操作要求 64 位对齐，但 32 位编译器无法保证这个要求
	// 因此分配 12 字节然后将他们对齐，其中 8 字节作为状态，其他 4 字节用于存储原语
	state1 [3]uint32
}

// state returns pointers to the state and sema fields stored within wg.state1.
// state 返回 wg.state1 中存储的状态和原语字段
func (wg *WaitGroup) state() (statep *uint64, semap *uint32) {
	/*
			在32位机器上 state1[1] 和 state1[2] 分别用于计数和等待计数，而第一个 state1[0] 用于存储原语。
			在64位机器上 state1[0] 和 state1[1] 分别用于计数和等待计数，而最后一个 state1[2] 用于存储原语。
		个人理解：
		1. 内存对齐
		一、32位架构中，一个字长是4bytes，要操作64位的数据，需要从 两个数据块 中操作，而每次只能操作一块，需要操作两次，
		在这两次操作中可能有其他操作修改，不能保证原子性。
		假设起始位置为0:
		第一个位置被某个变量占用（因为是32位机器，可以假设这个变量是占用四个字节），之后从第四个字节开始，此时uintptr(unsafe.Pointer(&wg.state1))%8会等于4,这时为了返回8字节的statep，
		拿state1[0]来填充
		二、对于64位架构中，一个字长是8bytes，是8字节对齐的，会自动把wg.state1[0]和wg.state1[1]合并成8字节的uint64，wg.state1[2]虽然占4个字节，但是会操作的时候，会自动填充4字节起到8字节
		对齐
		如果wg.state1[0]用于存储原语，wg.state1[1]和wg.state1[2]用于statep，因为64位机器每次操作8字节，wg.state1[0]和wg.state1[1]在操作的时候被合并了，state跨两个8字节块了，因此
		会进行两次操作，不能保证到原子操作
		内存对齐知识可以看这个博客：
		1. https://www.cnblogs.com/luozhiyun/p/14289034.html
		2. https://zhuanlan.zhihu.com/p/106933470
	*/
	if uintptr(unsafe.Pointer(&wg.state1))%8 == 0 {
		// 如果地址是64位对齐的，wg.state1前两个合并做state，第三个做信号量
		return (*uint64)(unsafe.Pointer(&wg.state1)), &wg.state1[2]
	} else {
		// 如果当前地址是32位对齐，则第一个做信号量，后面两个合并做state
		return (*uint64)(unsafe.Pointer(&wg.state1[1])), &wg.state1[0]
	}
}

// Add adds delta, which may be negative, to the WaitGroup counter.
// If the counter becomes zero, all goroutines blocked on Wait are released.
// If the counter goes negative, Add panics.
// Add 将 delta（可能为负）加到 WaitGroup 的计数器上
// 如果计数器归零，则所有阻塞在 Wait 的 Goroutine 被释放
// 如果计数器为负，则panic
//
// Note that calls with a positive delta that occur when the counter is zero
// must happen before a Wait. Calls with a negative delta, or calls with a
// positive delta that start when the counter is greater than zero, may happen
// at any time.
// Typically this means the calls to Add should execute before the statement
// creating the goroutine or other event to be waited for.
// If a WaitGroup is reused to wait for several independent sets of events,
// new Add calls must happen after all previous Wait calls have returned.
// See the WaitGroup example.
// 请注意，当计数器为 0 时发生的带有正的 delta 的调用必须在 Wait 之前。
// 当计数器大于 0 时，带有负 delta 的调用或带有正 delta 调用可能在任何时候发生。
// 通常，这意味着 Add 调用必须发生在 Goroutine 创建之前或被其他等待事件之前。
// 如果一个 WaitGroup 被复用于等待几个不同的独立事件集合，必须在前一个 Wait 调用返回后才能调用 Add。
func (wg *WaitGroup) Add(delta int) {
	// 首先获取状态指针和存储指针
	statep, semap := wg.state()
	if race.Enabled {
		_ = *statep // trigger nil deref early
		if delta < 0 {
			// Synchronize decrements with Wait.
			race.ReleaseMerge(unsafe.Pointer(wg))
		}
		race.Disable()
		defer race.Enable()
	}
	// 将 delta 加到 statep 的前 32 位上，即加到计数器上
	state := atomic.AddUint64(statep, uint64(delta)<<32)
	// 计数器的值
	v := int32(state >> 32)
	// 等待器的值
	w := uint32(state)
	if race.Enabled && delta > 0 && v == int32(delta) {
		// The first increment must be synchronized with Wait.
		// Need to model this as a read, because there can be
		// several concurrent wg.counter transitions from 0.
		race.Read(unsafe.Pointer(semap))
	}
	// 如果实际计数为负则直接 panic，因为不允许计数为负值的
	if v < 0 {
		panic("sync: negative WaitGroup counter")
	}
	// 如果等待器不为零，但 delta 是处于增加的状态，而且存储计数与 delta 的值相同，则立即 panic
	if w != 0 && delta > 0 && v == int32(delta) {
		panic("sync: WaitGroup misuse: Add called concurrently with Wait")
	}
	// 如果计数器 > 0 或等待器为 0 则一切都很好，直接返回
	if v > 0 || w == 0 {
		return
	}
	// This goroutine has set counter to 0 when waiters > 0.
	// Now there can't be concurrent mutations of state:
	// - Adds must not happen concurrently with Wait,
	// - Wait does not increment waiters if it sees counter == 0.
	// Still do a cheap sanity check to detect WaitGroup misuse.
	// 这时 Goroutine 已经将计数器清零，且等待器大于零（并发调用导致）
	// 这时不允许出现并发使用导致的状态突变，否则就应该panic
	// - Add 不能与 Wait 并发调用
	// - Wait 在计数器已经归零的情况下，不能再继续增加等待器了
	// 仍然检查来保证 WaitGroup 不会被滥用
	if *statep != state {
		panic("sync: WaitGroup misuse: Add called concurrently with Wait")
	}
	// Reset waiters count to 0.
	// 结束后将等待器清零
	*statep = 0
	// 等待器大于零，减少 runtime_Semrelease 产生的阻塞
	for ; w != 0; w-- {
		runtime_Semrelease(semap, false, 0)
	}
}

// Done decrements the WaitGroup counter by one.
func (wg *WaitGroup) Done() {
	wg.Add(-1)
}

// Wait blocks until the WaitGroup counter is zero.
// Wait会保持阻塞直到WaitGroup计数器归零
func (wg *WaitGroup) Wait() {
	// 先获得计数器和存储原语
	statep, semap := wg.state()
	if race.Enabled {
		_ = *statep // trigger nil deref early
		race.Disable()
	}
	// 只有计数器归零才会结束
	for {
		state := atomic.LoadUint64(statep)
		// 计数
		v := int32(state >> 32)
		// 无符号计数
		w := uint32(state)
		// 如果计数器已经归零，则直接退出循环
		if v == 0 {
			// Counter is 0, no need to wait.
			if race.Enabled {
				race.Enable()
				race.Acquire(unsafe.Pointer(wg))
			}
			return
		}
		// Increment waiters count.
		// 增加等待计数，此处的原语会比较statep和state的值，如果相同则等待计数加1
		if atomic.CompareAndSwapUint64(statep, state, state+1) {
			if race.Enabled && w == 0 {
				// Wait must be synchronized with the first Add.
				// Need to model this is as a write to race with the read in Add.
				// As a consequence, can do the write only for the first waiter,
				// otherwise concurrent Waits will race with each other.
				race.Write(unsafe.Pointer(semap))
			}
			// 会阻塞到存储原语是否 > 0（即睡眠），如果 *semp > 0 则会减一，因此最终的 semp 理论为0
			runtime_Semacquire(semap)
			// 在这种情况下，如果 *semap 不等于0，则说明使用失误，直接panic
			if *statep != 0 {
				panic("sync: WaitGroup is reused before previous Wait has returned")
			}
			if race.Enabled {
				race.Enable()
				race.Acquire(unsafe.Pointer(wg))
			}
			return
		}
	}
}
