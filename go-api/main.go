package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
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

func main() {
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "blog.db"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	if err := initializeDatabase(db); err != nil {
		log.Fatalf("initialize database: %v", err)
	}

	log.Printf("listening on :8080")
	if err := http.ListenAndServe(":8080", newRouter(db)); err != nil {
		log.Fatalf("start server: %v", err)
	}
}

func initializeDatabase(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`)
	return err
}

func newRouter(db *sql.DB) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /posts", listPosts(db))
	mux.HandleFunc("POST /posts", createPost(db))
	mux.HandleFunc("GET /posts/{id}", getPost(db))
	mux.HandleFunc("PUT /posts/{id}", updatePost(db))
	mux.HandleFunc("DELETE /posts/{id}", deletePost(db))
	return mux
}

func listPosts(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.QueryContext(r.Context(), `
			SELECT id, title, content, created_at, updated_at
			FROM posts ORDER BY created_at DESC, id DESC
		`)
		if err != nil {
			writeDatabaseError(w, err)
			return
		}
		defer rows.Close()

		posts := make([]Post, 0)
		for rows.Next() {
			post, err := scanPost(rows)
			if err != nil {
				writeDatabaseError(w, err)
				return
			}
			posts = append(posts, post)
		}
		if err := rows.Err(); err != nil {
			writeDatabaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, posts)
	}
}

func createPost(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input postInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || !validPost(input) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title and content are required"})
			return
		}

		now := time.Now().UTC()
		result, err := db.ExecContext(r.Context(), `
			INSERT INTO posts (title, content, created_at, updated_at)
			VALUES (?, ?, ?, ?)
		`, strings.TrimSpace(input.Title), strings.TrimSpace(input.Content), now, now)
		if err != nil {
			writeDatabaseError(w, err)
			return
		}
		id, err := result.LastInsertId()
		if err != nil {
			writeDatabaseError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, Post{
			ID: id, Title: strings.TrimSpace(input.Title), Content: strings.TrimSpace(input.Content),
			CreatedAt: now, UpdatedAt: now,
		})
	}
}

func getPost(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := postID(w, r)
		if !ok {
			return
		}
		post, err := findPost(r, db, id)
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
			return
		}
		if err != nil {
			writeDatabaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, post)
	}
}

func updatePost(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := postID(w, r)
		if !ok {
			return
		}
		var input postInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || !validPost(input) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title and content are required"})
			return
		}

		result, err := db.ExecContext(r.Context(), `
			UPDATE posts SET title = ?, content = ?, updated_at = ? WHERE id = ?
		`, strings.TrimSpace(input.Title), strings.TrimSpace(input.Content), time.Now().UTC(), id)
		if err != nil {
			writeDatabaseError(w, err)
			return
		}
		count, err := result.RowsAffected()
		if err != nil {
			writeDatabaseError(w, err)
			return
		}
		if count == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
			return
		}
		post, err := findPost(r, db, id)
		if err != nil {
			writeDatabaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, post)
	}
}

func deletePost(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := postID(w, r)
		if !ok {
			return
		}
		result, err := db.ExecContext(r.Context(), `DELETE FROM posts WHERE id = ?`, id)
		if err != nil {
			writeDatabaseError(w, err)
			return
		}
		count, err := result.RowsAffected()
		if err != nil {
			writeDatabaseError(w, err)
			return
		}
		if count == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPost(row rowScanner) (Post, error) {
	var post Post
	err := row.Scan(&post.ID, &post.Title, &post.Content, &post.CreatedAt, &post.UpdatedAt)
	return post, err
}

func findPost(r *http.Request, db *sql.DB, id int64) (Post, error) {
	return scanPost(db.QueryRowContext(r.Context(), `
		SELECT id, title, content, created_at, updated_at FROM posts WHERE id = ?
	`, id))
}

func postID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid post id"})
		return 0, false
	}
	return id, true
}

func validPost(input postInput) bool {
	return strings.TrimSpace(input.Title) != "" && strings.TrimSpace(input.Content) != ""
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

func writeDatabaseError(w http.ResponseWriter, err error) {
	log.Printf("database request failed: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
