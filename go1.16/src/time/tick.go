// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

import "errors"

// A Ticker holds a channel that delivers ``ticks'' of a clock
// at intervals.
// Ticker持有一个通道，每隔一段时间提供一个时钟的 "ticks"。
type Ticker struct {
	// 用于传输时间的通道
	C <-chan Time // The channel on which the ticks are delivered.
	r runtimeTimer
}

// NewTicker returns a new Ticker containing a channel that will send
// the time on the channel after each tick. The period of the ticks is
// specified by the duration argument. The ticker will adjust the time
// interval or drop ticks to make up for slow receivers.
// The duration d must be greater than zero; if not, NewTicker will
// panic. Stop the ticker to release associated resources.
// NewTicker返回一个新的Ticker，其中包含一个通道，在每个tick之后将发送通道上的时间。
// tick的周期是由持续时间参数指定的。Ticker将调整时间间隔或放弃ticks，以弥补缓慢的接收者。
// 持续时间d必须大于0；如果不是，NewTicker会恐慌。停止ticker以释放相关资源。
func NewTicker(d Duration) *Ticker {
	if d <= 0 {
		panic(errors.New("non-positive interval for NewTicker"))
	}
	// Give the channel a 1-element time buffer.
	// If the client falls behind while reading, we drop ticks
	// on the floor until the client catches up.
	c := make(chan Time, 1)
	t := &Ticker{
		C: c,
		r: runtimeTimer{
			when:   when(d),
			period: int64(d),
			f:      sendTime,
			arg:    c,
		},
	}
	startTimer(&t.r)
	return t
}

// Stop turns off a ticker. After Stop, no more ticks will be sent.
// Stop does not close the channel, to prevent a concurrent goroutine
// reading from the channel from seeing an erroneous "tick".
// "Stop"（停止）会关闭一个ticker。在停止之后，将不再发送ticks。
// 停止并不关闭通道，以防止同时从该通道读取的goroutine看到一个错误的 "tick"。
func (t *Ticker) Stop() {
	stopTimer(&t.r)
}

// Reset stops a ticker and resets its period to the specified duration.
// The next tick will arrive after the new period elapses.
// Reset 停止一个ticker并将其周期重置为指定的持续时间。下一个tick将在新的周期过后到达。
func (t *Ticker) Reset(d Duration) {
	if t.r.f == nil {
		panic("time: Reset called on uninitialized Ticker")
	}
	modTimer(&t.r, when(d), int64(d), t.r.f, t.r.arg, t.r.seq)
}

// Tick is a convenience wrapper for NewTicker providing access to the ticking
// channel only. While Tick is useful for clients that have no need to shut down
// the Ticker, be aware that without a way to shut it down the underlying
// Ticker cannot be recovered by the garbage collector; it "leaks".
// Unlike NewTicker, Tick will return nil if d <= 0.
// Tick是NewTicker的一个方便的包装器，只提供对打勾通道的访问。
// 虽然Tick对于那些不需要关闭Ticker的客户来说很有用，但要注意的是，
// 如果没有关闭的方法，底层的Ticker就不能被垃圾收集器恢复，
// 它就会 "泄漏"。与NewTicker不同，如果d<=0，Tick将返回nil。
func Tick(d Duration) <-chan Time {
	if d <= 0 {
		return nil
	}
	return NewTicker(d).C
}
