package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	port := getPort()

	s := &http.Server{
		Addr:           port,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Response from Mock Backend (%s) \nURL:%s \nX-Forwarded-For: %s \n", port, r.URL.Path, r.Header.Get("X-Forwarded-For"))
	})

	if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func getPort() string {
	if len(os.Args) < 2 {
		if envPort := os.Getenv("PORT"); envPort != "" {
			return envPort
		} else {
			return ":9001"
		}
	} else {
		return os.Args[1]
	}
}
