// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package strings

// stringFinder efficiently finds strings in a source text. It's implemented
// using the Boyer-Moore string search algorithm:
// https://en.wikipedia.org/wiki/Boyer-Moore_string_search_algorithm
// https://www.cs.utexas.edu/~moore/publications/fstrpos.pdf (note: this aged
// document uses 1-based indexing)
// stringFinder可以有效地查找源文本中的字符串。它是用Boyer-Moore字符串搜索算法实现的。
// https://en.wikipedia.org/wiki/Boyer-Moore_string_search_algorithm
// https://www.cs.utexas.edu/~moore/publications/fstrpos.pdf（注意：这个古老的文件使用基于1的索引）。
type stringFinder struct {
	// pattern is the string that we are searching for in the text.
	// pattern是我们要在文本中搜索的字符串。
	pattern string

	// badCharSkip[b] contains the distance between the last byte of pattern
	// and the rightmost occurrence of b in pattern. If b is not in pattern,
	// badCharSkip[b] is len(pattern).
	// badCharSkip[b]包含图案的最后一个字节与图案中最右边的b之间的距离。如果b不在图案中，
	// badCharSkip[b]就是len(pattern)。
	//
	// Whenever a mismatch is found with byte b in the text, we can safely
	// shift the matching frame at least badCharSkip[b] until the next time
	// the matching char could be in alignment.
	// 每当发现与文本中的字节b不匹配时，我们可以安全地将匹配的帧至少移动到badCharSkip[b]，
	// 直到下一次匹配的字符可以对齐。
	badCharSkip [256]int

	// goodSuffixSkip[i] defines how far we can shift the matching frame given
	// that the suffix pattern[i+1:] matches, but the byte pattern[i] does
	// not. There are two cases to consider:
	// goodSuffixSkip[i]定义了在后缀模式[i+1:]匹配，但字节模式[i]不匹配的情况下，
	// 我们可以将匹配的框架转移多远。有两种情况需要考虑。
	//
	// 1. The matched suffix occurs elsewhere in pattern (with a different
	// byte preceding it that we might possibly match). In this case, we can
	// shift the matching frame to align with the next suffix chunk. For
	// example, the pattern "mississi" has the suffix "issi" next occurring
	// (in right-to-left order) at index 1, so goodSuffixSkip[3] ==
	// shift+len(suffix) == 3+4 == 7.
	// 1. 匹配的后缀在模式中的其他地方出现（前面有一个不同的字节，我们可能会匹配）。在这种情况下，
	// 我们可以将匹配的帧移到与下一个后缀块对齐。例如，模式 "mississi "的后缀 "issi "在索引1处
	// 出现（按从右到左的顺序），所以goodSuffixSkip[3] == shift+len(senix) == 3+4 == 7。
	//
	// 2. If the matched suffix does not occur elsewhere in pattern, then the
	// matching frame may share part of its prefix with the end of the
	// matching suffix. In this case, goodSuffixSkip[i] will contain how far
	// to shift the frame to align this portion of the prefix to the
	// suffix. For example, in the pattern "abcxxxabc", when the first
	// mismatch from the back is found to be in position 3, the matching
	// suffix "xxabc" is not found elsewhere in the pattern. However, its
	// rightmost "abc" (at position 6) is a prefix of the whole pattern, so
	// goodSuffixSkip[3] == shift+len(suffix) == 6+5 == 11.
	// 2. 如果匹配的后缀在模式中没有出现，那么匹配的框架可能与匹配后缀的结尾共享其部分前缀。
	// 在这种情况下，goodSuffixSkip[i]将包含将框架移动多远以使这部分前缀与后缀对齐。例如，
	// 在模式 "abcxxxabc "中，当发现后面的第一个错位在第3位时，匹配的后缀 "xxabc "在该模
	// 式的其他地方没有发现。然而，它最右边的 "abc"（位于第6位）是整个模式的前缀，所以
	// goodSuffixSkip[3]==shift+len(后缀) ==6+5 ==11。
	goodSuffixSkip []int
}

func makeStringFinder(pattern string) *stringFinder {
	f := &stringFinder{
		pattern:        pattern,
		goodSuffixSkip: make([]int, len(pattern)),
	}
	// last is the index of the last character in the pattern.
	// last是模式中最后一个字符的索引。
	last := len(pattern) - 1

	// Build bad character table.
	// Bytes not in the pattern can skip one pattern's length.
	// 建立bad character。不在模式中的字节可以跳过一个模式的长度。
	for i := range f.badCharSkip {
		f.badCharSkip[i] = len(pattern)
	}
	// The loop condition is < instead of <= so that the last byte does not
	// have a zero distance to itself. Finding this byte out of place implies
	// that it is not in the last position.
	for i := 0; i < last; i++ {
		f.badCharSkip[pattern[i]] = last - i
	}

	// Build good suffix table.
	// First pass: set each value to the next index which starts a prefix of
	// pattern.
	// 建立好的后缀表。第一遍：将每个值设置为下一个索引，该索引开始一个模式的前缀。
	lastPrefix := last
	for i := last; i >= 0; i-- {
		if HasPrefix(pattern, pattern[i+1:]) {
			lastPrefix = i + 1
		}
		// lastPrefix is the shift, and (last-i) is len(suffix).
		// lastPrefix是移位，(last-i)是len(senix)。
		f.goodSuffixSkip[i] = lastPrefix + last - i
	}
	// Second pass: find repeats of pattern's suffix starting from the front.
	// 第二遍：从前面开始寻找图案后缀的重复部分。
	for i := 0; i < last; i++ {
		lenSuffix := longestCommonSuffix(pattern, pattern[1:i+1])
		if pattern[i-lenSuffix] != pattern[last-lenSuffix] {
			// (last-i) is the shift, and lenSuffix is len(suffix).
			// (last-i)是移位，lenSuffix是len(后缀)。
			f.goodSuffixSkip[last-lenSuffix] = lenSuffix + last - i
		}
	}

	return f
}

func longestCommonSuffix(a, b string) (i int) {
	for ; i < len(a) && i < len(b); i++ {
		if a[len(a)-1-i] != b[len(b)-1-i] {
			break
		}
	}
	return
}

// next returns the index in text of the first occurrence of the pattern. If
// the pattern is not found, it returns -1.
// 接下来返回该模式第一次出现在文本中的索引。如果没有找到该模式，则返回-1。
func (f *stringFinder) next(text string) int {
	i := len(f.pattern) - 1
	for i < len(text) {
		// Compare backwards from the end until the first unmatching character.
		// 从最后开始向后比较，直到第一个不匹配的字符。
		j := len(f.pattern) - 1
		for j >= 0 && text[i] == f.pattern[j] {
			i--
			j--
		}
		if j < 0 {
			return i + 1 // match
		}
		i += max(f.badCharSkip[text[i]], f.goodSuffixSkip[j])
	}
	return -1
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
