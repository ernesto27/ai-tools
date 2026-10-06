package main

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

	router := newRouter(db)
	if err := router.Run(":8080"); err != nil {
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

func newRouter(db *sql.DB) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/posts", listPosts(db))
	router.POST("/posts", createPost(db))
	router.GET("/posts/:id", getPost(db))
	router.PUT("/posts/:id", updatePost(db))
	router.DELETE("/posts/:id", deletePost(db))
	return router
}

func listPosts(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.QueryContext(c.Request.Context(), `
			SELECT id, title, content, created_at, updated_at
			FROM posts ORDER BY created_at DESC, id DESC
		`)
		if err != nil {
			writeDatabaseError(c, err)
			return
		}
		defer rows.Close()

		posts := make([]Post, 0)
		for rows.Next() {
			post, err := scanPost(rows)
			if err != nil {
				writeDatabaseError(c, err)
				return
			}
			posts = append(posts, post)
		}
		if err := rows.Err(); err != nil {
			writeDatabaseError(c, err)
			return
		}
		c.JSON(http.StatusOK, posts)
	}
}

func createPost(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input postInput
		if err := c.ShouldBindJSON(&input); err != nil || !validPost(input) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "title and content are required"})
			return
		}

		now := time.Now().UTC()
		result, err := db.ExecContext(c.Request.Context(), `
			INSERT INTO posts (title, content, created_at, updated_at)
			VALUES (?, ?, ?, ?)
		`, strings.TrimSpace(input.Title), strings.TrimSpace(input.Content), now, now)
		if err != nil {
			writeDatabaseError(c, err)
			return
		}
		id, err := result.LastInsertId()
		if err != nil {
			writeDatabaseError(c, err)
			return
		}
		c.JSON(http.StatusCreated, Post{
			ID: id, Title: strings.TrimSpace(input.Title), Content: strings.TrimSpace(input.Content),
			CreatedAt: now, UpdatedAt: now,
		})
	}
}

func getPost(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := postID(c)
		if !ok {
			return
		}
		post, err := findPost(c, db, id)
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}
		if err != nil {
			writeDatabaseError(c, err)
			return
		}
		c.JSON(http.StatusOK, post)
	}
}

func updatePost(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := postID(c)
		if !ok {
			return
		}
		var input postInput
		if err := c.ShouldBindJSON(&input); err != nil || !validPost(input) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "title and content are required"})
			return
		}

		result, err := db.ExecContext(c.Request.Context(), `
			UPDATE posts SET title = ?, content = ?, updated_at = ? WHERE id = ?
		`, strings.TrimSpace(input.Title), strings.TrimSpace(input.Content), time.Now().UTC(), id)
		if err != nil {
			writeDatabaseError(c, err)
			return
		}
		count, err := result.RowsAffected()
		if err != nil {
			writeDatabaseError(c, err)
			return
		}
		if count == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}
		post, err := findPost(c, db, id)
		if err != nil {
			writeDatabaseError(c, err)
			return
		}
		c.JSON(http.StatusOK, post)
	}
}

func deletePost(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := postID(c)
		if !ok {
			return
		}
		result, err := db.ExecContext(c.Request.Context(), `DELETE FROM posts WHERE id = ?`, id)
		if err != nil {
			writeDatabaseError(c, err)
			return
		}
		count, err := result.RowsAffected()
		if err != nil {
			writeDatabaseError(c, err)
			return
		}
		if count == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}
		c.Status(http.StatusNoContent)
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

func findPost(c *gin.Context, db *sql.DB, id int64) (Post, error) {
	return scanPost(db.QueryRowContext(c.Request.Context(), `
		SELECT id, title, content, created_at, updated_at FROM posts WHERE id = ?
	`, id))
}

func postID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid post id"})
		return 0, false
	}
	return id, true
}

func validPost(input postInput) bool {
	return strings.TrimSpace(input.Title) != "" && strings.TrimSpace(input.Content) != ""
}

func writeDatabaseError(c *gin.Context, err error) {
	log.Printf("database request failed: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}
