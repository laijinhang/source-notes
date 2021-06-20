# 1、数据结构
### 1. 结构
```go
/*
1、初始化
	dataqsiz记录长度、elemtype记录类型
2、向chan发送数据
	sendx、sendq、buf存放数据
3、向chan读取数据
	recvx、recvq、buf存放数据
4、关闭chan
	closed
互斥锁：保证并发安全
*/
type hchan struct {
	qcount   uint           // total data in the queue					// buffer中已放入的元素个数
	dataqsiz uint           // size of the circular queue				// 用户构造 channel 时指定的 buf 大小，可以理解成类似数组中初始分配的长度
	buf      unsafe.Pointer // points to an array of dataqsiz elements
	elemsize uint16
	closed   uint32 // 是否关闭，值为0表示未关闭
	elemtype *_type // element type				// 元素的类型信息
	sendx    uint   // send index				// 已发送的索引位置
	recvx    uint   // receive index			// 已接收的索引位置
	/*
		接收而阻塞的等待队列
	*/
	recvq waitq // list of recv waiters
	/*
		发送而阻塞的等待队列
	*/
	sendq waitq // list of send waiters

	// lock protects all fields in hchan, as well as several
	// fields in sudogs blocked on this channel.
	// lock保护hchan中的所有字段，以及在这个通道上被封锁的sudogs中的几个字段。
	//
	// Do not change another G's status while holding this lock
	// (in particular, do not ready a G), as this can deadlock
	// with stack shrinking.
	// 在持有这个锁的时候不要改变另一个G的状态（特别是不要准备好一个G），因为这可能会与堆栈收缩发生死锁。
	/*
		保护hchan的所有字段
	*/
	lock mutex
}

type waitq struct {
	first *sudog
	last  *sudog
}
```
# 二、源码分析
### 1. 创建
### 2. 读
### 3. 写
### 4. 关闭
* 关闭未分配内存的chan：`panic(plainError("close of nil channel"))`
* 关闭之后再写：`panic(plainError("send on closed channel"))`
* 关闭之后再关闭：`panic(plainError("close of closed channel"))`

```go
func closechan(c *hchan) {
	// 1、如果clone未分配内存的chan，抛出 关闭空channel的错误
	if c == nil {
		panic(plainError("close of nil channel"))
	}

	// 2、上runtime内部实现的互斥锁
	lock(&c.lock)
	// 3、如果是关闭已关闭的chan，则抛出 关闭已关闭channel的错误
	if c.closed != 0 {
		unlock(&c.lock)
		panic(plainError("close of closed channel"))
	}

	if raceenabled {
		callerpc := getcallerpc()
		racewritepc(c.raceaddr(), callerpc, funcPC(closechan))
		racerelease(c.raceaddr())
	}

	// 设置关闭状态 为已关闭
	c.closed = 1

	var glist gList

	/*
		因为在channel可读为空的时候，再读会被阻塞，那么就有可能存在若干goroutine被阻塞在读等待，因此需要释放所有读等待的goroutine
	*/
	// release all readers
	// 释放所有readers
	for {
		sg := c.recvq.dequeue()
		if sg == nil {
			break
		}
		if sg.elem != nil {
			typedmemclr(c.elemtype, sg.elem)
			sg.elem = nil
		}
		if sg.releasetime != 0 {
			sg.releasetime = cputicks()
		}
		gp := sg.g
		gp.param = unsafe.Pointer(sg)
		sg.success = false
		if raceenabled {
			raceacquireg(gp, c.raceaddr())
		}
		glist.push(gp)
	}

	/*
		因为在关闭之前，有正在写或准备写的
	*/
	// release all writers (they will panic)
	// 释放所有的writers（写已经关闭的，会导致panic）。
	for {
		sg := c.sendq.dequeue()
		if sg == nil {
			break
		}
		sg.elem = nil
		if sg.releasetime != 0 {
			sg.releasetime = cputicks()
		}
		gp := sg.g
		gp.param = unsafe.Pointer(sg)
		sg.success = false
		if raceenabled {
			raceacquireg(gp, c.raceaddr())
		}
		glist.push(gp)
	}
	// 解锁
	unlock(&c.lock)

	// Ready all Gs now that we've dropped the channel lock.
	// 准备好所有的G，现在我们已经放弃了通道锁。
	for !glist.empty() {
		gp := glist.pop()
		gp.schedlink = 0
		goready(gp, 3)
	}
}
```