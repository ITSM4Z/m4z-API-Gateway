package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	port := os.Args[1]
	s := &http.Server{
		Addr:           port,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Response from Mock Backend (%s) \nURL:%s \nX-Forwarded-For: %s \n", port, r.URL.Path, r.Header.Get("X-Forwarded-For"))
	})

	fmt.Println(s.ListenAndServe())
}
