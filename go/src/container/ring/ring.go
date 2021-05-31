// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ring implements operations on circular lists.
// 包ring实现循环列表上的操作。
package ring

// A Ring is an element of a circular list, or ring.
// Rings do not have a beginning or end; a pointer to any ring element
// serves as reference to the entire ring. Empty rings are represented
// as nil Ring pointers. The zero value for a Ring is a one-element
// ring with a nil Value.
//
// Ring是循环列表或环的元素。ring没有开始或结束；
// 指向任何环元素的指针用作对整个环的引用。
// 空环表示为Nil环指针。环的零值是具有零值的单元素环。
type Ring struct {
	next, prev *Ring
	Value      interface{} // for use by client; untouched by this library	// 供客户端使用；此库未触及
}

func (r *Ring) init() *Ring {
	r.next = r
	r.prev = r
	return r
}

// Next returns the next ring element. r must not be empty.
// Next返回下一个环元素。r不能为空。
func (r *Ring) Next() *Ring {
	if r.next == nil {
		return r.init()
	}
	return r.next
}

// Prev returns the previous ring element. r must not be empty.
// Prev返回上一个环元素。r不能为空。
func (r *Ring) Prev() *Ring {
	if r.next == nil {
		return r.init()
	}
	return r.prev
}

// Move moves n % r.Len() elements backward (n < 0) or forward (n >= 0)
// in the ring and returns that ring element. r must not be empty.
// Move在环中向后（n<0）或向前（n>=0）移动 n%r.Len() 元素并返回该环元素。r不能为空。
//
func (r *Ring) Move(n int) *Ring {
	if r.next == nil {
		return r.init()
	}
	switch {
	case n < 0:
		for ; n < 0; n++ {
			r = r.prev
		}
	case n > 0:
		for ; n > 0; n-- {
			r = r.next
		}
	}
	return r
}

// New creates a ring of n elements.
// New创建一个由n个元素组成的环。
func New(n int) *Ring {
	if n <= 0 {
		return nil
	}
	r := new(Ring)
	p := r
	for i := 1; i < n; i++ {
		p.next = &Ring{prev: p}
		p = p.next
	}
	p.next = r
	r.prev = p
	return r
}

// Link connects ring r with ring s such that r.Next()
// becomes s and returns the original value for r.Next().
// r must not be empty.
// Link将环r与环s连接起来，使r.Next()变成s并返回r.Next()的原始值。r不能为空。
//
// If r and s point to the same ring, linking
// them removes the elements between r and s from the ring.
// The removed elements form a subring and the result is a
// reference to that subring (if no elements were removed,
// the result is still the original value for r.Next(),
// and not nil).
// 如果r和s指向同一个环，连接它们就会从环上移除r和s之间的元素。
// 被删除的元素形成一个子链表，其结果是对该子链表的引用(如果没
// 有删除元素，结果仍然是r.t next()的原始值，而不是nil)。
//
// If r and s point to different rings, linking
// them creates a single ring with the elements of s inserted
// after r. The result points to the element following the
// last element of s after insertion.
// 如果r和s指向不同的环，连接它们将创建一个单独的环，其中s的元素
// 插入到r之后。结果指向插入后s的最后一个元素后面的元素。
//
func (r *Ring) Link(s *Ring) *Ring {
	n := r.Next()
	if s != nil {
		p := s.Prev()
		// Note: Cannot use multiple assignment because
		// evaluation order of LHS is not specified.
		r.next = s
		s.prev = r
		n.prev = p
		p.next = n
	}
	return n
}

// Unlink removes n % r.Len() elements from the ring r, starting
// at r.Next(). If n % r.Len() == 0, r remains unchanged.
// The result is the removed subring. r must not be empty.
// Unlink从环r开始删除第 n%r.Len()元素，从r.Next（）开始。
// 如果 n%r.Len（）==0，则r保持不变。结果就是移除的子环。r不能为空。
//
func (r *Ring) Unlink(n int) *Ring {
	if n <= 0 {
		return nil
	}
	return r.Link(r.Move(n + 1))
}

// Len computes the number of elements in ring r.
// It executes in time proportional to the number of elements.
// Len计算环r中的元素数量。它的执行时间与元素数量成比例。
//
func (r *Ring) Len() int {
	n := 0
	if r != nil {
		n = 1
		for p := r.Next(); p != r; p = p.next {
			n++
		}
	}
	return n
}

// Do calls function f on each element of the ring, in forward order.
// The behavior of Do is undefined if f changes *r.
// Do对环的每个元素调用函数f，按正向顺序。
// 如果f改变*r，Do的行为是未定义的。
func (r *Ring) Do(f func(interface{})) {
	if r != nil {
		f(r.Value)
		for p := r.Next(); p != r; p = p.next {
			f(p.Value)
		}
	}
}
