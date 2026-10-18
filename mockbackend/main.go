package main

import (
	"fmt"
	"net/http"
	"time"
)

/*
Browser:
The server at localhost:8080 sent back an error: 502 Bad Gateway
Terminal:
2026/10/18 09:41:20 http: proxy error: dial tcp 127.0.0.1:9000: connect: connection refused
*/

func main() {
	s := &http.Server{
		Addr:           ":9000",
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Response from Mock Backend (:9000) \nURL:%s \nX-Forwarded-For: %s \n", r.URL.Path, r.Header.Get("X-Forwarded-For"))
	})

	fmt.Println(s.ListenAndServe())
}
