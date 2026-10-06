package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"agent-sandbox/internal/git"
)

// prContentFile lives in the coding worktree only until the host consumes it.
// The agent writes it after committing so PR metadata never enters the commit.
const prContentFile = ".agent-sandbox-pr.json"

type prContent struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// checkPRContentPath refuses collisions before the agent starts. A tracked
// path is reserved even when its working file is currently missing.
func checkPRContentPath(ctx context.Context, worktree *git.Repo) error {
	if err := requireUntrackedPRContent(ctx, worktree); err != nil {
		return err
	}
	path := filepath.Join(worktree.Dir, prContentFile)
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("PR artifact path already exists: %s; move it before running --pr", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking PR artifact path: %w", err)
	}
	return nil
}

func requireUntrackedPRContent(ctx context.Context, worktree *git.Repo) error {
	tracked, err := worktree.IsTracked(ctx, prContentFile)
	if err != nil {
		return fmt.Errorf("checking PR artifact tracking: %w", err)
	}
	if tracked {
		return fmt.Errorf("%s must not be staged or committed", prContentFile)
	}
	return nil
}

// consumePRContent validates before deleting and refuses tracked artifacts so
// a mistaken agent commit cannot turn cleanup into a tracked deletion. Invalid
// files remain in the worktree for inspection; valid content stays in memory.
func consumePRContent(ctx context.Context, worktree *git.Repo) (prContent, error) {
	if err := requireUntrackedPRContent(ctx, worktree); err != nil {
		return prContent{}, err
	}
	content, err := readPRContent(worktree.Dir)
	if err != nil {
		return prContent{}, err
	}
	if err := os.Remove(filepath.Join(worktree.Dir, prContentFile)); err != nil {
		return prContent{}, fmt.Errorf("removing PR artifact: %w", err)
	}
	return content, nil
}

func readPRContent(dir string) (prContent, error) {
	path := filepath.Join(dir, prContentFile)
	info, err := os.Lstat(path)
	if err != nil {
		return prContent{}, fmt.Errorf("reading generated PR artifact: %w", err)
	}
	if !info.Mode().IsRegular() {
		return prContent{}, fmt.Errorf("generated PR artifact must be a regular file, not a symlink")
	}
	// OpenRoot confines reads to the worktree even if the artifact is replaced
	// between the type check and opening it.
	root, err := os.OpenRoot(dir)
	if err != nil {
		return prContent{}, err
	}
	defer root.Close()
	file, err := root.Open(prContentFile)
	if err != nil {
		return prContent{}, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return prContent{}, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return prContent{}, fmt.Errorf("generated PR artifact changed while opening it")
	}
	var content prContent
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&content); err != nil {
		return prContent{}, fmt.Errorf("invalid generated PR JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return prContent{}, fmt.Errorf("generated PR artifact must contain exactly one JSON object")
	}
	if strings.ContainsAny(content.Title, "\r\n\u0085\u2028\u2029") {
		return prContent{}, fmt.Errorf("generated PR title must be a single line")
	}
	content.Title = strings.TrimSpace(content.Title)
	content.Body = strings.TrimSpace(content.Body)
	if content.Title == "" || content.Body == "" {
		return prContent{}, fmt.Errorf("generated PR title and body must be nonempty strings")
	}
	return content, nil
}
