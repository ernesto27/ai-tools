package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"blog-api/internal/blog"
)

func main() {
	dataFile := os.Getenv("BLOG_DATA_FILE")
	if dataFile == "" {
		dataFile = "data/posts.json"
	}

	store, err := blog.NewFileStore(dataFile)
	if err != nil {
		log.Fatalf("open blog store: %v", err)
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	addr := ":" + strings.TrimPrefix(port, ":")

	server := &http.Server{
		Addr:              addr,
		Handler:           blog.NewHandler(store),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("blog API listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve blog API: %v", err)
	}
}
