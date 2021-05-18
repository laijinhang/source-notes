// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package suffixarray implements substring search in logarithmic time using
// an in-memory suffix array.
// Package suffixarray在后缀数组内存在对数时间内实现子串搜索。
//
// Example use:
// 使用实例:
//
//	// create index for some data
//  // 为一些数据创建索引
//	index := suffixarray.New(data)
//
//	// lookup byte slice s
//	// 查找字节片s
//	offsets1 := index.Lookup(s, -1) // the list of all indices where s occurs in data	// 数据中出现s的所有索引的列表
//	offsets2 := index.Lookup(s, 3)  // the list of at most 3 indices where s occurs in data	// 列表中出现s的所有索引
//
package suffixarray

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"regexp"
	"sort"
)

// Can change for testing
// 可以为测试而改变
var maxData32 int = realMaxData32

const realMaxData32 = math.MaxInt32

// Index implements a suffix array for fast substring search.
// 索引实现了一个后缀数组，用于快速子串搜索。
type Index struct {
	data []byte
	sa   ints // suffix array for data; sa.len() == len(data)	// 数据的后缀数组; sa.len() == len(data)
}

// An ints is either an []int32 or an []int64.
// That is, one of them is empty, and one is the real data.
// The int64 form is used when len(data) > maxData32
// 一个ints要么是一个[]int32，要么是一个[]int64。也就是说，其中一个是空的，
// 一个是真正的数据。当len(data)>maxData32时，使用int64形式。
type ints struct {
	int32 []int32
	int64 []int64
}

func (a *ints) len() int {
	return len(a.int32) + len(a.int64)
}

func (a *ints) get(i int) int64 {
	if a.int32 != nil {
		return int64(a.int32[i])
	}
	return a.int64[i]
}

func (a *ints) set(i int, v int64) {
	if a.int32 != nil {
		a.int32[i] = int32(v)
	} else {
		a.int64[i] = v
	}
}

func (a *ints) slice(i, j int) ints {
	if a.int32 != nil {
		return ints{a.int32[i:j], nil}
	}
	return ints{nil, a.int64[i:j]}
}

// New creates a new Index for data.
// Index creation time is O(N) for N = len(data).
// New为数据创建一个新的索引。
// 创建索引的时间是O(N) for N = len(data).
func New(data []byte) *Index {
	ix := &Index{data: data}
	if len(data) <= maxData32 {
		ix.sa.int32 = make([]int32, len(data))
		text_32(data, ix.sa.int32)
	} else {
		ix.sa.int64 = make([]int64, len(data))
		text_64(data, ix.sa.int64)
	}
	return ix
}

// writeInt writes an int x to w using buf to buffer the write.
// writeInt将一个int x写入w，使用buf来缓冲写入。
func writeInt(w io.Writer, buf []byte, x int) error {
	binary.PutVarint(buf, int64(x))
	_, err := w.Write(buf[0:binary.MaxVarintLen64])
	return err
}

// readInt reads an int x from r using buf to buffer the read and returns x.
// readInt从r中读取一个int x，使用buf来缓冲读取，并返回x。
func readInt(r io.Reader, buf []byte) (int64, error) {
	_, err := io.ReadFull(r, buf[0:binary.MaxVarintLen64]) // ok to continue with error	 //可以继续进行错误的处理
	x, _ := binary.Varint(buf)
	return x, err
}

// writeSlice writes data[:n] to w and returns n.
// It uses buf to buffer the write.
// writeSlice将data[:n]写到w，并返回n。
// 它使用buf来缓冲写入的内容。
func writeSlice(w io.Writer, buf []byte, data ints) (n int, err error) {
	// encode as many elements as fit into buf
	// 将尽可能多的元素编码到buf中
	p := binary.MaxVarintLen64
	m := data.len()
	for ; n < m && p+binary.MaxVarintLen64 <= len(buf); n++ {
		p += binary.PutUvarint(buf[p:], uint64(data.get(n)))
	}

	// update buffer size
	//更新缓冲区大小
	binary.PutVarint(buf, int64(p))

	// write buffer
	// 写入缓冲区
	_, err = w.Write(buf[0:p])
	return
}

var errTooBig = errors.New("suffixarray: data too large")

// readSlice reads data[:n] from r and returns n.
// It uses buf to buffer the read.
// readSlice从r中读取数据[:n]并返回n。
// 它使用buf来缓冲读取的内容。
func readSlice(r io.Reader, buf []byte, data ints) (n int, err error) {
	// read buffer size
	// 读取缓冲区大小
	var size64 int64
	size64, err = readInt(r, buf)
	if err != nil {
		return
	}
	if int64(int(size64)) != size64 || int(size64) < 0 {
		// We never write chunks this big anyway.
		return 0, errTooBig
	}
	size := int(size64)

	// read buffer w/o the size
	// 读取缓冲区，不考虑其大小
	if _, err = io.ReadFull(r, buf[binary.MaxVarintLen64:size]); err != nil {
		return
	}

	// decode as many elements as present in buf
	// 对buf中存在的元素进行解码，越多越好。
	for p := binary.MaxVarintLen64; p < size; n++ {
		x, w := binary.Uvarint(buf[p:])
		data.set(n, int64(x))
		p += w
	}

	return
}

