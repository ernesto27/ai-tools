package utils

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestGetContentFile(t *testing.T) {
	t.Run("returns the file contents unchanged", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "prompt.md")
		want := "# Fix login\n\nPreserve UTF-8: áéíóú.\n"
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatalf("write prompt file: %v", err)
		}

		got, err := GetContentFile(path)
		if err != nil {
			t.Fatalf("getContentFile() error = %v", err)
		}
		if got != want {
			t.Errorf("getContentFile() = %q, want %q", got, want)
		}
	})

	t.Run("returns the underlying error when the file is missing", func(t *testing.T) {
		_, err := GetContentFile(filepath.Join(t.TempDir(), "missing.md"))
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("getContentFile() error = %v, want an error matching fs.ErrNotExist", err)
		}
	})
}
