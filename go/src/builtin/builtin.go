// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
	Package builtin provides documentation for Go's predeclared identifiers.
	The items documented here are not actually in package builtin
	but their descriptions here allow godoc to present documentation
	for the language's special identifiers.

	包内置为Go的预声明标识符提供了文档。这里记录的条目实际上并不是包内置的，
	但它们在这里的描述允许godoc为语言的特殊标识符提供文档。
*/
package builtin

// bool is the set of boolean values, true and false.
// bool是布尔值true和false的集合。
type bool bool

// true and false are the two untyped boolean values.
// true和false是两个无类型的布尔值。
const (
	true  = 0 == 0 // Untyped bool.
	false = 0 != 0 // Untyped bool.
)

// uint8 is the set of all unsigned 8-bit integers.
// Range: 0 through 255.
// uint8是所有无符号8位整数的集合。
// 取值范围:0 ~ 255。
type uint8 uint8

// uint16 is the set of all unsigned 16-bit integers.
// Range: 0 through 65535.
// uint16是所有16位无符号整数的集合。
// 取值范围:0 ~ 65535。
type uint16 uint16

// uint32 is the set of all unsigned 32-bit integers.
// Range: 0 through 4294967295.
// uint32是所有无符号32位整数的集合。
// 范围:0到4294967295。
type uint32 uint32

// uint64 is the set of all unsigned 64-bit integers.
// Range: 0 through 18446744073709551615.
// uint64是所有无符号64位整数的集合。
// 范围:0 ~ 18446744073709551615。
type uint64 uint64

// int8 is the set of all signed 8-bit integers.
// Range: -128 through 127.
// int8是所有带符号8位整数的集合。
// 范围:-128到127。
type int8 int8

// int16 is the set of all signed 16-bit integers.
// Range: -32768 through 32767.
// int16是所有16位有符号整数的集合。
// 范围:-32768到32767。
type int16 int16

// int32 is the set of all signed 32-bit integers.
// Range: -2147483648 through 2147483647.
// int32是所有带符号32位整数的集合。
// 范围: -2147483648~2147483647。
type int32 int32

// int64 is the set of all signed 64-bit integers.
// Range: -9223372036854775808 through 9223372036854775807.
// int64是所有有符号的64位整数的集合。
// 范围: -9223372036854775808~9223372036854775807.
type int64 int64

// float32 is the set of all IEEE-754 32-bit floating-point numbers.
type float32 float32

// float64 is the set of all IEEE-754 64-bit floating-point numbers.
// float64是所有IEEE-754 64位浮点数的集合。
type float64 float64

// complex64 is the set of all complex numbers with float32 real and
// imaginary parts.
// complex64是所有实部和虚部为float32的复数的集合。
type complex64 complex64

// complex128 is the set of all complex numbers with float64 real and
// imaginary parts.
// complex128是所有实部和虚部为float64的复数的集合。
type complex128 complex128

// string is the set of all strings of 8-bit bytes, conventionally but not
// necessarily representing UTF-8-encoded text. A string may be empty, but
// not nil. Values of string type are immutable.
// string是所有8位字节字符串的集合，通常但不一定代表UTF8编码文本。
// 字符串可以为空，但不能为零。字符串类型的值是不可变的。
type string string

// int is a signed integer type that is at least 32 bits in size. It is a
// distinct type, however, and not an alias for, say, int32.
// int是一种有符号整数类型，其大小至少为32位。
// 但是，它是一个不同的类型，而不是int32的别名
type int int

// uint is an unsigned integer type that is at least 32 bits in size. It is a
// distinct type, however, and not an alias for, say, uint32.
// uint是大小至少为32位的无符号整数类型。但是，
// 它是一种不同的类型，而不是uint32的别名。
type uint uint

// uintptr is an integer type that is large enough to hold the bit pattern of
// any pointer.
// uintptr是一个整数类型，其大小足以容纳任何指针的比特模式。
type uintptr uintptr

// byte is an alias for uint8 and is equivalent to uint8 in all ways. It is
// used, by convention, to distinguish byte values from 8-bit unsigned
// integer values.
// uintptr是一个整数类型，大到足以容纳任何指针的比特模式。byte是uint8的一个别名，在
// 所有方面都等同于uint8。按照惯例，它被用来区分字节值和8位无符号整数值。
type byte = uint8

