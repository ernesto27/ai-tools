package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Post struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type postInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type blog struct {
	mu     sync.RWMutex
	posts  map[int64]Post
	nextID int64
}

func newBlog() *blog {
	return &blog{posts: make(map[int64]Post), nextID: 1}
}

func (b *blog) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /posts", b.listPosts)
	mux.HandleFunc("POST /posts", b.createPost)
	mux.HandleFunc("GET /posts/{id}", b.getPost)
	mux.HandleFunc("PUT /posts/{id}", b.updatePost)
	mux.HandleFunc("DELETE /posts/{id}", b.deletePost)
	return mux
}

func (b *blog) listPosts(w http.ResponseWriter, _ *http.Request) {
	b.mu.RLock()
	posts := make([]Post, 0, len(b.posts))
	for _, post := range b.posts {
		posts = append(posts, post)
	}
	b.mu.RUnlock()

	writeJSON(w, http.StatusOK, posts)
}

func (b *blog) createPost(w http.ResponseWriter, r *http.Request) {
	var input postInput
	if err := decodeJSON(r, &input); err != nil || !validInput(input) {
		http.Error(w, "title and content are required", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	b.mu.Lock()
	post := Post{
		ID: b.nextID, Title: strings.TrimSpace(input.Title), Content: strings.TrimSpace(input.Content),
		CreatedAt: now, UpdatedAt: now,
	}
	b.posts[post.ID] = post
	b.nextID++
	b.mu.Unlock()

	w.Header().Set("Location", "/posts/"+strconv.FormatInt(post.ID, 10))
	writeJSON(w, http.StatusCreated, post)
}

func (b *blog) getPost(w http.ResponseWriter, r *http.Request) {
	id, ok := postID(r)
	if !ok {
		http.Error(w, "invalid post id", http.StatusBadRequest)
		return
	}

	b.mu.RLock()
	post, found := b.posts[id]
	b.mu.RUnlock()
	if !found {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, post)
}

func (b *blog) updatePost(w http.ResponseWriter, r *http.Request) {
	id, ok := postID(r)
	if !ok {
		http.Error(w, "invalid post id", http.StatusBadRequest)
		return
	}

	var input postInput
	if err := decodeJSON(r, &input); err != nil || !validInput(input) {
		http.Error(w, "title and content are required", http.StatusBadRequest)
		return
	}

	b.mu.Lock()
	post, found := b.posts[id]
	if found {
		post.Title = strings.TrimSpace(input.Title)
		post.Content = strings.TrimSpace(input.Content)
		post.UpdatedAt = time.Now().UTC()
		b.posts[id] = post
	}
	b.mu.Unlock()
	if !found {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, post)
}

func (b *blog) deletePost(w http.ResponseWriter, r *http.Request) {
	id, ok := postID(r)
	if !ok {
		http.Error(w, "invalid post id", http.StatusBadRequest)
		return
	}

	b.mu.Lock()
	_, found := b.posts[id]
	delete(b.posts, id)
	b.mu.Unlock()
	if !found {
		http.NotFound(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON value")
	}
	return nil
}

func validInput(input postInput) bool {
	return strings.TrimSpace(input.Title) != "" && strings.TrimSpace(input.Content) != ""
}

func postID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

func main() {
	server := &http.Server{
		Addr:              ":8080",
		Handler:           newBlog().routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("blog API listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
