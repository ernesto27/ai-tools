package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type post struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type postInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type blog struct {
	mu     sync.RWMutex
	posts  map[int]post
	nextID int
}

func main() {
	b := &blog{posts: make(map[int]post), nextID: 1}
	mux := http.NewServeMux()
	mux.HandleFunc("/posts", b.handlePosts)
	mux.HandleFunc("/posts/", b.handlePost)

	log.Println("blog API listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (b *blog) handlePosts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		b.mu.RLock()
		posts := make([]post, 0, len(b.posts))
		for _, p := range b.posts {
			posts = append(posts, p)
		}
		b.mu.RUnlock()
		writeJSON(w, http.StatusOK, posts)
	case http.MethodPost:
		input, err := decodePost(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		b.mu.Lock()
		p := post{ID: b.nextID, Title: input.Title, Content: input.Content, CreatedAt: time.Now().UTC()}
		b.nextID++
		b.posts[p.ID] = p
		b.mu.Unlock()
		writeJSON(w, http.StatusCreated, p)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (b *blog) handlePost(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(strings.TrimPrefix(r.URL.Path, "/posts/"), "/") {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/posts/"))
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "post ID must be a positive integer")
		return
	}

	switch r.Method {
	case http.MethodGet:
		b.mu.RLock()
		p, ok := b.posts[id]
		b.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, "post not found")
			return
		}
		writeJSON(w, http.StatusOK, p)
	case http.MethodPut:
		input, err := decodePost(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		b.mu.Lock()
		p, ok := b.posts[id]
		if ok {
			p.Title, p.Content = input.Title, input.Content
			b.posts[id] = p
		}
		b.mu.Unlock()
		if !ok {
			writeError(w, http.StatusNotFound, "post not found")
			return
		}
		writeJSON(w, http.StatusOK, p)
	case http.MethodDelete:
		b.mu.Lock()
		_, ok := b.posts[id]
		delete(b.posts, id)
		b.mu.Unlock()
		if !ok {
			writeError(w, http.StatusNotFound, "post not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, PUT, DELETE")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func decodePost(r *http.Request) (postInput, error) {
	var input postInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return input, errors.New("request body must contain valid JSON with title and content")
	}
	if strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.Content) == "" {
		return input, errors.New("title and content are required")
	}
	return input, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