// rune is an alias for int32 and is equivalent to int32 in all ways. It is
// used, by convention, to distinguish character values from integer values.
// rune是int32的一个别名，在所有方面都等同于int32。按照惯例，它被用来区分字符值和整数值。
type rune = int32

// iota is a predeclared identifier representing the untyped integer ordinal
// number of the current const specification in a (usually parenthesized)
// const declaration. It is zero-indexed.
// iota是一个预先声明的标识符，代表在一个（通常是括号内的）const声明中的当前const规范的未定型整数序号。它是零指数的。
const iota = 0 // Untyped int.

// nil is a predeclared identifier representing the zero value for a
// pointer, channel, func, interface, map, or slice type.
// nil是一个预先声明的标识符，代表一个指针、通道、func、接口、map或切片类型的零值。
var nil Type // Type must be a pointer, channel, func, interface, map, or slice type

// Type is here for the purposes of documentation only. It is a stand-in
// for any Go type, but represents the same type for any given function
// invocation.
// 类型在这里只是为了文档的目的。它是任何Go类型的替身，但在任何给定的函数调用中代表相同的类型。
type Type int

// Type1 is here for the purposes of documentation only. It is a stand-in
// for any Go type, but represents the same type for any given function
// invocation.
// Type1在这里只是为了说明问题。它是任何Go类型的替身，但在任何给定的函数调用中代表同一类型。
type Type1 int

// IntegerType is here for the purposes of documentation only. It is a stand-in
// for any integer type: int, uint, int8 etc.
// IntegerType在这里只是为了文档的目的。它是任何整数类型的替身：int、uint、int8等等。
type IntegerType int

// FloatType is here for the purposes of documentation only. It is a stand-in
// for either float type: float32 or float64.
// FloatType在这里只是为了文档的目的。它是浮点类型的替身：float32或float64。
type FloatType float32

// ComplexType is here for the purposes of documentation only. It is a
// stand-in for either complex type: complex64 or complex128.
// ComplexType在这里只是为了文档的目的。它是复数类型的代名词：complex64或complex128。
type ComplexType complex64

// The append built-in function appends elements to the end of a slice. If
// it has sufficient capacity, the destination is resliced to accommodate the
// new elements. If it does not, a new underlying array will be allocated.
// Append returns the updated slice. It is therefore necessary to store the
// result of append, often in the variable holding the slice itself:
//	slice = append(slice, elem1, elem2)
//	slice = append(slice, anotherSlice...)
// As a special case, it is legal to append a string to a byte slice, like this:
//	slice = append([]byte("hello "), "world"...)

// append内置函数将元素追加到片断的末尾。如果它有足够的容量，目的地将被重新切分以容纳新的元素。
// 如果没有，将分配一个新的底层数组。
// Append返回更新后的切片。
// 因此，有必要存储append的结果，通常是在保存切片本身的变量中。
//	slice = append(slice, elem1, elem2)
//	slice = append(slice, anotherSlice...)
// 作为一种特殊情况，将一个字符串追加到一个字节片上是合法的，像这样。
//	slice = append([]byte("hello"), "world"...)
func append(slice []Type, elems ...Type) []Type

// The copy built-in function copies elements from a source slice into a
// destination slice. (As a special case, it also will copy bytes from a
// string to a slice of bytes.) The source and destination may overlap. Copy
// returns the number of elements copied, which will be the minimum of
// len(src) and len(dst).
// 内置函数copy将源片中的元素复制到目标片中。(作为一种特殊情况，它也会从一个字符串复制字节到一个字节片中)。
// 源切片和目的切片可以重合。复制返回被复制元素的数量，这将是len(src)和len(dst)的最小值。
func copy(dst, src []Type) int

// The delete built-in function deletes the element with the specified key
// (m[key]) from the map. If m is nil or there is no such element, delete
// is a no-op.
// 内置函数delete从map中删除具有指定键（m[key]）的元素。
// 如果m是nil或者没有这样的元素，delete就是一个无用操作。
func delete(m map[Type]Type1, key Type)

// The len built-in function returns the length of v, according to its type:
//	Array: the number of elements in v.
//	Pointer to array: the number of elements in *v (even if v is nil).
//	Slice, or map: the number of elements in v; if v is nil, len(v) is zero.
//	String: the number of bytes in v.
//	Channel: the number of elements queued (unread) in the channel buffer;
//	         if v is nil, len(v) is zero.
// For some arguments, such as a string literal or a simple array expression, the
// result can be a constant. See the Go language specification's "Length and
// capacity" section for details.

