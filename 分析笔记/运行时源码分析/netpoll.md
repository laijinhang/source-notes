# 一、相关函数
### 1. runtime_pollServerInit
func runtime_pollServerInit()

runtime/netpoll.go
```go
func poll_runtime_pollServerInit() {
	netpollGenericInit()
}

/*
保证netpoll只被初始化一次
 */
func netpollGenericInit() {
    // 1、如果已经初始化，则直接结束
    if atomic.Load(&netpollInited) == 0 {
        lockInit(&netpollInitLock, lockRankNetpollInit)
        // 2、上锁，在初始化完成之前，可能有多个goroutine进到这里进行初始化
        lock(&netpollInitLock)
        // 3、初始化完成前，可能有goroutine进到这里，所以需要再判断一下最新的初始化状态
        if netpollInited == 0 {
            // 4、初始化netpoll
            netpollinit()
            // 5、设置初始化状态为已初始化
            atomic.Store(&netpollInited, 1)
        }
        // 6、解锁
        unlock(&netpollInitLock)
    }
}
```

runtime/netpoll_epoll.go
```go
// epoll模型的初始化
// 对于一个M主线程只会初始化一张epoll表，所有要监听的文件描述符都会放入这个表中。
/*
	1、创建epoll（_EPOLL_CLOEXEC用来设置文件close-on-exec状态的。当close-on-exec状态为0时，调用exec时，fd不会被关闭；状态非零时则会被关闭，这样做可以防止fd泄露给执行exec后的进程）
	2、如果第一步创建失败，则调用epollcreate(1024)进行创建，如果再创建失败，则抛出netpoll初始化错误，如果创建成功，则设置close-on-exec标识，也就是_EPOLL_CLOEXEC
	在第一步创建失败时，尝试第二步的创建是为了兼容旧的linux内核
	3、设置epoll事件为EPOLLIN事件（EPOLLIN事件则只有当对端有数据写入时才会触发，所以触发一次后需要不断读取所有数据直到读完EAGAIN为止。否则剩下的数据只有在下次对端有写入时才能一起取出来了）
	4、注册epoll事件
*/
func netpollinit() {
	epfd = epollcreate1(_EPOLL_CLOEXEC)
	if epfd < 0 {
		epfd = epollcreate(1024)
		if epfd < 0 {
			println("runtime: epollcreate failed with", -epfd)
			throw("runtime: netpollinit failed")
		}
		closeonexec(epfd)
	}
	r, w, errno := nonblockingPipe()
	if errno != 0 {
		println("runtime: pipe failed with", -errno)
		throw("runtime: pipe failed")
	}
	ev := epollevent{
		events: _EPOLLIN,
	}
	*(**uintptr)(unsafe.Pointer(&ev.data)) = &netpollBreakRd
	errno = epollctl(epfd, _EPOLL_CTL_ADD, r, &ev)
	if errno != 0 {
		println("runtime: epollctl failed with", -errno)
		throw("runtime: epollctl failed")
	}
	netpollBreakRd = uintptr(r)
	netpollBreakWr = uintptr(w)
}
```
### 2. runtime_pollOpen
func runtime_pollOpen(fd uintptr) (uintptr, int)

runtime/net_epoll.go
```go
//go:linkname poll_runtime_pollOpen internal/poll.runtime_pollOpen
func poll_runtime_pollOpen(fd uintptr) (*pollDesc, int) {
	// 1、从pollcache里拿出第一个pollDesc，如果pollcache里面第一个是空的，则为其分配一个，然后返回第一个，pollcache指向第二个
	pd := pollcache.alloc()
	// 2、上锁
	lock(&pd.lock)

	// 3、正在写
	if pd.wg != 0 && pd.wg != pdReady {
		throw("runtime: blocked write on free polldesc") // 运行时：在空闲的Polldesc上写东西受阻
	}
	// 4、正在读
	if pd.rg != 0 && pd.rg != pdReady {
		throw("runtime: blocked read on free polldesc") // 运行时：阻断了对free polldesc的读取
	}
	// 5、初始化pd
	pd.fd = fd
	pd.closing = false
	pd.everr = false
	pd.rseq++
	pd.rg = 0
	pd.rd = 0
	pd.wseq++
	pd.wg = 0
	pd.wd = 0
	pd.self = pd
	// 6、解锁
	unlock(&pd.lock)
	// 7、事件注册函数，将监听套接字描述符加入监听事件
	errno := netpollopen(fd, pd)
	// 8、如果注册事件失败，则将其放回到pollcache，并返回错误信息
	if errno != 0 {
		pollcache.free(pd)
		return nil, int(errno)
	}
	return pd, 0
}

```
### 3. runtime_pollClose
func runtime_pollClose(ctx uintptr)
### 4. runtime_pollWait
func runtime_pollWait(ctx uintptr, mode int) int
### 5. runtime_pollWaitCanceled
func runtime_pollWaitCanceled(ctx uintptr, mode int) int
### 6. runtime_pollReset
func runtime_pollReset(ctx uintptr, mode int) int
### 7. runtime_pollSetDeadline
func runtime_pollSetDeadline(ctx uintptr, d int64, mode int)
### 8. runtime_pollUnblock
func runtime_pollUnblock(ctx uintptr)
### 9. runtime_isPollServerDescriptor
func runtime_isPollServerDescriptor(fd uintptr) bool
