除了直接研究go编译源码外，有办法直接看到编译过程吗？

有的，在go里面可以使用-n选项，在命令不执行的情况下，查看go build的执行流程

main.go
```go
package main

func main()  {
	
}
```
> go build -n main.go

在linux下，打印了如下信息
```go
#
# command-line-arguments
#

mkdir -p $WORK/command-line-arguments/_obj/
mkdir -p $WORK/command-line-arguments/_obj/exe/
cd /root
/usr/lib/go-1.6/pkg/tool/linux_amd64/compile -o $WORK/command-line-arguments.a -trimpath $WORK -p main -complete -buildid 40c0ec8c673eaf6eea285e983c7d53e053f2906b -D _/root -I $WORK -pack ./main.go
cd .
/usr/lib/go-1.6/pkg/tool/linux_amd64/link -o $WORK/command-line-arguments/_obj/exe/a.out -L $WORK -extld=gcc -buildmode=exe -buildid=40c0ec8c673eaf6eea285e983c7d53e053f2906b $WORK/command-line-arguments.a
mv $WORK/command-line-arguments/_obj/exe/a.out main

```
加上注释
```
#
# command-line-arguments
#

# 创建目录
mkdir -p $WORK/command-line-arguments/_obj/
# 创建command-line-arguments中exe目录
mkdir -p $WORK/command-line-arguments/_obj/exe/
# 编译文件
cd /root
# 链接生成a.out可执行文件
/usr/lib/go-1.6/pkg/tool/linux_amd64/compile -o $WORK/command-line-arguments.a -trimpath $WORK -p main -complete -buildid 40c0ec8c673eaf6eea285e983c7d53e053f2906b -D _/root -I $WORK -pack ./main.go
# 更新a.out id
cd .
# mv a.out改变名为default的可执行文件
/usr/lib/go-1.6/pkg/tool/linux_amd64/link -o $WORK/command-line-arguments/_obj/exe/a.out -L $WORK -extld=gcc -buildmode=exe -buildid=40c0ec8c673eaf6eea285e983c7d53e053f2906b $WORK/command-line-arguments.a
mv $WORK/command-line-arguments/_obj/exe/a.out main

```