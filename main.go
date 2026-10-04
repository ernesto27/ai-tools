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
	post := Post{
		ID:        s.nextID,
		Title:     strings.TrimSpace(input.Title),
		Content:   strings.TrimSpace(input.Content),
		CreatedAt: now,
		UpdatedAt: now,
	}
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
	post.Title = strings.TrimSpace(input.Title)
	post.Content = strings.TrimSpace(input.Content)
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

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /posts", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, store.list())
	})
	mux.HandleFunc("POST /posts", func(w http.ResponseWriter, r *http.Request) {
		input, err := decodePostInput(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := validatePostInput(input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, store.create(input))
	})
	mux.HandleFunc("GET /posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := postID(w, r)
		if !ok {
			return
		}
		post, found := store.get(id)
		if !found {
			writeError(w, http.StatusNotFound, "post not found")
			return
		}
		writeJSON(w, http.StatusOK, post)
	})
	mux.HandleFunc("PUT /posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := postID(w, r)
		if !ok {
			return
		}
		input, err := decodePostInput(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := validatePostInput(input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		post, found := store.update(id, input)
		if !found {
			writeError(w, http.StatusNotFound, "post not found")
			return
		}
		writeJSON(w, http.StatusOK, post)
	})
	mux.HandleFunc("DELETE /posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := postID(w, r)
		if !ok {
			return
		}
		if !store.delete(id) {
			writeError(w, http.StatusNotFound, "post not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	addr := ":8080"
	log.Printf("blog API listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func decodePostInput(r *http.Request) (postInput, error) {
	defer r.Body.Close()
	var input postInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return postInput{}, errors.New("request body must be valid JSON")
	}
	return input, nil
}

func validatePostInput(input postInput) error {
	if strings.TrimSpace(input.Title) == "" {
		return errors.New("title is required")
	}
	if strings.TrimSpace(input.Content) == "" {
		return errors.New("content is required")
	}
	return nil
}

func postID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "post id must be a positive integer")
		return 0, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
