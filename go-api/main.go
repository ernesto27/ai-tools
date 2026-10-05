package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

type Post struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type postInput struct {
	Title string `json:"title" binding:"required"`
	Body  string `json:"body" binding:"required"`
}

func main() {
	databasePath := os.Getenv("DATABASE_PATH")
	if databasePath == "" {
		databasePath = "blog.db"
	}

	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	if err := initializeDatabase(db); err != nil {
		log.Fatalf("initialize database: %v", err)
	}

	router := gin.Default()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/posts", listPosts(db))
	router.POST("/posts", createPost(db))
	router.GET("/posts/:id", getPost(db))
	router.PUT("/posts/:id", updatePost(db))
	router.DELETE("/posts/:id", deletePost(db))

	if err := router.Run(":8080"); err != nil {
		log.Fatalf("run server: %v", err)
	}
}

func initializeDatabase(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func listPosts(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.QueryContext(c.Request.Context(), `
			SELECT id, title, body, created_at, updated_at FROM posts ORDER BY id DESC
		`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list posts"})
			return
		}
		defer rows.Close()

		posts := make([]Post, 0)
		for rows.Next() {
			post, err := scanPost(rows)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read posts"})
				return
			}
			posts = append(posts, post)
		}
		if err := rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read posts"})
			return
		}
		c.JSON(http.StatusOK, posts)
	}
}

func createPost(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input postInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "title and body are required"})
			return
		}

		result, err := db.ExecContext(c.Request.Context(),
			`INSERT INTO posts (title, body) VALUES (?, ?)`, input.Title, input.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create post"})
			return
		}
		id, err := result.LastInsertId()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create post"})
			return
		}
		post, err := findPost(c, db, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load post"})
			return
		}
		c.JSON(http.StatusCreated, post)
	}
}

func getPost(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := postID(c)
		if !ok {
			return
		}
		post, err := findPost(c, db, id)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load post"})
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
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "title and body are required"})
			return
		}

		result, err := db.ExecContext(c.Request.Context(), `
			UPDATE posts SET title = ?, body = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
		`, input.Title, input.Body, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update post"})
			return
		}
		count, err := result.RowsAffected()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update post"})
			return
		}
		if count == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}
		post, err := findPost(c, db, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load post"})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete post"})
			return
		}
		count, err := result.RowsAffected()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete post"})
			return
		}
		if count == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func postID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid post id"})
		return 0, false
	}
	return id, true
}

func findPost(c *gin.Context, db *sql.DB, id int64) (Post, error) {
	return scanPost(db.QueryRowContext(c.Request.Context(), `
		SELECT id, title, body, created_at, updated_at FROM posts WHERE id = ?
	`, id))
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPost(row rowScanner) (Post, error) {
	var post Post
	var createdAt, updatedAt string
	err := row.Scan(&post.ID, &post.Title, &post.Body, &createdAt, &updatedAt)
	if err != nil {
		return Post{}, err
	}
	post.CreatedAt, err = parseSQLiteTime(createdAt)
	if err != nil {
		return Post{}, err
	}
	post.UpdatedAt, err = parseSQLiteTime(updatedAt)
	return post, err
}

func parseSQLiteTime(value string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, strconv.ErrSyntax
}
