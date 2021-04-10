// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"sync/atomic"
)

// Once is an object that will perform exactly one action.
//
// A Once must not be copied after first use.
// Once是一个只执行一个动作的对象。
//
// 一个Once使用后不得复制。
type Once struct {
	// done indicates whether the action has been performed.
	// It is first in the struct because it is used in the hot path.
	// The hot path is inlined at every call site.
	// Placing done first allows more compact instructions on some architectures (amd64/386),
	// and fewer instructions (to calculate offset) on other architectures.
	// done表示动作是否已经执行。它在结构体中排在第一位，因为它被用于热路径。
	// 热路径在每个调用点都是内联的。将 done 放在第一位，可以在某些架构上
	// (amd64/386)使指令更加紧凑，在其他架构上可以减少指令(计算偏移量)。
	done uint32 // 0 表示未执行，1 表示已执行，只有第一次执行f完之后才将其设为1，没有执行完之前使用互斥锁保证只有一个在执行
	m    Mutex
}

// Do calls the function f if and only if Do is being called for the
// first time for this instance of Once. In other words, given
// 	var once Once
// if once.Do(f) is called multiple times, only the first call will invoke f,
// even if f has a different value in each invocation. A new instance of
// Once is required for each function to execute.
// 如果且仅当Do对这个Once实例是第一次被调用时，Do才会调用函数f。
// 换句话说，给定var once once，如果 once.Do(f)被多次调用，
// 只有第一次调用才会调用f，即使f在每次调用中都有不同的值。每一
// 个函数的执行都需要一个新的Once实例。
//
// Do is intended for initialization that must be run exactly once. Since f
// is niladic, it may be necessary to use a function literal to capture the
// arguments to a function to be invoked by Do:
// 	config.once.Do(func() { config.init(filename) })
// Do的目的是用于初始化，必须准确地运行一次。由于f是niladic的，所以可能需要使用一个函数文字
// 来捕获Do所要调用的函数的参数：config.once.Do(func() { config.init(filename) })
//
// Because no call to Do returns until the one call to f returns, if f causes
// Do to be called, it will deadlock.
// 因为在对f的一次调用没有返回之前，对Do的调用都不会返回，如果f导致Do被调用，就会死锁。
//
// If f panics, Do considers it to have returned; future calls of Do return
// without calling f.
// 如果f panics，Do就认为它已经返回了；未来Do的调用不需要调用f就能返回。
func (o *Once) Do(f func()) {
	// Note: Here is an incorrect implementation of Do:
	// 注：这里是Do的错误实现。
	//
	//	if atomic.CompareAndSwapUint32(&o.done, 0, 1) {
	//		f()
	//	}
	//
	// Do guarantees that when it returns, f has finished.
	// This implementation would not implement that guarantee:
	// given two simultaneous calls, the winner of the cas would
	// call f, and the second would return immediately, without
	// waiting for the first's call to f to complete.
	// This is why the slow path falls back to a mutex, and why
	// the atomic.StoreUint32 must be delayed until after f returns.

	if atomic.LoadUint32(&o.done) == 0 {
		// Outlined slow-path to allow inlining of the fast-path.
		o.doSlow(f)
	}
}

func (o *Once) doSlow(f func()) {
	o.m.Lock()
	defer o.m.Unlock()
	if o.done == 0 {
		defer atomic.StoreUint32(&o.done, 1)
		f()
	}
}
