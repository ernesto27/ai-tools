package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type post struct {
	ID        uint64    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type postInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type blogAPI struct {
	mu     sync.RWMutex
	posts  map[uint64]post
	nextID uint64
}

func newBlogAPI() http.Handler {
	api := &blogAPI{posts: make(map[uint64]post), nextID: 1}
	mux := http.NewServeMux()
	mux.HandleFunc("/posts", api.collection)
	mux.HandleFunc("/posts/{id}", api.item)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "route not found")
	})
	return mux
}

func (api *blogAPI) collection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		api.mu.RLock()
		posts := make([]post, 0, len(api.posts))
		for _, p := range api.posts {
			posts = append(posts, p)
		}
		api.mu.RUnlock()
		sort.Slice(posts, func(i, j int) bool { return posts[i].ID < posts[j].ID })
		writeJSON(w, http.StatusOK, posts)
	case http.MethodPost:
		input, ok := readPost(w, r)
		if !ok {
			return
		}
		now := time.Now().UTC()
		api.mu.Lock()
		p := post{ID: api.nextID, Title: input.Title, Content: input.Content, CreatedAt: now, UpdatedAt: now}
		api.posts[p.ID] = p
		api.nextID++
		api.mu.Unlock()
		w.Header().Set("Location", fmt.Sprintf("/posts/%d", p.ID))
		writeJSON(w, http.StatusCreated, p)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (api *blogAPI) item(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPut && r.Method != http.MethodDelete {
		w.Header().Set("Allow", "GET, PUT, DELETE")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		writeError(w, http.StatusBadRequest, "id must be a positive integer")
		return
	}
	var input postInput
	if r.Method == http.MethodPut {
		var ok bool
		input, ok = readPost(w, r)
		if !ok {
			return
		}
	}
	api.mu.Lock()
	p, exists := api.posts[id]
	if !exists {
		api.mu.Unlock()
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	switch r.Method {
	case http.MethodPut:
		p.Title, p.Content, p.UpdatedAt = input.Title, input.Content, time.Now().UTC()
		api.posts[id] = p
	case http.MethodDelete:
		delete(api.posts, id)
	}
	api.mu.Unlock()
	if r.Method == http.MethodDelete {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func readPost(w http.ResponseWriter, r *http.Request) (postInput, bool) {
	const maxBodySize = 1 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input postInput
	err := decoder.Decode(&input)
	if err == nil {
		var extra any
		if err = decoder.Decode(&extra); err == io.EOF {
			err = nil
		} else if err == nil {
			err = errors.New("multiple JSON values")
		}
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "body must not exceed 1 MiB")
		} else {
			writeError(w, http.StatusBadRequest, "body must be one JSON object containing only title and content strings")
		}
		return postInput{}, false
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Content = strings.TrimSpace(input.Content)
	if input.Title == "" || input.Content == "" {
		writeError(w, http.StatusBadRequest, "title and content are required")
		return postInput{}, false
	}
	return input, true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           newBlogAPI(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("blog API listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
