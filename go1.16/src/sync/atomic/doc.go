// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package atomic provides low-level atomic memory primitives
// useful for implementing synchronization algorithms.
//
// These functions require great care to be used correctly.
// Except for special, low-level applications, synchronization is better
// done with channels or the facilities of the sync package.
// Share memory by communicating;
// don't communicate by sharing memory.
//
// The swap operation, implemented by the SwapT functions, is the atomic
// equivalent of:
//
//	old = *addr
//	*addr = new
//	return old
//
// The compare-and-swap operation, implemented by the CompareAndSwapT
// functions, is the atomic equivalent of:
//
//	if *addr == old {
//		*addr = new
//		return true
//	}
//	return false
//
// The add operation, implemented by the AddT functions, is the atomic
// equivalent of:
//
//	*addr += delta
//	return *addr
//
// The load and store operations, implemented by the LoadT and StoreT
// functions, are the atomic equivalents of "return *addr" and
// "*addr = val".
//
package atomic

import (
	"unsafe"
)

// BUG(rsc): On 386, the 64-bit functions use instructions unavailable before the Pentium MMX.
// BUG(rsc)。在386上，64位函数使用了奔腾MMX之前不可用的指令。
//
// On non-Linux ARM, the 64-bit functions use instructions unavailable before the ARMv6k core.
// 在非Linux ARM上，64位功能使用ARMv6k内核之前不可用的指令。
//
// On ARM, 386, and 32-bit MIPS, it is the caller's responsibility
// to arrange for 64-bit alignment of 64-bit words accessed atomically.
// The first word in a variable or in an allocated struct, array, or slice can
// be relied upon to be 64-bit aligned.
// 在ARM、386和32位MIPS上，调用者有责任安排原子访问的64位字的64位对齐。
// 在一个变量或分配的结构、数组或片中的第一个字可以依靠64位对齐。

// SwapInt32 atomically stores new into *addr and returns the previous *addr value.
// SwapInt32原子性地将new存储到*addr中，并返回之前的*addr值。
func SwapInt32(addr *int32, new int32) (old int32)

// SwapInt64 atomically stores new into *addr and returns the previous *addr value.
// SwapInt64原子性地将new存储到*addr中，并返回之前的*addr值。
func SwapInt64(addr *int64, new int64) (old int64)

// SwapUint32 atomically stores new into *addr and returns the previous *addr value.
// SwapUint32原子性地将new存储到*addr中，并返回之前的*addr值。
func SwapUint32(addr *uint32, new uint32) (old uint32)

// SwapUint64 atomically stores new into *addr and returns the previous *addr value.
// SwapUint64原子化地将new存储到*addr中，并返回之前的*addr值。
func SwapUint64(addr *uint64, new uint64) (old uint64)

// SwapUintptr atomically stores new into *addr and returns the previous *addr value.
// SwapUintptr原子性地将new存储到*addr中，并返回之前的*addr值。
func SwapUintptr(addr *uintptr, new uintptr) (old uintptr)

// SwapPointer atomically stores new into *addr and returns the previous *addr value.
// SwapPointer原子性地将new存储到*addr中，并返回之前的*addr值。
func SwapPointer(addr *unsafe.Pointer, new unsafe.Pointer) (old unsafe.Pointer)

// CompareAndSwapInt32 executes the compare-and-swap operation for an int32 value.
// CompareAndSwapInt32执行对一个int32值的比较和交换操作。
func CompareAndSwapInt32(addr *int32, old, new int32) (swapped bool)

// CompareAndSwapInt64 executes the compare-and-swap operation for an int64 value.
// CompareAndSwapInt64执行对一个int64值的比较和交换操作。
func CompareAndSwapInt64(addr *int64, old, new int64) (swapped bool)

// CompareAndSwapUint32 executes the compare-and-swap operation for a uint32 value.
// CompareAndSwapUint32对一个uint32值执行比较和交换操作。
func CompareAndSwapUint32(addr *uint32, old, new uint32) (swapped bool)

// CompareAndSwapUint64 executes the compare-and-swap operation for a uint64 value.
// CompareAndSwapUint64执行对一个uint64值的比较和交换操作。
func CompareAndSwapUint64(addr *uint64, old, new uint64) (swapped bool)

