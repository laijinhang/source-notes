// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io_test

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

/*
Copy的案例
1. 创建一个strings.NewReader
2. 将strings.NewReader内容拷贝到标准输出中
*/
func ExampleCopy() {
	r := strings.NewReader("some io.Reader stream to be read\n")

	if _, err := io.Copy(os.Stdout, r); err != nil {
		log.Fatal(err)
	}

	// Output:
	// some io.Reader stream to be read
}

/*
CopyBuffer的案例
1. 创建r1，r2 strings.NewReader
2. io.CopyBuffer
*/
func ExampleCopyBuffer() {
	r1 := strings.NewReader("first reader\n")
	r2 := strings.NewReader("second reader\n")
	buf := make([]byte, 8)

	// buf is used here...
	if _, err := io.CopyBuffer(os.Stdout, r1, buf); err != nil {
		log.Fatal(err)
	}

	// ... reused here also. No need to allocate an extra buffer.
	if _, err := io.CopyBuffer(os.Stdout, r2, buf); err != nil {
		log.Fatal(err)
	}

	// Output:
	// first reader
	// second reader
}

/*
CopyN的案例
*/
func ExampleCopyN() {
	r := strings.NewReader("some io.Reader stream to be read")

	/*
		每次从r读取至多4个字节的数据，然后写入到os.Stdout，依次重复这个过程，直到r中读取完
	*/
	if _, err := io.CopyN(os.Stdout, r, 4); err != nil {
		log.Fatal(err)
	}

	// Output:
	// some
}

/*
ReadAtLeast的案例：
场景一：r的数据 > buf > min	正常
场景二：r的数据 > min > buf	short buffer 的错误
场景三：min > r的数据			unexpected EOF 的错误
*/
func ExampleReadAtLeast() {
	r := strings.NewReader("some io.Reader stream to be read\n")

	buf := make([]byte, 14)
	if _, err := io.ReadAtLeast(r, buf, 4); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", buf)

	// buffer smaller than minimal read size.
	// 缓冲区小于最小读取大小。
	shortBuf := make([]byte, 3)
	if _, err := io.ReadAtLeast(r, shortBuf, 4); err != nil {
		fmt.Println("error:", err)
	}

	// minimal read size bigger than io.Reader stream
	// 最小读取范围大于io.Reader流
	longBuf := make([]byte, 64)
	if _, err := io.ReadAtLeast(r, longBuf, 64); err != nil {
		fmt.Println("error:", err)
	}

	// Output:
	// some io.Reader
	// error: short buffer
	// error: unexpected EOF
}

/*
ReadFull的案例：
场景一：如果 r的数据长度 >= len(buf)，则正常读取，不会返回错误
场景二：如果 r的数据长度 < len(buf)，则会返回 unexpected EOF 的错误
*/
func ExampleReadFull() {
	r := strings.NewReader("some io.Reader stream to be read\n")

	buf := make([]byte, 4)
	if _, err := io.ReadFull(r, buf); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", buf)

	// minimal read size bigger than io.Reader stream
	// 最小读取 大于io.Reader流
	longBuf := make([]byte, 64)
	if _, err := io.ReadFull(r, longBuf); err != nil {
		fmt.Println("error:", err)
	}

	// Output:
	// some
	// error: unexpected EOF
}

/*
WriteString的案例
*/
func ExampleWriteString() {
	if _, err := io.WriteString(os.Stdout, "Hello World"); err != nil {
		log.Fatal(err)
	}

	// Output: Hello World
}

/*
LimitReader的案例
*/
func ExampleLimitReader() {
	r := strings.NewReader("some io.Reader stream to be read\n")
	lr := io.LimitReader(r, 4)

	if _, err := io.Copy(os.Stdout, lr); err != nil {
		log.Fatal(err)
	}

	// Output:
	// some
}

/*
MultiReader的案例
*/
func ExampleMultiReader() {
	r1 := strings.NewReader("first reader ")
	r2 := strings.NewReader("second reader ")
	r3 := strings.NewReader("third reader\n")
	r := io.MultiReader(r1, r2, r3)

	if _, err := io.Copy(os.Stdout, r); err != nil {
		log.Fatal(err)
	}

	// Output:
	// first reader second reader third reader
}

/*
TeeReader的案例
*/
func ExampleTeeReader() {
	var r io.Reader = strings.NewReader("some io.Reader stream to be read\n")

	r = io.TeeReader(r, os.Stdout)

	// Everything read from r will be copied to stdout.
	// 从r读取的所有内容都将被复制到stdout。
	io.ReadAll(r)

	// Output:
	// some io.Reader stream to be read
}

/*
SectionReader的案例
场景：将r数据从第五个到十七个拷贝到os.Stdout上
*/
func ExampleSectionReader() {
	r := strings.NewReader("some io.Reader stream to be read\n")
	s := io.NewSectionReader(r, 5, 17)

	if _, err := io.Copy(os.Stdout, s); err != nil {
		log.Fatal(err)
	}

	// Output:
	// io.Reader stream
}

func ExampleSectionReader_ReadAt() {
	r := strings.NewReader("some io.Reader stream to be read\n")
	s := io.NewSectionReader(r, 5, 17)

	buf := make([]byte, 6)
	if _, err := s.ReadAt(buf, 10); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s\n", buf)

	// Output:
	// stream
}

func ExampleSectionReader_Seek() {
	r := strings.NewReader("some io.Reader stream to be read\n")
	s := io.NewSectionReader(r, 5, 17)

	if _, err := s.Seek(10, io.SeekStart); err != nil {
		log.Fatal(err)
	}

	if _, err := io.Copy(os.Stdout, s); err != nil {
		log.Fatal(err)
	}

	// Output:
	// stream
}

/*
Seeker_Seek的案例
*/
func ExampleSeeker_Seek() {
	r := strings.NewReader("some io.Reader stream to be read\n")

	r.Seek(5, io.SeekStart) // move to the 5th char from the start	// 开始位置第五个字符开始
	if _, err := io.Copy(os.Stdout, r); err != nil {
		log.Fatal(err)
	}

	// len(r) - 5 的位置开始读
	r.Seek(-5, io.SeekEnd)
	if _, err := io.Copy(os.Stdout, r); err != nil {
		log.Fatal(err)
	}

	// Output:
	// io.Reader stream to be read
	// read
}

/*
MultiWriter的案例

有点类似，一个份数据，向n个地方写入
*/
func ExampleMultiWriter() {
	r := strings.NewReader("some io.Reader stream to be read\n")

	var buf1, buf2 bytes.Buffer
	w := io.MultiWriter(&buf1, &buf2)

	if _, err := io.Copy(w, r); err != nil {
		log.Fatal(err)
	}

	fmt.Print(buf1.String())
	fmt.Print(buf2.String())

	// Output:
	// some io.Reader stream to be read
	// some io.Reader stream to be read
}

func ExamplePipe() {
	r, w := io.Pipe()

	go func() {
		fmt.Fprint(w, "some io.Reader stream to be read\n")
		w.Close()
	}()

	if _, err := io.Copy(os.Stdout, r); err != nil {
		log.Fatal(err)
	}

	// Output:
	// some io.Reader stream to be read
}

func ExampleReadAll() {
	r := strings.NewReader("Go is a general-purpose language designed with systems programming in mind.")

	b, err := io.ReadAll(r)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s", b)

	// Output:
	// Go is a general-purpose language designed with systems programming in mind.
}
