# 一、结构
### 1、defer数据结构
runtime/panic.go
```go
type _defer struct {
	siz int32       // 参数的大小
	started bool    // 是否执行过了
	heap    bool
	openDefer bool
	sp        uintptr  
	pc        uintptr  
	fn        *funcval 
	// defer中的panic
	_panic *_panic
	// defer链表，函数执行流程中的defer，会通过 link 这个属性进行串联
	link *_defer

	fd   unsafe.Pointer 
	varp uintptr        
	framepc uintptr
}
```
### 2、panic数据结构
```go
/*
在panic中使用 _panic 作为其基础单元，每执行一次 panic 语句，都会创建一个 _panic 对象。
它包含了一些基础的字段用于存储当前的panic调用情况，涉及的字段如下：
1. argp：指向 defer 延迟调用的参数的指针
2. arg：panic的原因，也就是调用 panic 时传入的参数
3. link：指向上一个调用的 _panic，这里说明panci也是一个链表
4. recovered：panic是否已经被处理过，也就是是否被recover接收掉了
5. aborted：panic是否被终止
*/
type _panic struct {
	argp      unsafe.Pointer // pointer to arguments of deferred call run during panic; cannot move - known to liblink
	arg       interface{}    // argument to panic
	link      *_panic        // link to earlier panic
	pc        uintptr        // where to return to in runtime if this panic is bypassed
	sp        unsafe.Pointer // where to return to in runtime if this panic is bypassed
	recovered bool           // whether this panic is over
	aborted   bool           // the panic was aborted
	goexit    bool
}
```
### 3、G
defer和panic都是绑定在 所运行的g上的
```go
// g中与defer、panic相关属性
type g struct {
_panic         *_panic // panic组成的链表
_defer         *_defer // defer组成的先进后出的链表，同栈
}
```
# 二、源码分析
