// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package expvar provides a standardized interface to public variables, such
// as operation counters in servers. It exposes these variables via HTTP at
// /debug/vars in JSON format.
//
// Operations to set or modify these public variables are atomic.
//
// In addition to adding the HTTP handler, this package registers the
// following variables:
//
//	cmdline   os.Args
//	memstats  runtime.Memstats
//
// The package is sometimes only imported for the side effect of
// registering its HTTP handler and the above variables. To use it
// this way, link this package into your program:
//	import _ "expvar"
//

// 这个库用来暴露一些公共的变量，它会自动注册到 /debug/vars 这个路径下
// 这个库使用了原子操作，并发安全
package expvar

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Var is an abstract type for all exported variables.
// Var是所有导出变量的抽象类型。
type Var interface {
	// String返回变量的有效JSON值。带有不返回有效JSON（例如time.Time）的String方法的类型不得用作Var。
	String() string
}

// Int是一个满足Var接口的64位整数变量。
type Int struct {
	i int64
}

// 获取值
func (v *Int) Value() int64 {
	return atomic.LoadInt64(&v.i)
}

// 转成字符串
func (v *Int) String() string {
	return strconv.FormatInt(atomic.LoadInt64(&v.i), 10)
}

// 加
func (v *Int) Add(delta int64) {
	atomic.AddInt64(&v.i, delta)
}

// 设置值
func (v *Int) Set(value int64) {
	atomic.StoreInt64(&v.i, value)
}

// Float是一个满足Var接口的64位float变量。
type Float struct {
	f uint64
}

func (v *Float) Value() float64 {
	return math.Float64frombits(atomic.LoadUint64(&v.f))
}

func (v *Float) String() string {
	return strconv.FormatFloat(
		math.Float64frombits(atomic.LoadUint64(&v.f)), 'g', -1, 64)
}

// Add将增量添加到v。
func (v *Float) Add(delta float64) {
	for {
		cur := atomic.LoadUint64(&v.f)
		curVal := math.Float64frombits(cur)
		nxtVal := curVal + delta
		nxt := math.Float64bits(nxtVal)
		if atomic.CompareAndSwapUint64(&v.f, cur, nxt) {
			return
		}
	}
}

// 设置值
func (v *Float) Set(value float64) {
	atomic.StoreUint64(&v.f, math.Float64bits(value))
}

// Map是满足Var接口的字符串到Var映射变量。
type Map struct {
	m      sync.Map // map[string]Var
	keysMu sync.RWMutex
	keys   []string // sorted
}

// KeyValue表示Map中的单个条目。
type KeyValue struct {
	Key   string
	Value Var
}

func (v *Map) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "{")
	first := true
	v.Do(func(kv KeyValue) {
		if !first {
			fmt.Fprintf(&b, ", ")
		}
		fmt.Fprintf(&b, "%q: %v", kv.Key, kv.Value)
		first = false
	})
	fmt.Fprintf(&b, "}")
	return b.String()
}

// Init 从v.m中移除所有key值
func (v *Map) Init() *Map {
	v.keysMu.Lock()
	defer v.keysMu.Unlock()
	v.keys = v.keys[:0]
	v.m.Range(func(k, _ interface{}) bool {
		v.m.Delete(k)
		return true
	})
	return v
}

// addKey updates the sorted list of keys in v.keys.
func (v *Map) addKey(key string) {
	v.keysMu.Lock()
	defer v.keysMu.Unlock()
	// Using insertion sort to place key into the already-sorted v.keys.
	if i := sort.SearchStrings(v.keys, key); i >= len(v.keys) {
		v.keys = append(v.keys, key)
	} else if v.keys[i] != key {
		v.keys = append(v.keys, "")
		copy(v.keys[i+1:], v.keys[i:])
		v.keys[i] = key
	}
}

func (v *Map) Get(key string) Var {
	i, _ := v.m.Load(key)
	av, _ := i.(Var)
	return av
}

func (v *Map) Set(key string, av Var) {
	// Before we store the value, check to see whether the key is new. Try a Load
	// before LoadOrStore: LoadOrStore causes the key interface to escape even on
	// the Load path.
	if _, ok := v.m.Load(key); !ok {
		if _, dup := v.m.LoadOrStore(key, av); !dup {
			v.addKey(key)
			return
		}
	}

	v.m.Store(key, av)
}

