package main

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
)

func main() {
	jar, _ := cookiejar.New(nil)
	client := http.Client{
		Jar:       jar,
		Transport: t{},
	}
	req, _ := http.NewRequest("GET", "http://127.0.0.1:10000/", nil)
	for i := 0; i < 10; i++ {
		_, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
}

type t struct {
}

func (t) RoundTrip(*http.Request) (*http.Response, error) {
	fmt.Println("123")
	return &http.Response{}, nil
}
