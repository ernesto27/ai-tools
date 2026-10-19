package utils

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetContentFile(t *testing.T) {
	t.Run("returns the file contents unchanged", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "prompt.md")
		want := "# Fix login\n\nPreserve UTF-8: áéíóú.\n"
		require.NoError(t, os.WriteFile(path, []byte(want), 0o644))

		got, err := GetContentFile(path)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("returns the underlying error when the file is missing", func(t *testing.T) {
		_, err := GetContentFile(filepath.Join(t.TempDir(), "missing.md"))
		assert.ErrorIs(t, err, fs.ErrNotExist)
	})
}
