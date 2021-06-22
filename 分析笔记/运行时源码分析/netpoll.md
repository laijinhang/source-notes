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
