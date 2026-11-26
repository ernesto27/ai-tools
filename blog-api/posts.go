package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const postColumns = "id, title, content, created_at, updated_at"

type post struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type postInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type postStore struct {
	db *sql.DB
}

func openDatabase(ctx context.Context, path string) (*sql.DB, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	uriPath := filepath.ToSlash(absolutePath)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	dsn := url.URL{
		Scheme:   "file",
		Path:     uriPath,
		RawQuery: url.Values{"_pragma": {"busy_timeout(5000)"}}.Encode(),
	}
	db, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// A single connection serializes SQLite writes within this process.
	db.SetMaxOpenConns(1)
	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	return db, nil
}

func scanPost(row interface{ Scan(...any) error }) (post, error) {
	var p post
	err := row.Scan(&p.ID, &p.Title, &p.Content, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (s postStore) list(ctx context.Context, limit, offset int) ([]post, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+postColumns+`
		FROM posts ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := make([]post, 0)
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func (s postStore) get(ctx context.Context, id int64) (post, error) {
	return scanPost(s.db.QueryRowContext(ctx, `SELECT `+postColumns+` FROM posts WHERE id = ?`, id))
}

func (s postStore) create(ctx context.Context, input postInput) (post, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return scanPost(s.db.QueryRowContext(ctx, `INSERT INTO posts (title, content, created_at, updated_at)
		VALUES (?, ?, ?, ?) RETURNING `+postColumns, input.Title, input.Content, now, now))
}

func (s postStore) update(ctx context.Context, id int64, input postInput) (post, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return scanPost(s.db.QueryRowContext(ctx, `UPDATE posts SET title = ?, content = ?, updated_at = ?
		WHERE id = ? RETURNING `+postColumns, input.Title, input.Content, now, id))
}

func (s postStore) delete(ctx context.Context, id int64) error {
	var deletedID int64
	return s.db.QueryRowContext(ctx, `DELETE FROM posts WHERE id = ? RETURNING id`, id).Scan(&deletedID)
}
