package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerificationFiles(t *testing.T) {
	for _, tc := range []struct {
		name     string
		files    []string
		want     []string
		missing  bool
		relative bool
		symlink  bool
	}{
		{name: "missing folder", missing: true},
		{name: "empty folder"},
		{name: "recursive files", files: []string{"summary.md", "nested/desktop.png", "nested/deeper/check.log"}, want: []string{"summary.md", "nested/desktop.png", "nested/deeper/check.log"}},
		{name: "hidden entries", files: []string{".gitignore", ".secret", ".hidden/report.log", "nested/.secret", "nested/.cache/screenshot.png", "nested/report.log"}, want: []string{"nested/report.log"}},
		{name: "relative folder produces absolute paths", files: []string{"summary.md"}, want: []string{"summary.md"}, relative: true},
		{name: "symlinks excluded", files: []string{"summary.md"}, want: []string{"summary.md"}, symlink: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), verificationDir)
			if !tc.missing {
				require.NoError(t, os.MkdirAll(filepath.Join(root, "empty"), 0o755))
			}
			for _, name := range tc.files {
				path := filepath.Join(root, name)
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte("evidence"), 0o600))
			}
			if tc.symlink {
				outside := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("private"), 0o600))
				require.NoError(t, os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "linked.txt")))
				require.NoError(t, os.Symlink(outside, filepath.Join(root, "linked-directory")))
			}
			input := root
			if tc.relative {
				cwd, err := os.Getwd()
				require.NoError(t, err)
				input, err = filepath.Rel(cwd, root)
				require.NoError(t, err)
			}
			files, err := verificationFiles(input)
			require.NoError(t, err)
			var want []string
			for _, name := range tc.want {
				want = append(want, filepath.Join(root, name))
			}
			require.ElementsMatch(t, want, files)
		})
	}
}

func TestClearVerificationFiles(t *testing.T) {
	for _, tc := range []struct {
		name    string
		missing bool
		files   []string
	}{
		{name: "missing folder", missing: true},
		{name: "empty folder"},
		{name: "evidence and ignore rule", files: []string{".gitignore", "summary.md", "nested/desktop.png", ".hidden/report.log"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			worktree := t.TempDir()
			dir := filepath.Join(worktree, verificationDir)
			if !tc.missing {
				require.NoError(t, os.MkdirAll(dir, 0o755))
			}
			for _, name := range tc.files {
				path := filepath.Join(dir, name)
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte("*\n"), 0o600))
			}
			projectFile := filepath.Join(worktree, "main.rs")
			require.NoError(t, os.WriteFile(projectFile, []byte("project"), 0o600))
			require.NoError(t, clearVerificationFiles(worktree))
			content, err := os.ReadFile(projectFile)
			require.NoError(t, err)
			require.Equal(t, "project", string(content))
			if tc.missing {
				require.NoDirExists(t, dir)
				return
			}
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			if len(tc.files) == 0 {
				require.Empty(t, entries)
				return
			}
			require.Len(t, entries, 1)
			require.Equal(t, ".gitignore", entries[0].Name())
			ignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
			require.NoError(t, err)
			require.Equal(t, "*\n", string(ignore))
		})
	}
}
