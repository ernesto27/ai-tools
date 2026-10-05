package blog

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("post not found")

// Store defines the operations the HTTP API needs from a post repository.
type Store interface {
	List(query string, offset, limit int) ([]Post, int)
	Get(id string) (Post, error)
	Create(input PostInput) (Post, error)
	Update(id string, input PostInput) (Post, error)
	Delete(id string) error
}

// FileStore keeps posts in memory and atomically writes each change to disk.
type FileStore struct {
	mu    sync.RWMutex
	path  string
	posts map[string]Post
}

func NewFileStore(path string) (*FileStore, error) {
	s := &FileStore{path: path, posts: make(map[string]Post)}
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read posts: %w", err)
		}
		return s, nil
	}

	var posts []Post
	if err := json.Unmarshal(data, &posts); err != nil {
		return nil, fmt.Errorf("decode posts: %w", err)
	}
	for _, post := range posts {
		if post.ID == "" {
			return nil, errors.New("decode posts: post has an empty id")
		}
		s.posts[post.ID] = post
	}
	return s, nil
}

func (s *FileStore) List(query string, offset, limit int) ([]Post, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query = strings.ToLower(strings.TrimSpace(query))
	all := make([]Post, 0, len(s.posts))
	for _, post := range s.posts {
		if query == "" || matches(post, query) {
			all = append(all, post)
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].ID < all[j].ID
		}
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	total := len(all)
	if offset >= total {
		return []Post{}, total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return all[offset:end], total
}

func (s *FileStore) Get(id string) (Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	post, ok := s.posts[id]
	if !ok {
		return Post{}, ErrNotFound
	}
	return post, nil
}

func (s *FileStore) Create(input PostInput) (Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	post := Post{
		ID:        newID(),
		Slug:      s.uniqueSlug(slugify(input.Title), ""),
		Title:     strings.TrimSpace(input.Title),
		Excerpt:   excerpt(input),
		Content:   strings.TrimSpace(input.Content),
		Author:    strings.TrimSpace(input.Author),
		Tags:      cleanTags(input.Tags),
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.posts[post.ID] = post
	if err := s.persist(); err != nil {
		delete(s.posts, post.ID)
		return Post{}, err
	}
	return post, nil
}

func (s *FileStore) Update(id string, input PostInput) (Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous, ok := s.posts[id]
	if !ok {
		return Post{}, ErrNotFound
	}
	post := previous
	post.Slug = s.uniqueSlug(slugify(input.Title), id)
	post.Title = strings.TrimSpace(input.Title)
	post.Excerpt = excerpt(input)
	post.Content = strings.TrimSpace(input.Content)
	post.Author = strings.TrimSpace(input.Author)
	post.Tags = cleanTags(input.Tags)
	post.UpdatedAt = time.Now().UTC()
	s.posts[id] = post
	if err := s.persist(); err != nil {
		s.posts[id] = previous
		return Post{}, err
	}
	return post, nil
}

func (s *FileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	post, ok := s.posts[id]
	if !ok {
		return ErrNotFound
	}
	delete(s.posts, id)
	if err := s.persist(); err != nil {
		s.posts[id] = post
		return err
	}
	return nil
}

func (s *FileStore) persist() error {
	posts := make([]Post, 0, len(s.posts))
	for _, post := range s.posts {
		posts = append(posts, post)
	}
	sort.Slice(posts, func(i, j int) bool { return posts[i].ID < posts[j].ID })

	data, err := json.MarshalIndent(posts, "", "  ")
	if err != nil {
		return fmt.Errorf("encode posts: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".posts-*.json")
	if err != nil {
		return fmt.Errorf("create temporary posts file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write posts: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync posts: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close posts: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace posts file: %w", err)
	}
	return nil
}

func (s *FileStore) uniqueSlug(base, exceptID string) string {
	if base == "" {
		base = "post"
	}
	candidate := base
	for suffix := 2; ; suffix++ {
		available := true
		for id, post := range s.posts {
			if id != exceptID && post.Slug == candidate {
				available = false
				break
			}
		}
		if available {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, suffix)
	}
}

func newID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func slugify(value string) string {
	var b strings.Builder
	separator := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if separator && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			separator = false
		} else {
			separator = true
		}
	}
	return b.String()
}

func excerpt(input PostInput) string {
	if value := strings.TrimSpace(input.Excerpt); value != "" {
		return value
	}
	content := strings.TrimSpace(input.Content)
	runes := []rune(content)
	if len(runes) > 180 {
		content = string(runes[:180])
	}
	return content
}

func cleanTags(tags []string) []string {
	cleaned := make([]string, 0, len(tags))
	seen := make(map[string]bool)
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		key := strings.ToLower(tag)
		if tag != "" && !seen[key] {
			seen[key] = true
			cleaned = append(cleaned, tag)
		}
	}
	return cleaned
}

func matches(post Post, query string) bool {
	if strings.Contains(strings.ToLower(post.Title+" "+post.Excerpt+" "+post.Content+" "+post.Author), query) {
		return true
	}
	for _, tag := range post.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}
