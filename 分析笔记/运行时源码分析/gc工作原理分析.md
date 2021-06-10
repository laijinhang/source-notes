# 1、GC初始化
runtime/proc.go
```go
func schedinit() {
	...
	// 垃圾回收器初始化
	gcinit()
	...
}
```
# 2、启动GC后台工作
runtime/proc.go
```go
func main() {
	...
    gcenable()
	...
}
```
runtime/mgc.go
```go
func gcenable() {
	// Kick off sweeping and scavenging.
	// 开启清扫的程序
	gcenable_setup = make(chan int, 2)
	go bgsweep()
	go bgscavenge()
	<-gcenable_setup
	<-gcenable_setup
	gcenable_setup = nil
	memstats.enablegc = true // now that runtime is initialized, GC is okay
}
```
