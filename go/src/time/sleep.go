// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

// Sleep pauses the current goroutine for at least the duration d.
// A negative or zero duration causes Sleep to return immediately.
// Sleep会让当前的goroutine暂停，至少持续时间为d。
// 一个负的或零的持续时间会使Sleep立即返回。
func Sleep(d Duration)

// Interface to timers implemented in package runtime.
// Must be in sync with ../runtime/time.go:/^type timer
// 在包runtime中实现的定时器的接口。
// 必须与./runtime/time.go:/^type timer同步。
type runtimeTimer struct {
	pp       uintptr
	when     int64
	period   int64
	f        func(interface{}, uintptr) // NOTE: must not be closure	// 注意：不得关闭
	arg      interface{}
	seq      uintptr
	nextwhen int64
	status   uint32
}

// when is a helper function for setting the 'when' field of a runtimeTimer.
// It returns what the time will be, in nanoseconds, Duration d in the future.
// If d is negative, it is ignored. If the returned value would be less than
// zero because of an overflow, MaxInt64 is returned.
// when是一个辅助函数，用于设置运行时间定时器的'when'字段。
// 它返回未来的时间，以纳秒为单位，时间为d。如果d为负数，则被忽略。
// 如果由于溢出，返回值将小于0，则返回MaxInt64。
func when(d Duration) int64 {
	// 如果d小于等于0，则返回当前纳秒时间
	if d <= 0 {
		return runtimeNano()
	}
	t := runtimeNano() + int64(d)
	if t < 0 {
		// N.B. runtimeNano() and d are always positive, so addition
		// (including overflow) will never result in t == 0.
		t = 1<<63 - 1 // math.MaxInt64
	}
	return t
}

func startTimer(*runtimeTimer)
func stopTimer(*runtimeTimer) bool
func resetTimer(*runtimeTimer, int64) bool
func modTimer(t *runtimeTimer, when, period int64, f func(interface{}, uintptr), arg interface{}, seq uintptr)

// The Timer type represents a single event.
// When the Timer expires, the current time will be sent on C,
// unless the Timer was created by AfterFunc.
// A Timer must be created with NewTimer or AfterFunc.
// 定时器类型代表一个单一的事件。
// 当定时器过期时，当前时间将被发送到C上，
// 除非该定时器是由AfterFunc创建的。
// 定时器必须用NewTimer或AfterFunc创建。
type Timer struct {
	C <-chan Time
	r runtimeTimer
}

// Stop prevents the Timer from firing.
// It returns true if the call stops the timer, false if the timer has already
// expired or been stopped.
// Stop does not close the channel, to prevent a read from the channel succeeding
// incorrectly.
// 如果调用停止定时器，则返回true；
// 如果定时器已经过期或被停止，则返回false。
// 不关闭通道，以防止从通道中的读取错误地成功。
//
// To ensure the channel is empty after a call to Stop, check the
// return value and drain the channel.
// For example, assuming the program has not received from t.C already:
//
// 	if !t.Stop() {
// 		<-t.C
// 	}
//
// This cannot be done concurrent to other receives from the Timer's
// channel or other calls to the Timer's Stop method.
//
// For a timer created with AfterFunc(d, f), if t.Stop returns false, then the timer
// has already expired and the function f has been started in its own goroutine;
// Stop does not wait for f to complete before returning.
// If the caller needs to know whether f is completed, it must coordinate
// with f explicitly.
func (t *Timer) Stop() bool {
	if t.r.f == nil {
		panic("time: Stop called on uninitialized Timer")
	}
	return stopTimer(&t.r)
}

// NewTimer creates a new Timer that will send
// the current time on its channel after at least duration d.
// NewTimer创建一个新的Timer，它将在至少持续时间d之后在其通道上发送当前时间。
func NewTimer(d Duration) *Timer {
	// 创建一个长度为1的缓冲
	c := make(chan Time, 1)
	t := &Timer{
		C: c,
		r: runtimeTimer{
			when: when(d),
			f:    sendTime,
			arg:  c,
		},
	}
	startTimer(&t.r)
	return t
}