const bufSize = 16 << 10 // reasonable for BenchmarkSaveRestore	// 对BenchmarkSaveRestore来说是合理的

// Read reads the index from r into x; x must not be nil.
// 读取将索引从r中读入x中；x不能是nil。
func (x *Index) Read(r io.Reader) error {
	// buffer for all reads
	// 所有读数的缓冲区
	buf := make([]byte, bufSize)

	// read length
	// 读取长度
	n64, err := readInt(r, buf)
	if err != nil {
		return err
	}
	if int64(int(n64)) != n64 || int(n64) < 0 {
		return errTooBig
	}
	n := int(n64)

	// allocate space
	// 分配空间
	if 2*n < cap(x.data) || cap(x.data) < n || x.sa.int32 != nil && n > maxData32 || x.sa.int64 != nil && n <= maxData32 {
		// new data is significantly smaller or larger than
		// existing buffers - allocate new ones
		// 新数据明显小于或大于现有的缓冲区 - 分配新的缓冲区
		x.data = make([]byte, n)
		x.sa.int32 = nil
		x.sa.int64 = nil
		if n <= maxData32 {
			x.sa.int32 = make([]int32, n)
		} else {
			x.sa.int64 = make([]int64, n)
		}
	} else {
		// re-use existing buffers
		// 重新使用现有的缓冲区
		x.data = x.data[0:n]
		x.sa = x.sa.slice(0, n)
	}

	// read data
	// 读取数据
	if _, err := io.ReadFull(r, x.data); err != nil {
		return err
	}

	// read index
	// 读取索引
	sa := x.sa
	for sa.len() > 0 {
		n, err := readSlice(r, buf, sa)
		if err != nil {
			return err
		}
		sa = sa.slice(n, sa.len())
	}
	return nil
}

// Write writes the index x to w.
// 写下索引x到w。
func (x *Index) Write(w io.Writer) error {
	// buffer for all writes
	// 用于所有写的缓冲区
	buf := make([]byte, bufSize)

	// write length
	// 写入长度
	if err := writeInt(w, buf, len(x.data)); err != nil {
		return err
	}

	// write data
	// 写入数据
	if _, err := w.Write(x.data); err != nil {
		return err
	}

	// write index
	// 写入索引
	sa := x.sa
	for sa.len() > 0 {
		n, err := writeSlice(w, buf, sa)
		if err != nil {
			return err
		}
		sa = sa.slice(n, sa.len())
	}
	return nil
}

// Bytes returns the data over which the index was created.
// It must not be modified.
// Bytes返回创建索引所依据的数据。
// 它不能被修改。
//
func (x *Index) Bytes() []byte {
	return x.data
}

func (x *Index) at(i int) []byte {
	return x.data[x.sa.get(i):]
}

// lookupAll returns a slice into the matching region of the index.
// The runtime is O(log(N)*len(s)).
// lookupAll返回索引的匹配区域的片断。
// 运行时间为O(log(N)*len(s))。
func (x *Index) lookupAll(s []byte) ints {
	// find matching suffix index range [i:j]
	// find the first index where s would be the prefix
	// 找到匹配的后缀索引范围 [i:j]
	// 找到第一个索引，其中s是前缀
	i := sort.Search(x.sa.len(), func(i int) bool { return bytes.Compare(x.at(i), s) >= 0 })
	// starting at i, find the first index at which s is not a prefix
	// 从i开始，找到s不是前缀的第一个索引
	j := i + sort.Search(x.sa.len()-i, func(j int) bool { return !bytes.HasPrefix(x.at(j+i), s) })
	return x.sa.slice(i, j)
}

// Lookup returns an unsorted list of at most n indices where the byte string s
// occurs in the indexed data. If n < 0, all occurrences are returned.
// The result is nil if s is empty, s is not found, or n == 0.
// Lookup time is O(log(N)*len(s) + len(result)) where N is the
// size of the indexed data.
// Lookup返回一个未经排序的列表，该列表中最多有n个索引，在这些索引的数据中出现了字节串s。
// 如果n<0，则返回所有出现的数据。如果s是空的，s没有被找到，或者n == 0，则结果为nil。
// 查找时间为O(log(N)*len(s) + len(result))，其中N是索引数据的大小。
//
func (x *Index) Lookup(s []byte, n int) (result []int) {
	if len(s) > 0 && n != 0 {
		matches := x.lookupAll(s)
		count := matches.len()
		if n < 0 || count < n {
			n = count
		}
		// 0 <= n <= count
		if n > 0 {
			result = make([]int, n)
			if matches.int32 != nil {
				for i := range result {
					result[i] = int(matches.int32[i])
				}
			} else {
				for i := range result {
					result[i] = int(matches.int64[i])
				}
			}
		}
	}
	return
}

