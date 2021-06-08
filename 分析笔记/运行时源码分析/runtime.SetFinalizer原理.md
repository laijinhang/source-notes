参考文章：[深入理解Go-runtime.SetFinalizer原理剖析](https://juejin.cn/post/6844903937649147912)

finalizer是与对象关联的一个函数，通过`runtime.SetFinalizer`来设置，这个被设置的对象在GC的时候，这个finalizer会被调用。