// Add adds delta to the *Int value stored under the given map key.
func (v *Map) Add(key string, delta int64) {
	i, ok := v.m.Load(key)
	if !ok {
		var dup bool
		i, dup = v.m.LoadOrStore(key, new(Int))
		if !dup {
			v.addKey(key)
		}
	}

	// Add to Int; ignore otherwise.
	if iv, ok := i.(*Int); ok {
		iv.Add(delta)
	}
}

// AddFloat adds delta to the *Float value stored under the given map key.
func (v *Map) AddFloat(key string, delta float64) {
	i, ok := v.m.Load(key)
	if !ok {
		var dup bool
		i, dup = v.m.LoadOrStore(key, new(Float))
		if !dup {
			v.addKey(key)
		}
	}

	// Add to Float; ignore otherwise.
	if iv, ok := i.(*Float); ok {
		iv.Add(delta)
	}
}

// Delete deletes the given key from the map.
func (v *Map) Delete(key string) {
	v.keysMu.Lock()
	defer v.keysMu.Unlock()
	i := sort.SearchStrings(v.keys, key)
	if i < len(v.keys) && key == v.keys[i] {
		v.keys = append(v.keys[:i], v.keys[i+1:]...)
		v.m.Delete(key)
	}
}

// Do calls f for each entry in the map.
// The map is locked during the iteration,
// but existing entries may be concurrently updated.
func (v *Map) Do(f func(KeyValue)) {
	v.keysMu.RLock()
	defer v.keysMu.RUnlock()
	for _, k := range v.keys {
		i, _ := v.m.Load(k)
		f(KeyValue{k, i.(Var)})
	}
}

// String is a string variable, and satisfies the Var interface.
type String struct {
	s atomic.Value // string
}

func (v *String) Value() string {
	p, _ := v.s.Load().(string)
	return p
}

// String implements the Var interface. To get the unquoted string
// use Value.
func (v *String) String() string {
	s := v.Value()
	b, _ := json.Marshal(s)
	return string(b)
}

func (v *String) Set(value string) {
	v.s.Store(value)
}

// Func implements Var by calling the function
// and formatting the returned value using JSON.
type Func func() interface{}

func (f Func) Value() interface{} {
	return f()
}

func (f Func) String() string {
	v, _ := json.Marshal(f())
	return string(v)
}

// All published variables.
var (
	vars      sync.Map // map[string]Var
	varKeysMu sync.RWMutex
	varKeys   []string // sorted
)

// Publish declares a named exported variable. This should be called from a
// package's init function when it creates its Vars. If the name is already
// registered then this will log.Panic.
func Publish(name string, v Var) {
	if _, dup := vars.LoadOrStore(name, v); dup {
		log.Panicln("Reuse of exported var name:", name)
	}
	varKeysMu.Lock()
	defer varKeysMu.Unlock()
	varKeys = append(varKeys, name)
	sort.Strings(varKeys)
}

// Get retrieves a named exported variable. It returns nil if the name has
// not been registered.
func Get(name string) Var {
	i, _ := vars.Load(name)
	v, _ := i.(Var)
	return v
}

// 方便的功能用于创建新的导出变量。
func NewInt(name string) *Int {
	v := new(Int)
	Publish(name, v)
	return v
}

func NewFloat(name string) *Float {
	v := new(Float)
	Publish(name, v)
	return v
}

func NewMap(name string) *Map {
	v := new(Map).Init()
	Publish(name, v)
	return v
}

func NewString(name string) *String {
	v := new(String)
	Publish(name, v)
	return v
}

// 为每个导出的变量调用f。全局变量映射在迭代过程中被锁定，但是现有条目可能会同时更新。
func Do(f func(KeyValue)) {
	varKeysMu.RLock()
	defer varKeysMu.RUnlock()
	for _, k := range varKeys {
		val, _ := vars.Load(k)
		f(KeyValue{k, val.(Var)})
	}
}

func expvarHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, "{\n")
	first := true
	Do(func(kv KeyValue) {
		if !first {
			fmt.Fprintf(w, ",\n")
		}
		first = false
		fmt.Fprintf(w, "%q: %s", kv.Key, kv.Value)
	})
	fmt.Fprintf(w, "\n}\n")
}

// 处理程序返回expvar HTTP处理程序。
//
// 仅需要将处理程序安装在非标准位置。
func Handler() http.Handler {
	return http.HandlerFunc(expvarHandler)
}

func cmdline() interface{} {
	return os.Args
}

func memstats() interface{} {
	stats := new(runtime.MemStats)
	runtime.ReadMemStats(stats)
	return *stats
}

func init() {
	http.HandleFunc("/debug/vars", expvarHandler)
	Publish("cmdline", Func(cmdline))
	Publish("memstats", Func(memstats))
}
