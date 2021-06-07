# 一、interface
src/runtime/runtime2.go
### 1. empty interface
> eface表示empty interface，不含任何方法

**数据结构：**
```go
type eface struct {
	_type *_type         // 实际类型，_type是Go语言中所有类型的公共描述，几乎所有的数据结构都可以抽象成_type
	data  unsafe.Pointer // 指向实际数据
}
```
**使用场景：**
* 用于存储数据
### 2. non-empty interface
> iface表示non-empty interface，即包含方法的接口，一般常用于定义接口，interface可以作为中间层进行解耦，将具体的实现和调用完全分离，上层的模块就不需要依赖某一个具体的实现，只需要依赖一个定义好的接口。

**数据结构：**
```go
type iface struct {
    tab  *itab
    data unsafe.Pointer
}
```
