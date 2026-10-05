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

type post struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type postInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type api struct {
	db *sql.DB
}

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "blog.db"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	if err := initializeDatabase(db); err != nil {
		log.Fatalf("initialize database: %v", err)
	}

	handler := &api{db: db}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.health)
	mux.HandleFunc("/posts", handler.posts)
	mux.HandleFunc("/posts/", handler.postByID)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("blog API listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func initializeDatabase(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func (a *api) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *api) posts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id, title, content, created_at FROM posts ORDER BY id DESC`)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list posts")
			return
		}
		defer rows.Close()

		posts := make([]post, 0)
		for rows.Next() {
			var p post
			if err := rows.Scan(&p.ID, &p.Title, &p.Content, &p.CreatedAt); err != nil {
				writeError(w, http.StatusInternalServerError, "could not read posts")
				return
			}
			posts = append(posts, p)
		}
		if err := rows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, "could not read posts")
			return
		}
		writeJSON(w, http.StatusOK, posts)
	case http.MethodPost:
		var input postInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "request body must be valid JSON")
			return
		}
		input.Title = strings.TrimSpace(input.Title)
		if input.Title == "" {
			writeError(w, http.StatusBadRequest, "title is required")
			return
		}

		result, err := a.db.Exec(`INSERT INTO posts (title, content) VALUES (?, ?)`, input.Title, input.Content)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create post")
			return
		}
		id, err := result.LastInsertId()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create post")
			return
		}
		var created post
		err = a.db.QueryRow(`SELECT id, title, content, created_at FROM posts WHERE id = ?`, id).
			Scan(&created.ID, &created.Title, &created.Content, &created.CreatedAt)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not read created post")
			return
		}
		writeJSON(w, http.StatusCreated, created)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *api) postByID(w http.ResponseWriter, r *http.Request) {
	idText := strings.TrimPrefix(r.URL.Path, "/posts/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "post id must be a positive integer")
		return
	}

	switch r.Method {
	case http.MethodGet:
		var p post
		err := a.db.QueryRow(`SELECT id, title, content, created_at FROM posts WHERE id = ?`, id).
			Scan(&p.ID, &p.Title, &p.Content, &p.CreatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "post not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not read post")
			return
		}
		writeJSON(w, http.StatusOK, p)
	case http.MethodDelete:
		result, err := a.db.Exec(`DELETE FROM posts WHERE id = ?`, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not delete post")
			return
		}
		deleted, err := result.RowsAffected()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not delete post")
			return
		}
		if deleted == 0 {
			writeError(w, http.StatusNotFound, "post not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, DELETE")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
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
