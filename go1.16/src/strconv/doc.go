// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package strconv implements conversions to and from string representations
// of basic data types.
// strconv包实现了基本数据类型的字符串表示形式之间的转换。
//
// Numeric Conversions
// 数值转换
//
// The most common numeric conversions are Atoi (string to int) and Itoa (int to string).
// 最常见的数字转换是Atoi（从字符串到int）和Itoa（从int到字符串）
//
//	i, err := strconv.Atoi("-42")
//	s := strconv.Itoa(-42)
//
// These assume decimal and the Go int type.
// 这些假定为十进制和Go int类型。
//
// ParseBool, ParseFloat, ParseInt, and ParseUint convert strings to values:
// ParseBool，ParseFloat，ParseInt和ParseUint将字符串转换为值：
//
//	b, err := strconv.ParseBool("true")
//	f, err := strconv.ParseFloat("3.1415", 64)
//	i, err := strconv.ParseInt("-42", 10, 64)
//	u, err := strconv.ParseUint("42", 10, 64)
//
// The parse functions return the widest type (float64, int64, and uint64),
// but if the size argument specifies a narrower width the result can be
// converted to that narrower type without data loss:
// 解析函数返回最宽的类型（float64，int64和uint64），但是如果size参数指定了较窄的宽度，则结果可以转换为该较窄的类型而不会造成数据丢失：
//
//	s := "2147483647" // biggest int32
//	i64, err := strconv.ParseInt(s, 10, 32)
//	...
//	i := int32(i64)
//
// FormatBool, FormatFloat, FormatInt, and FormatUint convert values to strings:
// FormatBool，FormatFloat，FormatInt和FormatUint将值转换为字符串：
//
//	s := strconv.FormatBool(true)
//	s := strconv.FormatFloat(3.1415, 'E', -1, 64)
//	s := strconv.FormatInt(-42, 16)
//	s := strconv.FormatUint(42, 16)
//
// AppendBool, AppendFloat, AppendInt, and AppendUint are similar but
// append the formatted value to a destination slice.
// Append Bool，Append Float，Append Int和AppendUint相似，但是将格式化后的值附加到目标切片中。
//
// String Conversions
// 字符串转换
//
// Quote and QuoteToASCII convert strings to quoted Go string literals.
// The latter guarantees that the result is an ASCII string, by escaping
// any non-ASCII Unicode with \u:
// Quote和QuoteToASCII将字符串转换为带引号的Go字符串文字。后者通过使用\ u转义任何非ASCII Unicode来保证结果是ASCII字符串：
//
//	q := strconv.Quote("Hello, 世界")
//	q := strconv.QuoteToASCII("Hello, 世界")
//
// QuoteRune and QuoteRuneToASCII are similar but accept runes and
// return quoted Go rune literals.
// QuoteRune和QuoteRuneToASCII相似，但是接受符文并返回加引号的Go符文文字。
//
// Unquote and UnquoteChar unquote Go string and rune literals.
// Unquote和UnquoteChar unquote Go字符串和符文文字。
package strconv
