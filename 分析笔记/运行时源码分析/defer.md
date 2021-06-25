# 一、使用
### 1. 解决的问题
1. 保证在函数在退出时，能够执行某些操作，除非遇到其他地方导致程序结束，如defer执行前调用exit和其他协程导致的panic在defer之前触发导致程序结束
2. 处理这个函数里panic

关于第一点的panic导致没执行defer的案例，如下：
```go
func main() {
	go func() {
		a := 2
		defer func(v int) {
			fmt.Println(v)
		}(a)
		a = 1
		time.Sleep(3 * time.Second)
	}()
	time.Sleep(time.Second)
	panic(1)
	time.Sleep(3 * time.Second)
}
```
### 2. 注意
* 在同一个函数里是栈的形式执行，也就是先入后出
* defer 后跟一个函数，如果这个函数里的实参里面包含变量，那么这个实参是取当前的defer时的值，而不是这个函数执行完后去执行defer之前的值
* defer 后跟一个函数，如果这个函数里的实参里面包含直接执行的函数，那么这个实参是取当前的defer时的值，而不是这个函数执行完后去执行这个实参函数

上面两点结合起来就是，经过defer的时候，其实参的值是固定下来，就是这一刻的值，就算是后面有变更该变量的，其实参的值也不会因此而改变

下面给个例子：
```go
func main() {
	a := 1
	defer func(v1, v2 int) {
		fmt.Println(v1, v2, a)
	}(a, func() int {
		return 12
	}())
	defer func() {
		fmt.Println("123")
	}()
	a = 20
}
```
分析：
1. 经过第一个defer时，其实参就固定下来的，分别是1,12
2. 结束时，先执行后面的那个defer，也就是输出 12 ，之后输出 1 12 20
### 3. 性能
在go1.14之后有进行优化
# 二、源码分析