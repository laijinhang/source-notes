```go
func Get(url string) (resp *Response, err error) {
	return DefaultClient.Get(url)
}
```