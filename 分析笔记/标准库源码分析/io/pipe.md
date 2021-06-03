pipe包内部先是实现了一个非暴露的可读可写的pipe，pipe被翻译成管道，数据从写的地方流入，从读的地方流出，
用chan来实现数据流转，并配合互斥锁，达到并发安全，使用sync.Once来执行关闭操作，使得多次调用，
最多只执行一次。然后再封装了只写pipe和只读pipe，用Pipe创建的只读pipe和只写pipe是同一对，
他们底层是共用一个pipe。

通过Pipe()方法来创建一个只读pipe和只写pipe。

只写pipe有三个向外部暴露的接口Write，CloseWrite、CloseWriteError
只读pipe有三个向外部暴露的接口Read，CloseRead、CloseReadError