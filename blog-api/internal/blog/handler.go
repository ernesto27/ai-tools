package blog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const maxRequestBytes = 1 << 20

type Handler struct {
	store Store
}

func NewHandler(store Store) http.Handler {
	h := &Handler{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /api/posts", h.listPosts)
	mux.HandleFunc("POST /api/posts", h.createPost)
	mux.HandleFunc("GET /api/posts/{id}", h.getPost)
	mux.HandleFunc("PUT /api/posts/{id}", h.updatePost)
	mux.HandleFunc("DELETE /api/posts/{id}", h.deletePost)
	return mux
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) listPosts(w http.ResponseWriter, r *http.Request) {
	limit, err := queryInt(r, "limit", 20)
	if err != nil || limit < 1 || limit > 100 {
		writeError(w, http.StatusBadRequest, "limit must be between 1 and 100")
		return
	}
	offset, err := queryInt(r, "offset", 0)
	if err != nil || offset < 0 {
		writeError(w, http.StatusBadRequest, "offset must be a non-negative integer")
		return
	}
	posts, total := h.store.List(r.URL.Query().Get("q"), offset, limit)
	writeJSON(w, http.StatusOK, map[string]any{
		"posts": posts, "total": total, "limit": limit, "offset": offset,
	})
}

func (h *Handler) createPost(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeInput(w, r)
	if !ok {
		return
	}
	if message := validate(input); message != "" {
		writeError(w, http.StatusBadRequest, message)
		return
	}
	post, err := h.store.Create(input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save post")
		return
	}
	w.Header().Set("Location", "/api/posts/"+post.ID)
	writeJSON(w, http.StatusCreated, post)
}

func (h *Handler) getPost(w http.ResponseWriter, r *http.Request) {
	post, err := h.store.Get(r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load post")
		return
	}
	writeJSON(w, http.StatusOK, post)
}

func (h *Handler) updatePost(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeInput(w, r)
	if !ok {
		return
	}
	if message := validate(input); message != "" {
		writeError(w, http.StatusBadRequest, message)
		return
	}
	post, err := h.store.Update(r.PathValue("id"), input)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save post")
		return
	}
	writeJSON(w, http.StatusOK, post)
}

func (h *Handler) deletePost(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Delete(r.PathValue("id")); errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "post not found")
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete post")
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func decodeInput(w http.ResponseWriter, r *http.Request) (PostInput, bool) {
	if mediaType := strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]); mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "content type must be application/json")
		return PostInput{}, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input PostInput
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return PostInput{}, false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "request body must contain one JSON object")
		return PostInput{}, false
	}
	return input, true
}

func validate(input PostInput) string {
	if strings.TrimSpace(input.Title) == "" {
		return "title is required"
	}
	if len(input.Title) > 200 {
		return "title must be at most 200 characters"
	}
	if strings.TrimSpace(input.Content) == "" {
		return "content is required"
	}
	if len(input.Content) > 500_000 {
		return "content must be at most 500000 characters"
	}
	if len(input.Excerpt) > 1000 {
		return "excerpt must be at most 1000 characters"
	}
	if len(input.Author) > 120 {
		return "author must be at most 120 characters"
	}
	if len(input.Tags) > 20 {
		return "at most 20 tags are allowed"
	}
	for _, tag := range input.Tags {
		if len(tag) > 50 {
			return "tags must be at most 50 characters"
		}
	}
	return ""
}

func queryInt(r *http.Request, key string, fallback int) (int, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return n, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
