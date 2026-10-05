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

type post struct {
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
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "blog.db"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := initializeDB(db); err != nil {
		log.Fatal(err)
	}

	router := gin.Default()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/api/posts", listPosts(db))
	router.POST("/api/posts", createPost(db))
	router.GET("/api/posts/:id", getPost(db))
	router.PUT("/api/posts/:id", updatePost(db))
	router.DELETE("/api/posts/:id", deletePost(db))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

func initializeDB(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	return err
}

func listPosts(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`SELECT id, title, content, created_at, updated_at FROM posts ORDER BY id DESC`)
		if err != nil {
			serverError(c, err)
			return
		}
		defer rows.Close()

		posts := make([]post, 0)
		for rows.Next() {
			var item post
			if err := rows.Scan(&item.ID, &item.Title, &item.Content, &item.CreatedAt, &item.UpdatedAt); err != nil {
				serverError(c, err)
				return
			}
			posts = append(posts, item)
		}
		if err := rows.Err(); err != nil {
			serverError(c, err)
			return
		}
		c.JSON(http.StatusOK, posts)
	}
}

func createPost(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input postInput
		if err := c.ShouldBindJSON(&input); err != nil || !validInput(input) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "title and content are required"})
			return
		}
		result, err := db.Exec(`INSERT INTO posts (title, content) VALUES (?, ?)`, strings.TrimSpace(input.Title), strings.TrimSpace(input.Content))
		if err != nil {
			serverError(c, err)
			return
		}
		id, err := result.LastInsertId()
		if err != nil {
			serverError(c, err)
			return
		}
		item, err := findPost(db, id)
		if err != nil {
			serverError(c, err)
			return
		}
		c.JSON(http.StatusCreated, item)
	}
}

func getPost(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := postID(c)
		if !ok {
			return
		}
		item, err := findPost(db, id)
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}
		if err != nil {
			serverError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	}
}

func updatePost(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := postID(c)
		if !ok {
			return
		}
		var input postInput
		if err := c.ShouldBindJSON(&input); err != nil || !validInput(input) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "title and content are required"})
			return
		}
		result, err := db.Exec(`UPDATE posts SET title = ?, content = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, strings.TrimSpace(input.Title), strings.TrimSpace(input.Content), id)
		if err != nil {
			serverError(c, err)
			return
		}
		count, err := result.RowsAffected()
		if err != nil {
			serverError(c, err)
			return
		}
		if count == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}
		item, err := findPost(db, id)
		if err != nil {
			serverError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	}
}

func deletePost(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := postID(c)
		if !ok {
			return
		}
		result, err := db.Exec(`DELETE FROM posts WHERE id = ?`, id)
		if err != nil {
			serverError(c, err)
			return
		}
		count, err := result.RowsAffected()
		if err != nil {
			serverError(c, err)
			return
		}
		if count == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func findPost(db *sql.DB, id int64) (post, error) {
	var item post
	err := db.QueryRow(`SELECT id, title, content, created_at, updated_at FROM posts WHERE id = ?`, id).
		Scan(&item.ID, &item.Title, &item.Content, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func postID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid post id"})
		return 0, false
	}
	return id, true
}

func validInput(input postInput) bool {
	return strings.TrimSpace(input.Title) != "" && strings.TrimSpace(input.Content) != ""
}

func serverError(c *gin.Context, err error) {
	log.Printf("request failed: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}
