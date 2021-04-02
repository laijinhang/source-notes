package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		c := http.Cookie{
			Name:     "user",
			Value:    "12345",
			HttpOnly: true,
		}
		fmt.Println(request.Form)
		writer.Header().Set("Set-Cookie", c.String())
	})
	http.ListenAndServe(":10000", nil)
}
