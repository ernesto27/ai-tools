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

type Post struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type postInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type postStore struct {
	mu     sync.RWMutex
	posts  map[int]Post
	nextID int
}

func newPostStore() *postStore {
	return &postStore{posts: make(map[int]Post), nextID: 1}
}

func (s *postStore) list() []Post {
	s.mu.RLock()
	defer s.mu.RUnlock()

	posts := make([]Post, 0, len(s.posts))
	for _, post := range s.posts {
		posts = append(posts, post)
	}
	return posts
}

func (s *postStore) get(id int) (Post, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	post, ok := s.posts[id]
	return post, ok
}

func (s *postStore) create(input postInput) Post {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	post := Post{ID: s.nextID, Title: input.Title, Content: input.Content, CreatedAt: now, UpdatedAt: now}
	s.posts[post.ID] = post
	s.nextID++
	return post
}

func (s *postStore) update(id int, input postInput) (Post, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	post, ok := s.posts[id]
	if !ok {
		return Post{}, false
	}
	post.Title = input.Title
	post.Content = input.Content
	post.UpdatedAt = time.Now().UTC()
	s.posts[id] = post
	return post, true
}

func (s *postStore) delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.posts[id]; !ok {
		return false
	}
	delete(s.posts, id)
	return true
}

func main() {
	store := newPostStore()
	mux := http.NewServeMux()
	mux.HandleFunc("/posts", postsHandler(store))
	mux.HandleFunc("/posts/", postHandler(store))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	addr := ":8080"
	log.Printf("blog API listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func postsHandler(store *postStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, store.list())
		case http.MethodPost:
			input, err := decodePostInput(r)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			post := store.create(input)
			writeJSON(w, http.StatusCreated, post)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
	}
}

func postHandler(store *postStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idText := strings.TrimPrefix(r.URL.Path, "/posts/")
		id, err := strconv.Atoi(idText)
		if err != nil || id < 1 {
			writeError(w, http.StatusBadRequest, "post ID must be a positive integer")
			return
		}

		switch r.Method {
		case http.MethodGet:
			post, ok := store.get(id)
			if !ok {
				writeError(w, http.StatusNotFound, "post not found")
				return
			}
			writeJSON(w, http.StatusOK, post)
		case http.MethodPut:
			input, err := decodePostInput(r)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			post, ok := store.update(id, input)
			if !ok {
				writeError(w, http.StatusNotFound, "post not found")
				return
			}
			writeJSON(w, http.StatusOK, post)
		case http.MethodDelete:
			if !store.delete(id) {
				writeError(w, http.StatusNotFound, "post not found")
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete)
		}
	}
}

func decodePostInput(r *http.Request) (postInput, error) {
	var input postInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return postInput{}, errors.New("request body must be valid JSON with title and content")
	}
	if strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.Content) == "" {
		return postInput{}, errors.New("title and content are required")
	}
	return input, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}