// Reset changes the timer to expire after duration d.
// It returns true if the timer had been active, false if the timer had
// expired or been stopped.
// 如果定时器一直处于活动状态，则返回true；
// 如果定时器已经过期或被停止，则返回false。
//
// For a Timer created with NewTimer, Reset should be invoked only on
// stopped or expired timers with drained channels.
// 对于用NewTimer创建的定时器，Reset应该只在停止的或过期的定时器上调用，且通道已耗尽。
//
// If a program has already received a value from t.C, the timer is known
// to have expired and the channel drained, so t.Reset can be used directly.
// If a program has not yet received a value from t.C, however,
// the timer must be stopped and—if Stop reports that the timer expired
// before being stopped—the channel explicitly drained:
// 如果一个程序已经从t.C那里收到了一个值，那么定时器就被认为已经过期了，通道也被排空了，所以可以直接使用t.Reset。
// 然而，如果一个程序还没有从t.C那里收到一个值，则必须停止定时器，并且--如果Stop报告说定时器在被停止之前已经过期，则通道明确地被排空。
//
// 	if !t.Stop() {
// 		<-t.C
// 	}
// 	t.Reset(d)
//
// This should not be done concurrent to other receives from the Timer's
// channel.
// 这不应该与定时器通道的其他接收同时进行。
//
// Note that it is not possible to use Reset's return value correctly, as there
// is a race condition between draining the channel and the new timer expiring.
// Reset should always be invoked on stopped or expired channels, as described above.
// The return value exists to preserve compatibility with existing programs.
// 注意，不可能正确使用Reset的返回值，因为在耗尽通道和新的定时器到期之间存在一个竞赛条件。
// 复位应该总是在停止或过期的通道上调用，如上所述。返回值的存在是为了保持与现有程序的兼容性。
//
// For a Timer created with AfterFunc(d, f), Reset either reschedules
// when f will run, in which case Reset returns true, or schedules f
// to run again, in which case it returns false.
// 对于用AfterFunc(d, f)创建的定时器，Reset要么重新安排f的运行时间，
// 在这种情况下Reset返回真，要么重新安排f的运行时间，在这种情况下它返回假。
// When Reset returns false, Reset neither waits for the prior f to
// complete before returning nor does it guarantee that the subsequent
// goroutine running f does not run concurrently with the prior
// one. If the caller needs to know whether the prior execution of
// f is completed, it must coordinate with f explicitly.
func (t *Timer) Reset(d Duration) bool {
	if t.r.f == nil {
		panic("time: Reset called on uninitialized Timer")
	}
	w := when(d)
	return resetTimer(&t.r, w)
}

func sendTime(c interface{}, seq uintptr) {
	// Non-blocking send of time on c.
	// Used in NewTimer, it cannot block anyway (buffer).
	// Used in NewTicker, dropping sends on the floor is
	// the desired behavior when the reader gets behind,
	// because the sends are periodic.
	// 在c上非阻塞地发送时间。
	// 在NewTimer中使用，它无论如何不能阻塞（缓冲区）。
	// 在NewTicker中使用，当读取落后时，把发送的内容丢在地上是所希望的行为，因为发送是周期性的。
	select {
	case c.(chan Time) <- Now():
	default:
	}
}

/*
	定时器的原理：记录要触发的时间，到了触发时间时，往这个chan里面发送触发时间

	返回一个chan类型为chan Time，长度为1的chan，d时间之后这个chan会被写值
	使用案例：

func main()  {
	t := time.After(3 *time.Second)
	fmt.Println("3秒前",time.Now().String())
	fmt.Println("3秒后", (<-t).String())
}
 */
// After waits for the duration to elapse and then sends the current time
// on the returned channel.
// It is equivalent to NewTimer(d).C.
// The underlying Timer is not recovered by the garbage collector
// until the timer fires. If efficiency is a concern, use NewTimer
// instead and call Timer.Stop if the timer is no longer needed.
// After等待过去后，在返回的通道上发送当前时间。
// 它等同于NewTimer(d).C。
// 在定时器启动之前，底层的定时器是不会被垃圾收集器恢复的。如果担心效率问题，可以用NewTimer代替，如果不再需要定时器，就调用Timer.Stop。
func After(d Duration) <-chan Time {
	return NewTimer(d).C
}

// AfterFunc waits for the duration to elapse and then calls f
// in its own goroutine. It returns a Timer that can
// be used to cancel the call using its Stop method.
func AfterFunc(d Duration, f func()) *Timer {
	t := &Timer{
		r: runtimeTimer{
			when: when(d),
			f:    goFunc,
			arg:  f,
		},
	}
	startTimer(&t.r)
	return t
}

func goFunc(arg interface{}, seq uintptr) {
	go arg.(func())()
}
