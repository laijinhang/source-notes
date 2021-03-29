// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package strconv

const fnParseComplex = "ParseComplex"

// convErr splits an error returned by parseFloatPrefix
// into a syntax or range error for ParseComplex.
// convErr将parseFloatPrefix返回的错误
// 拆分为ParseComplex的语法或范围错误。
func convErr(err error, s string) (syntax, range_ error) {
	if x, ok := err.(*NumError); ok {
		x.Func = fnParseComplex
		x.Num = s
		if x.Err == ErrRange {
			return nil, x
		}
	}
	return err, nil
}

// ParseComplex converts the string s to a complex number
// with the precision specified by bitSize: 64 for complex64, or 128 for complex128.
// When bitSize=64, the result still has type complex128, but it will be
// convertible to complex64 without changing its value.
// ParseComplex用bitSize指定的精度将字符串s转换为复数：
// complex64为64，complex128为128。
// 当bitSize=64时，结果仍然具有complex128类型，
// 但是可以将其转换为complex64而无需更改其值。
//
// The number represented by s must be of the form N, Ni, or N±Ni, where N stands
// for a floating-point number as recognized by ParseFloat, and i is the imaginary
// component. If the second N is unsigned, a + sign is required between the two components
// as indicated by the ±. If the second N is NaN, only a + sign is accepted.
// The form may be parenthesized and cannot contain any spaces.
// The resulting complex number consists of the two components converted by ParseFloat.
// 用s表示的数字必须采用N，Ni或N±Ni的形式，其中N代表ParseFloat识别的浮点数，而i是虚部。
// 如果第二个N是无符号的，则在两个分量之间需要一个+号，如±所示。
// 如果第二个N为NaN，则仅接受+号。表格可以用括号括起来，不能包含任何空格。
// 生成的复数由ParseFloat转换的两个组件组成。
//
// The errors that ParseComplex returns have concrete type *NumError
// and include err.Num = s.
// ParseComplex返回的错误的具体类型为* NumError并包含err.Num = s。
//
// If s is not syntactically well-formed, ParseComplex returns err.Err = ErrSyntax.
// 如果s在语法上不正确，则ParseComplex返回err.Err = ErrSyntax。
//
// If s is syntactically well-formed but either component is more than 1/2 ULP
// away from the largest floating point number of the given component's size,
// ParseComplex returns err.Err = ErrRange and c = ±Inf for the respective component.
// 如果s的语法格式正确，但是任何一个组件距离给定组件大小的最大浮点数都超过1/2 ULP，
// 则ParseComplex返回err.Err = ErrRange和c =±Inf。
func ParseComplex(s string, bitSize int) (complex128, error) {
	size := 64
	if bitSize == 64 {
		size = 32 // complex64 uses float32 parts
	}

	orig := s

	// Remove parentheses, if any.
	// 删除括号（如果有）.
	if len(s) >= 2 && s[0] == '(' && s[len(s)-1] == ')' {
		s = s[1 : len(s)-1]
	}

	// 待处理的范围错误，或者为nil
	var pending error // pending range error, or nil

	// Read real part (possibly imaginary part if followed by 'i').
	// 读取实部（如果后面跟着“ i”，则可能是虚部）。
	re, n, err := parseFloatPrefix(s, size)
	if err != nil {
		err, pending = convErr(err, orig)
		if err != nil {
			return 0, err
		}
	}
	s = s[n:]

	// If we have nothing left, we're done.
	//如果我们一无所有，那就完成了。
	if len(s) == 0 {
		return complex(re, 0), pending
	}

	// Otherwise, look at the next character.
	// 否则，看下一个字符
	switch s[0] {
	case '+':
		// Consume the '+' to avoid an error if we have "+NaNi", but
		// do this only if we don't have a "++" (don't hide that error).
		// 如果我们有“ + NaNi”，请使用“ +”来避免错误，但是只有当我们没有“ ++”时才这样做（不要隐藏该错误）。
		if len(s) > 1 && s[1] != '+' {
			s = s[1:]
		}
	case '-':
		// ok
	case 'i':
		// If 'i' is the last character, we only have an imaginary part.
		// 如果'i'是最后一个字符，则我们只有一个虚构部分。
		if len(s) == 1 {
			return complex(0, re), pending
		}
		fallthrough
	default:
		return 0, syntaxError(fnParseComplex, orig)
	}

	// Read imaginary part.
	im, n, err := parseFloatPrefix(s, size)
	if err != nil {
		err, pending = convErr(err, orig)
		if err != nil {
			return 0, err
		}
	}
	s = s[n:]
	if s != "i" {
		return 0, syntaxError(fnParseComplex, orig)
	}
	return complex(re, im), pending
}