// FindAllIndex returns a sorted list of non-overlapping matches of the
// regular expression r, where a match is a pair of indices specifying
// the matched slice of x.Bytes(). If n < 0, all matches are returned
// in successive order. Otherwise, at most n matches are returned and
// they may not be successive. The result is nil if there are no matches,
// or if n == 0.
// FindAllIndex返回正则表达式r的非重叠匹配的排序列表，其中一个匹配是一对索引，指定x.Bytes()
// 的匹配切片。如果n<0，所有的匹配都会按顺序返回。否则，最多返回n个匹配项，它们可能不是连续的。
// 如果没有匹配，或者n == 0，则结果为nil。
//
func (x *Index) FindAllIndex(r *regexp.Regexp, n int) (result [][]int) {
	// a non-empty literal prefix is used to determine possible
	// match start indices with Lookup
	// 一个非空的字面前缀被用来确定可能与Lookup匹配的起始索引
	prefix, complete := r.LiteralPrefix()
	lit := []byte(prefix)

	// worst-case scenario: no literal prefix
	// 最坏的情况是：没有字面的前缀
	if prefix == "" {
		return r.FindAllIndex(x.data, n)
	}

	// if regexp is a literal just use Lookup and convert its
	// result into match pairs
	// 如果regexp是一个字面，只需使用Lookup并将其结果转换为匹配对。
	if complete {
		// Lookup returns indices that may belong to overlapping matches.
		// After eliminating them, we may end up with fewer than n matches.
		// If we don't have enough at the end, redo the search with an
		// increased value n1, but only if Lookup returned all the requested
		// indices in the first place (if it returned fewer than that then
		// there cannot be more).
		// 查询返回的指数可能属于重叠的匹配。在排除它们之后，我们最终可能会得到少于n个的匹配。
		// 如果我们最后没有足够的数量，就用增加的数值n1重新进行搜索，但前提是Lookup首先返回
		// 了所有要求的索引（如果它返回的数量少于这个数值，就不可能有更多的索引）。
		for n1 := n; ; n1 += 2 * (n - len(result)) /* overflow ok */ {
			indices := x.Lookup(lit, n1)
			if len(indices) == 0 {
				return
			}
			sort.Ints(indices)
			pairs := make([]int, 2*len(indices))
			result = make([][]int, len(indices))
			count := 0
			prev := 0
			for _, i := range indices {
				if count == n {
					break
				}
				// ignore indices leading to overlapping matches
				// 忽略导致重叠匹配的指数
				if prev <= i {
					j := 2 * count
					pairs[j+0] = i
					pairs[j+1] = i + len(lit)
					result[count] = pairs[j : j+2]
					count++
					prev = i + len(lit)
				}
			}
			result = result[0:count]
			if len(result) >= n || len(indices) != n1 {
				// found all matches or there's no chance to find more
				// (n and n1 can be negative)
				// 找到了所有的匹配，否则就没有机会找到更多的匹配（n和n1可以是负数）。
				break
			}
		}
		if len(result) == 0 {
			result = nil
		}
		return
	}

	// regexp has a non-empty literal prefix; Lookup(lit) computes
	// the indices of possible complete matches; use these as starting
	// points for anchored searches
	// (regexp "^" matches beginning of input, not beginning of line)
	// regexp有一个非空的字面前缀；Lookup(lit)计算可能的完整匹配的索引；使用这些索引作为锚定搜索的起点（regexp "^"匹配输入的开头，而不是行的开头）。
	r = regexp.MustCompile("^" + r.String()) // compiles because r compiled	// 编译了，因为R已经编译了

	// same comment about Lookup applies here as in the loop above
	// 关于Lookup的评论也适用于上述循环中。
	for n1 := n; ; n1 += 2 * (n - len(result)) /* overflow ok */ {
		indices := x.Lookup(lit, n1)
		if len(indices) == 0 {
			return
		}
		sort.Ints(indices)
		result = result[0:0]
		prev := 0
		for _, i := range indices {
			if len(result) == n {
				break
			}
			m := r.FindIndex(x.data[i:]) // anchored search - will not run off
			// ignore indices leading to overlapping matches
			// 忽略导致重叠匹配的指数
			if m != nil && prev <= i {
				m[0] = i // correct m
				m[1] += i
				result = append(result, m)
				prev = m[1]
			}
		}
		if len(result) >= n || len(indices) != n1 {
			// found all matches or there's no chance to find more
			// (n and n1 can be negative)
			// 找到了所有的匹配，否则就没有机会找到更多的匹配（n和n1可以是负数）。
			break
		}
	}
	if len(result) == 0 {
		result = nil
	}
	return
}
