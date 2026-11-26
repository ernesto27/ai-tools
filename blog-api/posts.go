package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const postColumns = "id, title, content, created_at, updated_at"

type postInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type post struct {
	ID        int64  `json:"id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	postInput
}

type blogAPI struct {
	db *sql.DB
}

func newHandler(db *sql.DB) http.Handler {
	api := blogAPI{db: db}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /posts", api.listPosts)
	mux.HandleFunc("POST /posts", api.createPost)
	mux.HandleFunc("GET /posts/{id}", api.getPost)
	mux.HandleFunc("PUT /posts/{id}", api.updatePost)
	mux.HandleFunc("DELETE /posts/{id}", api.deletePost)
	return mux
}

func (api blogAPI) listPosts(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid query parameters")
		return
	}
	limit, offset := 20, 0
	if query.Has("limit") {
		value, err := strconv.Atoi(query.Get("limit"))
		if err != nil || value < 1 || value > 100 {
			writeError(w, http.StatusBadRequest, "limit must be an integer from 1 to 100")
			return
		}
		limit = value
	}
	if query.Has("offset") {
		value, err := strconv.Atoi(query.Get("offset"))
		if err != nil || value < 0 {
			writeError(w, http.StatusBadRequest, "offset must be a non-negative integer")
			return
		}
		offset = value
	}

	rows, err := api.db.QueryContext(r.Context(), `SELECT `+postColumns+`
		FROM posts ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		writeDatabaseError(w, err)
		return
	}
	defer rows.Close()

	posts := make([]post, 0)
	for rows.Next() {
		value, err := scanPost(rows)
		if err != nil {
			writeDatabaseError(w, err)
			return
		}
		posts = append(posts, value)
	}
	if err := rows.Err(); err != nil {
		writeDatabaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, posts)
}

func (api blogAPI) createPost(w http.ResponseWriter, r *http.Request) {
	input, ok := readPost(w, r)
	if !ok {
		return
	}
	row := api.db.QueryRowContext(r.Context(), `INSERT INTO posts (title, content)
		VALUES (?, ?) RETURNING `+postColumns, input.Title, input.Content)
	writePost(w, http.StatusCreated, row)
}

func (api blogAPI) getPost(w http.ResponseWriter, r *http.Request) {
	id, ok := readID(w, r)
	if !ok {
		return
	}
	row := api.db.QueryRowContext(r.Context(), "SELECT "+postColumns+" FROM posts WHERE id = ?", id)
	writePost(w, http.StatusOK, row)
}

func (api blogAPI) updatePost(w http.ResponseWriter, r *http.Request) {
	id, ok := readID(w, r)
	if !ok {
		return
	}
	input, ok := readPost(w, r)
	if !ok {
		return
	}
	row := api.db.QueryRowContext(r.Context(), `UPDATE posts
		SET title = ?, content = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ? RETURNING `+postColumns, input.Title, input.Content, id)
	writePost(w, http.StatusOK, row)
}

func (api blogAPI) deletePost(w http.ResponseWriter, r *http.Request) {
	id, ok := readID(w, r)
	if !ok {
		return
	}
	var deletedID int64
	err := api.db.QueryRowContext(r.Context(), "DELETE FROM posts WHERE id = ? RETURNING id", id).Scan(&deletedID)
	if err != nil {
		writeDatabaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func readID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "post id must be a positive integer")
		return 0, false
	}
	return id, true
}

func readPost(w http.ResponseWriter, r *http.Request) (postInput, bool) {
	var input postInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&input)
	if err == nil {
		var extra json.RawMessage
		err = decoder.Decode(&extra)
		if err == nil {
			writeError(w, http.StatusBadRequest, "expected a single JSON object")
			return postInput{}, false
		}
		if errors.Is(err, io.EOF) {
			err = nil
		}
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds 1 MiB")
		return postInput{}, false
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "expected a JSON object with title and content fields")
		return postInput{}, false
	}
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" || strings.TrimSpace(input.Content) == "" {
		writeError(w, http.StatusBadRequest, "title and content must both be non-empty strings")
		return postInput{}, false
	}
	return input, true
}

func scanPost(row interface{ Scan(...any) error }) (post, error) {
	var value post
	err := row.Scan(&value.ID, &value.Title, &value.Content, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

func writePost(w http.ResponseWriter, status int, row *sql.Row) {
	value, err := scanPost(row)
	if err != nil {
		writeDatabaseError(w, err)
		return
	}
	if status == http.StatusCreated {
		w.Header().Set("Location", "/posts/"+strconv.FormatInt(value.ID, 10))
	}
	writeJSON(w, status, value)
}

func writeDatabaseError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	log.Printf("SQLite operation failed: %v", err)
	writeError(w, http.StatusInternalServerError, "database operation failed")
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("Write JSON response: %v", err)
	}
}
