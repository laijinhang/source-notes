1. 开始
2. NewRequest
3. (*Client).do
4. (*Client).send
5. send
6. (*Transport).getConn
7. (*persistConn).roundTrip
8. 结束


```go
func Get(url string) (resp *Response, err error) {
	return DefaultClient.Get(url)
}
```