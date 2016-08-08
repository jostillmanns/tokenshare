package main

import (
	"log"
	"net/http"
)

func main() {
	server, err := newSrv("bolt.db", "token", "storage", "www", "user", "pass", 16, int64(1024*1024*1024))
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	if err := http.ListenAndServe(":8080", server.mux); err != nil {
		log.Fatalf("server: %v", err)
	}

}