// CompareAndSwapUintptr executes the compare-and-swap operation for a uintptr value.
// CompareAndSwapUintptr执行对一个uintptr值的比较和交换操作。
func CompareAndSwapUintptr(addr *uintptr, old, new uintptr) (swapped bool)

// CompareAndSwapPointer executes the compare-and-swap operation for a unsafe.Pointer value.
// CompareAndSwapPointer对一个unsafe.Pointer值执行比较和交换操作。
func CompareAndSwapPointer(addr *unsafe.Pointer, old, new unsafe.Pointer) (swapped bool)

// AddInt32 atomically adds delta to *addr and returns the new value.
// AddInt32原子性地将delta加到*addr中，并返回新值。
func AddInt32(addr *int32, delta int32) (new int32)

// AddUint32 atomically adds delta to *addr and returns the new value.
// To subtract a signed positive constant value c from x, do AddUint32(&x, ^uint32(c-1)).
// In particular, to decrement x, do AddUint32(&x, ^uint32(0)).
// AddUint32原子性地将delta加到*addr中，并返回新值。
// 要从x中减去一个有符号的正值c，执行AddUint32(&x, ^uint32(c-1))。
// 特别是，要减去x，执行AddUint32(&x, ^uint32(0))。
func AddUint32(addr *uint32, delta uint32) (new uint32)

// AddInt64 atomically adds delta to *addr and returns the new value.
// AddInt64原子性地将delta加到*addr中，并返回新值。
func AddInt64(addr *int64, delta int64) (new int64)

// AddUint64 atomically adds delta to *addr and returns the new value.
// To subtract a signed positive constant value c from x, do AddUint64(&x, ^uint64(c-1)).
// In particular, to decrement x, do AddUint64(&x, ^uint64(0)).
// AddUint64原子性地将delta加到*addr中，并返回新值。
// 要从x中减去一个有符号的正值c，执行AddUint64(&x, ^uint64(c-1))。
// 特别是，要减去x，执行AddUint64(&x, ^uint64(0))。
func AddUint64(addr *uint64, delta uint64) (new uint64)

// AddUintptr atomically adds delta to *addr and returns the new value.
// AddUintptr原子性地将delta加到*addr中，并返回新值。
func AddUintptr(addr *uintptr, delta uintptr) (new uintptr)

// LoadInt32 atomically loads *addr.
// LoadInt32原子地加载*addr.
func LoadInt32(addr *int32) (val int32)

// LoadInt64 atomically loads *addr.
// LoadInt64原子地加载*addr.
func LoadInt64(addr *int64) (val int64)

// LoadUint32 atomically loads *addr.
// LoadUint32原子地加载*addr.
func LoadUint32(addr *uint32) (val uint32)

// LoadUint64 atomically loads *addr.
// LoadUint64原子地加载*addr.
func LoadUint64(addr *uint64) (val uint64)

// LoadUintptr atomically loads *addr.
// LoadUintptr原子地加载*addr.
func LoadUintptr(addr *uintptr) (val uintptr)

// LoadPointer atomically loads *addr.
// LoadPointer原子地加载*addr.
func LoadPointer(addr *unsafe.Pointer) (val unsafe.Pointer)

// StoreInt32 atomically stores val into *addr.
// StoreInt32原子地将val存储到*addr中。
func StoreInt32(addr *int32, val int32)

// StoreInt64 atomically stores val into *addr.
// StoreInt64原子地将val存储到*addr中。
func StoreInt64(addr *int64, val int64)

// StoreUint32 atomically stores val into *addr.
// StoreUint32原子地将val存储到*addr中。
func StoreUint32(addr *uint32, val uint32)

// StoreUint64 atomically stores val into *addr.
func StoreUint64(addr *uint64, val uint64)

// StoreUintptr atomically stores val into *addr.
// StoreUintptr原子地将val存储到*addr中。
func StoreUintptr(addr *uintptr, val uintptr)

// StorePointer atomically stores val into *addr.
// StorePointer原子地将val存储到*addr中。
func StorePointer(addr *unsafe.Pointer, val unsafe.Pointer)
