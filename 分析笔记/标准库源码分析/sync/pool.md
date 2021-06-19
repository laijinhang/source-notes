# 一、数据结构
# 二、内部工作原理
### 1. 程序开始
注册清理函数

sync/pool.go
```go
...
func init() {
	runtime_registerPoolCleanup(poolCleanup)
}
...
func runtime_registerPoolCleanup(cleanup func())
...
```
runtime/mgc.go
```go
...
var poolcleanup func()
...
func sync_runtime_registerPoolCleanup(f func()) {
	poolcleanup = f
}
```
### 2. GC开始前
### 3. GC处理中
runtime/mgc.go
```go
...
func gcStart(trigger gcTrigger) {
	...
	/*
		清理sched.sudogcache以及sync.Pools
	*/
	clearpools()
    ...
}
...


func clearpools() {
	// clear sync.Pools
	if poolcleanup != nil {
		poolcleanup()
	}

	// Clear central sudog cache.
	// Leave per-P caches alone, they have strictly bounded size.
	// Disconnect cached list before dropping it on the floor,
	// so that a dangling ref to one entry does not pin all of them.
	lock(&sched.sudoglock)
	var sg, sgnext *sudog
	for sg = sched.sudogcache; sg != nil; sg = sgnext {
		sgnext = sg.next
		sg.next = nil
	}
	sched.sudogcache = nil
	unlock(&sched.sudoglock)

	// Clear central defer pools.
	// Leave per-P pools alone, they have strictly bounded size.
	lock(&sched.deferlock)
	for i := range sched.deferpool {
		// disconnect cached list before dropping it on the floor,
		// so that a dangling ref to one entry does not pin all of them.
		var d, dlink *_defer
		for d = sched.deferpool[i]; d != nil; d = dlink {
			dlink = d.link
			d.link = nil
		}
		sched.deferpool[i] = nil
	}
	unlock(&sched.deferlock)
}
```
### 4. GC结束后
# 三、外部工作原理
### 1. Get
### 2. Put