// 内置函数len根据v的类型，返回v的长度
// 数组：v中元素的数量
// 指向数组的指针：*v中的元素数（即使v为nil）
// 切片或map：v中的元素数；如果v为nil，len(v)为零
// 通道：通道缓冲区中未读的元素的个数；如果v为nil，len(v)为零
//
// 对于某些参数，如字符串字面意义或简单的数组表达式，其结果可以是一个常数。
// 详见Go语言规范的 "长度和容量 "部分。
func len(v Type) int

// The cap built-in function returns the capacity of v, according to its type:
//	Array: the number of elements in v (same as len(v)).
//	Pointer to array: the number of elements in *v (same as len(v)).
//	Slice: the maximum length the slice can reach when resliced;
//	if v is nil, cap(v) is zero.
//	Channel: the channel buffer capacity, in units of elements;
//	if v is nil, cap(v) is zero.
// For some arguments, such as a simple array expression, the result can be a
// constant. See the Go language specification's "Length and capacity" section for
// details.
// 内置函数cap根据v的类型，返回v的容量。
// 	数组：v中的元素数（与len(v)相同）。
// 	指向数组的指针：*v中的元素数（与len(v)相同）。
// 	切片：切片在重新切分时能达到的最大长度。
//		如果v为nil，cap(v)为零。
// 通道：通道缓冲区的容量，以元素为单位。
// 		如果v为nil，cap(v)为零。
// 对于某些参数，如简单的数组表达式，其结果可以是一个常数。
// 详见Go语言规范中的 "长度和容量 "部分。
func cap(v Type) int

// The make built-in function allocates and initializes an object of type
// slice, map, or chan (only). Like new, the first argument is a type, not a
// value. Unlike new, make's return type is the same as the type of its
// argument, not a pointer to it. The specification of the result depends on
// the type:
//	Slice: The size specifies the length. The capacity of the slice is
//	equal to its length. A second integer argument may be provided to
//	specify a different capacity; it must be no smaller than the
//	length. For example, make([]int, 0, 10) allocates an underlying array
//	of size 10 and returns a slice of length 0 and capacity 10 that is
//	backed by this underlying array.
//	Map: An empty map is allocated with enough space to hold the
//	specified number of elements. The size may be omitted, in which case
//	a small starting size is allocated.
//	Channel: The channel's buffer is initialized with the specified
//	buffer capacity. If zero, or the size is omitted, the channel is
//	unbuffered.
// 内置函数make分配并初始化一个slice、map或chan（仅）类型的对象。和new一样，第一个参数是一个类型，而不是一个值。
// 与new不同，make的返回类型与它的参数类型相同，而不是一个指向它的指针。结果的规格取决于类型。
// 	切片：大小指定了长度。分片的容量等于它的长度。可以提供第二个整数参数来指定一个不同的容量；它必须不小于长度。
//		例如，make([]int, 0, 10)分配了一个大小为10的底层数组，并返回一个长度为0、容量为10的片，该片由这个底层数组支持。
// 	map：一个空的地图被分配了足够的空间来容纳指定数量的元素。大小可以省略，在这种情况下，会分配一个小的起始大小。
// 	通道：通道的缓冲区以指定的缓冲区容量被初始化。如果是零，或者省略了大小，通道就没有缓冲。
func make(t Type, size ...IntegerType) Type

// The new built-in function allocates memory. The first argument is a type,
// not a value, and the value returned is a pointer to a newly
// allocated zero value of that type.
// 内置函数new分配了内存。第一个参数是一个类型，而不是一个值，返回的值是一个指向该类型的新分配的零值的指针。
func new(Type) *Type

// The complex built-in function constructs a complex value from two
// floating-point values. The real and imaginary parts must be of the same
// size, either float32 or float64 (or assignable to them), and the return
// value will be the corresponding complex type (complex64 for float32,
// complex128 for float64).
// 内置函数complex从两个浮点值中构造一个复数值。实部和虚部的大小必须相同，要么是float32，
// 要么是float64（或可分配给它们），返回值将是相应的复数类型（float32为复数64，float64为复数128）。
func complex(r, i FloatType) ComplexType

