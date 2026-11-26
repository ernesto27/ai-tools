package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

const maxPostBodyBytes = 1 << 20

type postAPI struct {
	store postStore
}

func newHandler(store postStore) http.Handler {
	api := postAPI{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /posts", api.list)
	mux.HandleFunc("POST /posts", api.create)
	mux.HandleFunc("GET /posts/{id}", api.get)
	mux.HandleFunc("PUT /posts/{id}", api.update)
	mux.HandleFunc("DELETE /posts/{id}", api.delete)
	return mux
}

func (api postAPI) list(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit, ok := pageValue(query.Get("limit"), 20)
	if !ok || limit < 1 || limit > 100 {
		writeError(w, http.StatusBadRequest, "limit must be an integer from 1 to 100")
		return
	}
	offset, ok := pageValue(query.Get("offset"), 0)
	if !ok || offset < 0 {
		writeError(w, http.StatusBadRequest, "offset must be a non-negative integer")
		return
	}
	posts, err := api.store.list(r.Context(), limit, offset)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, posts)
}

func pageValue(raw string, fallback int) (int, bool) {
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	return value, err == nil
}

func (api postAPI) create(w http.ResponseWriter, r *http.Request) {
	input, ok := readPost(w, r)
	if !ok {
		return
	}
	p, err := api.store.create(r.Context(), input)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	w.Header().Set("Location", "/posts/"+strconv.FormatInt(p.ID, 10))
	writeJSON(w, http.StatusCreated, p)
}

func (api postAPI) get(w http.ResponseWriter, r *http.Request) {
	id, ok := readID(w, r)
	if !ok {
		return
	}
	p, err := api.store.get(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (api postAPI) update(w http.ResponseWriter, r *http.Request) {
	id, ok := readID(w, r)
	if !ok {
		return
	}
	input, ok := readPost(w, r)
	if !ok {
		return
	}
	p, err := api.store.update(r.Context(), id, input)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (api postAPI) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := readID(w, r)
	if !ok {
		return
	}
	if err := api.store.delete(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func readID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id must be a positive integer")
		return 0, false
	}
	return id, true
}

func readPost(w http.ResponseWriter, r *http.Request) (postInput, bool) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return postInput{}, false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPostBodyBytes))
	decoder.DisallowUnknownFields()

	var input postInput
	err = decoder.Decode(&input)
	if err == nil {
		// The stream must end after the post object and any trailing whitespace.
		switch err = decoder.Decode(&struct{}{}); err {
		case io.EOF:
			err = nil
		case nil:
			err = errors.New("unexpected data after post object")
		}
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds 1 MiB")
		} else {
			writeError(w, http.StatusBadRequest, "request body must be one JSON object with title and content")
		}
		return postInput{}, false
	}

	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" || strings.TrimSpace(input.Content) == "" {
		writeError(w, http.StatusBadRequest, "title and content are required")
		return postInput{}, false
	}
	return input, true
}

func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	log.Printf("post database error: %v", err)
	writeError(w, http.StatusInternalServerError, "database operation failed")
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, struct {
		Error string `json:"error"`
	}{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}
