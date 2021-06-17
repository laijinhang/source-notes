select 和 switch 虽然在用法上很相识，都有case，但是select中的case只能是chan的收发操作

select上有两个很有趣的现象：
1. `select`能在channel上进行非阻塞的收发操作
2. `select`在遇到多个channel同时响应时，会随机执行一种情况

**非阻塞的收发操作：**
当存在default时，如果有case满足了，则会执行case的
如果case不满足，那么就执行default

**随机执行：**
多个case，多个case都满足时，会随机执行一个case