// The real built-in function returns the real part of the complex number c.
// The return value will be floating point type corresponding to the type of c.
// 内置函数real返回复数c的实部。
// 返回值将是与c的类型相对应的浮点类型。
func real(c ComplexType) FloatType

// The imag built-in function returns the imaginary part of the complex
// number c. The return value will be floating point type corresponding to
// the type of c.
// 内置函数imag返回复数c的虚部，返回值将是与c的类型相对应的浮点类型。
func imag(c ComplexType) FloatType

// The close built-in function closes a channel, which must be either
// bidirectional or send-only. It should be executed only by the sender,
// never the receiver, and has the effect of shutting down the channel after
// the last sent value is received. After the last value has been received
// from a closed channel c, any receive from c will succeed without
// blocking, returning the zero value for the channel element. The form
//	x, ok := <-c
// will also set ok to false for a closed channel.
// 内置函数close关闭一个通道，该通道必须是双向的或只发送的。它只能由发送方执行，
// 而不能由接收方执行，其效果是在收到最后一个发送值后关闭通道。在从关闭的通道c中收到最后一个值后，
// 从c中的任何接收都将成功而不被阻塞，返回通道元素的零值。x, ok := <-c的形式也会将一个封闭通道的ok设置为false。
func close(c chan<- Type)

// The panic built-in function stops normal execution of the current
// goroutine. When a function F calls panic, normal execution of F stops
// immediately. Any functions whose execution was deferred by F are run in
// the usual way, and then F returns to its caller. To the caller G, the
// invocation of F then behaves like a call to panic, terminating G's
// execution and running any deferred functions. This continues until all
// functions in the executing goroutine have stopped, in reverse order. At
// that point, the program is terminated with a non-zero exit code. This
// termination sequence is called panicking and can be controlled by the
// built-in function recover.
// 内置函数panic停止当前goroutine的正常执行。当一个函数F调用panic时，F的正常执行立即停止。
// 任何被F推迟执行的函数都以常规方式运行，然后F返回给它的调用者。对于调用者G来说，
// 对F的调用就像对panic的调用一样，终止G的执行并运行任何延迟的函数。
// 这种情况一直持续到执行中的goroutine的所有函数都停止，顺序相反。
// 在这一点上，程序以非零的退出代码终止。这个终止序列被称为panicking，
// 可以通过内置函数recover来控制。
func panic(v interface{})

// The recover built-in function allows a program to manage behavior of a
// panicking goroutine. Executing a call to recover inside a deferred
// function (but not any function called by it) stops the panicking sequence
// by restoring normal execution and retrieves the error value passed to the
// call of panic. If recover is called outside the deferred function it will
// not stop a panicking sequence. In this case, or when the goroutine is not
// panicking, or if the argument supplied to panic was nil, recover returns
// nil. Thus the return value from recover reports whether the goroutine is
// panicking.
// 内置函数recover允许程序管理一个恐慌的goroutine的行为。
// 在一个递延函数内执行对recover的调用（但不是由它调用的任何函数），
// 通过恢复正常的执行来停止恐慌序列，并检索传递给panic调用的错误值。
// 如果recovery在递延函数之外被调用，它将不会停止恐慌序列。在这种情况下，
// 或者当goroutine没有恐慌时，或者如果提供给panic的参数是空的，recover返回空。
// 因此，recover的返回值报告了goroutine是否正在恐慌。
func recover() interface{}

// The print built-in function formats its arguments in an
// implementation-specific way and writes the result to standard error.
// Print is useful for bootstrapping and debugging; it is not guaranteed
// to stay in the language.
// 内置函数print以一种特定的实现方式格式化其参数，并将结果写入标准错误。
// print对于引导和调试是很有用的；它不能保证留在语言中。
func print(args ...Type)

// The println built-in function formats its arguments in an
// implementation-specific way and writes the result to standard error.
// Spaces are always added between arguments and a newline is appended.
// Println is useful for bootstrapping and debugging; it is not guaranteed
// to stay in the language.
// 内置函数println以一种特定的实现方式格式化其参数，并将结果写入标准错误。
// 参数之间总是添加空格，并附加一个换行符。Println对于引导和调试是很有用的；它不能保证在语言中保留。
func println(args ...Type)

// The error built-in interface type is the conventional interface for
// representing an error condition, with the nil value representing no error.
// 内置接口类型error是代表错误条件的常规接口，nil值代表没有错误。
type error interface {
	Error() string
}
