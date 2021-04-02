// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"errors"
	"internal/testlog"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// ErrProcessDone indicates a Process has finished.
var ErrProcessDone = errors.New("os: process already finished")

// Process stores the information about a process created by StartProcess.
type Process struct {
	Pid    int
	handle uintptr      // handle is accessed atomically on Windows
	isdone uint32       // process has been successfully waited on, non zero if true
	sigMu  sync.RWMutex // avoid race between wait and signal
}

func newProcess(pid int, handle uintptr) *Process {
	p := &Process{Pid: pid, handle: handle}
	runtime.SetFinalizer(p, (*Process).Release)
	return p
}

func (p *Process) setDone() {
	atomic.StoreUint32(&p.isdone, 1)
}

func (p *Process) done() bool {
	return atomic.LoadUint32(&p.isdone) > 0
}

// ProcAttr holds the attributes that will be applied to a new process
// started by StartProcess.
type ProcAttr struct {
	// If Dir is non-empty, the child changes into the directory before
	// creating the process.
	// 如果Dir非空，子进程会在创建Process实例前先进入该目录。（即设为子进程的当前目录）
	Dir string
	// If Env is non-nil, it gives the environment variables for the
	// new process in the form returned by Environ.
	// If it is nil, the result of Environ will be used.
	// 如果Env非空，它会作为新进程的环境变量。必须采样Environ返回值的格式。
	// 如果Env为nil，将使用Environ函数的返回值。
	Env []string
	// Files specifies the open files inherited by the new process. The
	// first three entries correspond to standard input, standard output, and
	// standard error. An implementation may support additional entries,
	// depending on the underlying operating system. A nil entry corresponds
	// to that file being closed when the process starts.
	// Files指定被新进程继承的打开文件对象。
	// 前三个绑定为标准输入、标准输出、标准错误输出。
	// 依赖底层操作系统的实现可能会支持额外的文件对象。
	// nil相当于在进程开始时关闭的文件对象。
	Files []*File

	// Operating system-specific process creation attributes.
	// Note that setting this field means that your program
	// may not execute properly or even compile on some
	// operating systems.
	// 操作系统特定的创建属性。
	// 注意设置本字段意味着你的程序可能会执行异常甚至在某些操作系统中无法通过编译。
	// 这时候可以通过为特定系统设置。
	// 看 syscall.SysProcAttr 的定义，可以知道用于控制进程的相关属性。
	Sys *syscall.SysProcAttr
}

// A Signal represents an operating system signal.
// The usual underlying implementation is operating system-dependent:
// on Unix it is syscall.Signal.
type Signal interface {
	String() string
	Signal() // to distinguish from other Stringers
}

// Getpid returns the process id of the caller.
func Getpid() int { return syscall.Getpid() }

// Getppid returns the process id of the caller's parent.
func Getppid() int { return syscall.Getppid() }

// FindProcess looks for a running process by its pid.
//
// The Process it returns can be used to obtain information
// about the underlying operating system process.
//
// On Unix systems, FindProcess always succeeds and returns a Process
// for the given pid, regardless of whether the process exists.
// FindProcess可以通过pid查找一个运行中的进程。该函数返回的Process对象可以用于获取
// 关于底层操作系统进程的信息。在Uinx系统中，此函数总是成功，即使pid对应的进程不存在。
func FindProcess(pid int) (*Process, error) {
	return findProcess(pid)
}

// StartProcess starts a new process with the program, arguments and attributes
// specified by name, argv and attr. The argv slice will become os.Args in the
// new process, so it normally starts with the program name.
//
// If the calling goroutine has locked the operating system thread
// with runtime.LockOSThread and modified any inheritable OS-level
// thread state (for example, Linux or Plan 9 name spaces), the new
// process will inherit the caller's thread state.
//
// StartProcess is a low-level interface. The os/exec package provides
// higher-level interfaces.
//
// If there is an error, it will be of type *PathError.
func StartProcess(name string, argv []string, attr *ProcAttr) (*Process, error) {
	testlog.Open(name)
	return startProcess(name, argv, attr)
}

/*
	Process 提供了四个方法：Kill、Signal、Wait和Release。
	1. Kill和Signal与信号有关，Kill是调用Signal，发送了 SIGKILL 信号，强制进程退出。
	2. Process方法用于释放Process对象相关的资源，以便将来可以被再使用。该方法只有在没有调用Wait时才需要调用。
		Unix中，该方法的内部实现只是将Process的pid置为-1.
*/

// Release releases any resources associated with the Process p,
// rendering it unusable in the future.
// Release only needs to be called if Wait is not.
func (p *Process) Release() error {
	return p.release()
}

// Kill causes the Process to exit immediately. Kill does not wait until
// the Process has actually exited. This only kills the Process itself,
// not any other processes it may have started.
func (p *Process) Kill() error {
	return p.kill()
}

// Wait waits for the Process to exit, and then returns a
// ProcessState describing its status and an error, if any.
// Wait releases any resources associated with the Process.
// On most operating systems, the Process must be a child
// of the current process or an error will be returned.
// Wait方法阻塞直到进程退出，然后返回一个ProcessState描述进程的状态和可能的错误。
// Wait方法会释放绑定到Process的所有资源。
// 在大多数操作系统中，Process必须是当前进程的子进程，否则会返回错误。
func (p *Process) Wait() (*ProcessState, error) {
	return p.wait()
}

// Signal sends a signal to the Process.
// Sending Interrupt on Windows is not implemented.
func (p *Process) Signal(sig Signal) error {
	return p.signal(sig)
}

// UserTime returns the user CPU time of the exited process and its children.
func (p *ProcessState) UserTime() time.Duration {
	return p.userTime()
}

// SystemTime returns the system CPU time of the exited process and its children.
func (p *ProcessState) SystemTime() time.Duration {
	return p.systemTime()
}

// Exited reports whether the program has exited.
// 是否正常退出
func (p *ProcessState) Exited() bool {
	return p.exited()
}

// Success reports whether the program exited successfully,
// such as with exit status 0 on Unix.
func (p *ProcessState) Success() bool {
	return p.success()
}

// Sys returns system-dependent exit information about
// the process. Convert it to the appropriate underlying
// type, such as syscall.WaitStatus on Unix, to access its contents.
func (p *ProcessState) Sys() interface{} {
	return p.sys()
}

// SysUsage returns system-dependent resource usage information about
// the exited process. Convert it to the appropriate underlying
// type, such as *syscall.Rusage on Unix, to access its contents.
// (On Unix, *syscall.Rusage matches struct rusage as defined in the
// getrusage(2) manual page.)
func (p *ProcessState) SysUsage() interface{} {
	return p.sysUsage()
}
