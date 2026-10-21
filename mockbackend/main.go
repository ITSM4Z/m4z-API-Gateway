package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	s := &http.Server{
		Addr:           os.Args[1],
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Response from Mock Backend (%s) \nURL:%s \nX-Forwarded-For: %s \n", os.Args[1], r.URL.Path, r.Header.Get("X-Forwarded-For"))
	})

	fmt.Println(s.ListenAndServe())
}
