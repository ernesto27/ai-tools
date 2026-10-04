package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Post struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type postStore struct {
	mu     sync.RWMutex
	posts  map[int]Post
	nextID int
}

func newPostStore() *postStore {
	return &postStore{posts: make(map[int]Post), nextID: 1}
}

func (s *postStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/posts" || r.URL.Path == "/posts/" {
		s.collection(w, r)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/posts/") {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/posts/"))
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "post id must be a positive integer")
		return
	}
	s.item(w, r, id)
}

func (s *postStore) collection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		posts := make([]Post, 0, len(s.posts))
		for _, post := range s.posts {
			posts = append(posts, post)
		}
		s.mu.RUnlock()
		writeJSON(w, http.StatusOK, posts)
	case http.MethodPost:
		var input postInput
		if err := decodeJSON(w, r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := input.validate(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		now := time.Now().UTC()
		s.mu.Lock()
		post := Post{ID: s.nextID, Title: strings.TrimSpace(input.Title), Content: strings.TrimSpace(input.Content), CreatedAt: now, UpdatedAt: now}
		s.posts[post.ID] = post
		s.nextID++
		s.mu.Unlock()
		writeJSON(w, http.StatusCreated, post)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *postStore) item(w http.ResponseWriter, r *http.Request, id int) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		post, ok := s.posts[id]
		s.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, "post not found")
			return
		}
		writeJSON(w, http.StatusOK, post)
	case http.MethodPut:
		var input postInput
		if err := decodeJSON(w, r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := input.validate(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.mu.Lock()
		post, ok := s.posts[id]
		if ok {
			post.Title = strings.TrimSpace(input.Title)
			post.Content = strings.TrimSpace(input.Content)
			post.UpdatedAt = time.Now().UTC()
			s.posts[id] = post
		}
		s.mu.Unlock()
		if !ok {
			writeError(w, http.StatusNotFound, "post not found")
			return
		}
		writeJSON(w, http.StatusOK, post)
	case http.MethodDelete:
		s.mu.Lock()
		_, ok := s.posts[id]
		delete(s.posts, id)
		s.mu.Unlock()
		if !ok {
			writeError(w, http.StatusNotFound, "post not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete)
	}
}

type postInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (in postInput) validate() error {
	if strings.TrimSpace(in.Title) == "" {
		return errors.New("title is required")
	}
	if strings.TrimSpace(in.Content) == "" {
		return errors.New("content is required")
	}
	return nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return errors.New("request body must be valid JSON with title and content")
	}
	if err := decoder.Decode(new(any)); err != nil {
		if !errors.Is(err, io.EOF) {
			return errors.New("request body must contain a single JSON object")
		}
	} else {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	posts := newPostStore()
	mux.Handle("/posts", posts)
	mux.Handle("/posts/", posts)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	log.Printf("blog API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
