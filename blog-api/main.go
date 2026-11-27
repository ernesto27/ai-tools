package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type post struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
type input struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}
type api struct{ db *sql.DB }

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS posts (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, content TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
func (a api) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /posts", a.list)
	mux.HandleFunc("POST /posts", a.create)
	mux.HandleFunc("GET /posts/{id}", a.get)
	mux.HandleFunc("PUT /posts/{id}", a.update)
	mux.HandleFunc("DELETE /posts/{id}", a.delete)
	return mux
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": message})
}
func readInput(w http.ResponseWriter, r *http.Request) (input, bool) {
	var v input
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(&v); err != nil {
		fail(w, 400, "invalid JSON body")
		return v, false
	}
	var extra any
	if err := d.Decode(&extra); err == nil || !errors.Is(err, io.EOF) {
		fail(w, 400, "invalid JSON body")
		return v, false
	}
	v.Title = strings.TrimSpace(v.Title)
	v.Content = strings.TrimSpace(v.Content)
	if v.Title == "" || v.Content == "" {
		fail(w, 400, "title and content are required")
		return v, false
	}
	return v, true
}
func idFrom(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid id")
	}
	return id, nil
}
func scanPost(s interface{ Scan(...any) error }) (post, error) {
	var p post
	err := s.Scan(&p.ID, &p.Title, &p.Content, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}
func (a api) find(id int64) (post, error) {
	return scanPost(a.db.QueryRow("SELECT id, title, content, created_at, updated_at FROM posts WHERE id = ?", id))
}
func dbError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "post not found")
	} else {
		log.Printf("database error: %v", err)
		fail(w, 500, "internal server error")
	}
}
func (a api) list(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryContext(r.Context(), "SELECT id, title, content, created_at, updated_at FROM posts ORDER BY id DESC")
	if err != nil {
		dbError(w, err)
		return
	}
	defer rows.Close()
	posts := []post{}
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			dbError(w, err)
			return
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		dbError(w, err)
		return
	}
	respond(w, 200, posts)
}
func (a api) create(w http.ResponseWriter, r *http.Request) {
	v, ok := readInput(w, r)
	if !ok {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := a.db.ExecContext(r.Context(), "INSERT INTO posts (title, content, created_at, updated_at) VALUES (?, ?, ?, ?)", v.Title, v.Content, now, now)
	if err != nil {
		dbError(w, err)
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		dbError(w, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/posts/%d", id))
	respond(w, 201, post{id, v.Title, v.Content, now, now})
}
func (a api) get(w http.ResponseWriter, r *http.Request) {
	id, err := idFrom(r)
	if err != nil {
		fail(w, 400, "invalid post id")
		return
	}
	p, err := a.find(id)
	if err != nil {
		dbError(w, err)
		return
	}
	respond(w, 200, p)
}
func (a api) update(w http.ResponseWriter, r *http.Request) {
	id, err := idFrom(r)
	if err != nil {
		fail(w, 400, "invalid post id")
		return
	}
	v, ok := readInput(w, r)
	if !ok {
		return
	}
	result, err := a.db.ExecContext(r.Context(), "UPDATE posts SET title = ?, content = ?, updated_at = ? WHERE id = ?", v.Title, v.Content, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		dbError(w, err)
		return
	}
	n, err := result.RowsAffected()
	if err != nil {
		dbError(w, err)
		return
	}
	if n == 0 {
		fail(w, 404, "post not found")
		return
	}
	p, err := a.find(id)
	if err != nil {
		dbError(w, err)
		return
	}
	respond(w, 200, p)
}
func (a api) delete(w http.ResponseWriter, r *http.Request) {
	id, err := idFrom(r)
	if err != nil {
		fail(w, 400, "invalid post id")
		return
	}
	result, err := a.db.ExecContext(r.Context(), "DELETE FROM posts WHERE id = ?", id)
	if err != nil {
		dbError(w, err)
		return
	}
	n, err := result.RowsAffected()
	if err != nil {
		dbError(w, err)
		return
	}
	if n == 0 {
		fail(w, 404, "post not found")
		return
	}
	w.WriteHeader(204)
}
func main() {
	path := os.Getenv("DATABASE_PATH")
	if path == "" {
		path = "blog.db"
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	db, err := openDB(path)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, api{db}.routes()))
}
