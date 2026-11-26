package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	address := os.Getenv("BLOG_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	path := os.Getenv("BLOG_DB_PATH")
	if path == "" {
		path = "blog.db"
	}

	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	db, err := openDatabase(startup, path)
	cancel()
	if err != nil {
		return err
	}
	defer db.Close()

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	server := &http.Server{
		Handler:           newHandler(postStore{db: db}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       time.Minute,
	}
	defer server.Close()
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	log.Printf("blog API listening on %s", listener.Addr())

	select {
	case err := <-served:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
