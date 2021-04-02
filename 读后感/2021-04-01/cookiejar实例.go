package main

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
)

func main() {
	jar, _ := cookiejar.New(nil)
	client := http.Client{
		Jar: jar,
	}
	req, _ := http.NewRequest("GET", "http://127.0.0.1:10000/", nil)
	for i := 0; i < 10; i++ {
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("req %+v\n", req.Cookies())
		fmt.Printf("resp %+v\n", resp.Cookies())
	}
}
