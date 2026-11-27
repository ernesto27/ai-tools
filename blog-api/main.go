package main

import (
	"database/sql"
	"encoding/json"
	"errors"
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

func main() {
	path := os.Getenv("DATABASE_PATH")
	if path == "" {
		path = "blog.db"
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = initDB(db); err != nil {
		log.Fatal(err)
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("blog API listening on %s", addr)
	server := &http.Server{Addr: addr, Handler: routes(db), ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.ListenAndServe())
}
func initDB(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS posts (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, content TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`)
	return err
}
func routes(db *sql.DB) http.Handler {
	a := &api{db}
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
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}
func decode(w http.ResponseWriter, r *http.Request) (input, bool) {
	var in input
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(&in); err != nil {
		fail(w, 400, "invalid JSON body")
		return in, false
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		fail(w, 400, "body must contain one JSON object")
		return in, false
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Content = strings.TrimSpace(in.Content)
	if in.Title == "" || in.Content == "" {
		fail(w, 400, "title and content are required")
		return in, false
	}
	return in, true
}
func idFrom(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}
func scan(row interface{ Scan(...any) error }) (post, error) {
	var p post
	err := row.Scan(&p.ID, &p.Title, &p.Content, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}
func (a *api) list(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryContext(r.Context(), "SELECT id,title,content,created_at,updated_at FROM posts ORDER BY id DESC")
	if err != nil {
		fail(w, 500, "database error")
		return
	}
	defer rows.Close()
	posts := []post{}
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			fail(w, 500, "database error")
			return
		}
		posts = append(posts, p)
	}
	if rows.Err() != nil {
		fail(w, 500, "database error")
		return
	}
	respond(w, 200, posts)
}
func (a *api) get(w http.ResponseWriter, r *http.Request) {
	id, err := idFrom(r)
	if err != nil {
		fail(w, 400, "invalid id")
		return
	}
	p, err := scan(a.db.QueryRowContext(r.Context(), "SELECT id,title,content,created_at,updated_at FROM posts WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "post not found")
		return
	}
	if err != nil {
		fail(w, 500, "database error")
		return
	}
	respond(w, 200, p)
}
func (a *api) create(w http.ResponseWriter, r *http.Request) {
	in, ok := decode(w, r)
	if !ok {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := a.db.ExecContext(r.Context(), "INSERT INTO posts (title,content,created_at,updated_at) VALUES (?,?,?,?)", in.Title, in.Content, now, now)
	if err != nil {
		fail(w, 500, "database error")
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		fail(w, 500, "database error")
		return
	}
	w.Header().Set("Location", "/posts/"+strconv.FormatInt(id, 10))
	respond(w, 201, post{id, in.Title, in.Content, now, now})
}
func (a *api) update(w http.ResponseWriter, r *http.Request) {
	id, err := idFrom(r)
	if err != nil {
		fail(w, 400, "invalid id")
		return
	}
	in, ok := decode(w, r)
	if !ok {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := a.db.ExecContext(r.Context(), "UPDATE posts SET title=?, content=?, updated_at=? WHERE id=?", in.Title, in.Content, now, id)
	if err != nil {
		fail(w, 500, "database error")
		return
	}
	n, err := result.RowsAffected()
	if err != nil {
		fail(w, 500, "database error")
		return
	}
	if n == 0 {
		fail(w, 404, "post not found")
		return
	}
	a.get(w, r)
}
func (a *api) delete(w http.ResponseWriter, r *http.Request) {
	id, err := idFrom(r)
	if err != nil {
		fail(w, 400, "invalid id")
		return
	}
	result, err := a.db.ExecContext(r.Context(), "DELETE FROM posts WHERE id=?", id)
	if err != nil {
		fail(w, 500, "database error")
		return
	}
	n, err := result.RowsAffected()
	if err != nil {
		fail(w, 500, "database error")
		return
	}
	if n == 0 {
		fail(w, 404, "post not found")
		return
	}
	w.WriteHeader(204)
}
