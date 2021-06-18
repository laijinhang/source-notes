// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import (
	"runtime"
)

// AllocsPerRun returns the average number of allocations during calls to f.
// Although the return value has type float64, it will always be an integral value.
// AllocsPerRun返回调用f时的平均分配次数。
// 虽然返回值的类型是float64，但它总是一个积分值。
//
// To compute the number of allocations, the function will first be run once as
// a warm-up. The average number of allocations over the specified number of
// runs will then be measured and returned.
// 为了计算分配的数量，该函数将首先运行一次warm-up。然后将测量并返回指定运行次数的平均分配数。
//
// AllocsPerRun sets GOMAXPROCS to 1 during its measurement and will restore
// it before returning.
// AllocsPerRun在测量期间将GOMAXPROCS设置为1，并在返回之前将其恢复。

/*
其工作原理：对指定函数执行n次，计算n次执行后的平均值

其工作过程：
1. 设置GOMAXPROCS为1
2. 尝试先运行一次
3. 用一个变量记录开始时的统计数据
4. 运行runs次
5. 读取结束时的统计数据
6. 将统计的次数 除于 runs次 来计算一次运行的情况
7. 将GOMAXPROCS为初始
*/
func AllocsPerRun(runs int, f func()) (avg float64) {
	/*
		刚开始的时候，会执行runtime.GOMAXPROCS(1)，设置了GOMAXPROCS为1，然后会返回原先的值，
		在AllocsPerRun执行完成的时候，会恢复到原型的
	*/
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))

	// Warm up the function
	// 先执行一下这个函数
	f()

	// Measure the starting statistics
	// Measure统计起始数据
	var memstats runtime.MemStats
	runtime.ReadMemStats(&memstats)
	mallocs := 0 - memstats.Mallocs

	// Run the function the specified number of times
	// 运行函数指定次
	for i := 0; i < runs; i++ {
		f()
	}

	// Read the final statistics
	// 读取结束时的统计数据
	runtime.ReadMemStats(&memstats)
	mallocs += memstats.Mallocs

	// Average the mallocs over the runs (not counting the warm-up).
	// We are forced to return a float64 because the API is silly, but do
	// the division as integers so we can ask if AllocsPerRun()==1
	// instead of AllocsPerRun()<2.
	// 在整个运行过程中平均分配mallocs（不计入Warm up）。
	// 我们被迫返回一个float64，因为API很傻，但是做除法时是整数，
	// 所以我们可以问AllocsPerRun()==1而不是AllocsPerRun()<2。
	return float64(mallocs / uint64(runs))
}
