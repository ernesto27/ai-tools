package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"soft-factory/internal/config"
)

// setupCriticalRepo creates a temporary repository with a "feature" worktree
// whose commit changes go.mod and README.md and adds
// internal/config/config.go and internal/configx/extra.go.
func setupCriticalRepo(t *testing.T, project string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	repo := filepath.Join(root, "repo")
	worktree := filepath.Join(root, "feature")
	git := func(dir string, args ...string) {
		t.Helper()
		output, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		require.NoError(t, err, "%s", output)
	}
	write := func(dir, name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
		require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	}

	require.NoError(t, os.Mkdir(repo, 0700))
	git(repo, "init", "-b", "main")
	git(repo, "config", "user.name", "Test")
	git(repo, "config", "user.email", "test@example.com")
	for _, name := range []string{"go.mod", "README.md"} {
		write(repo, filepath.Join(project, name), "base\n")
	}
	git(repo, "add", ".")
	git(repo, "commit", "-m", "initial")
	git(repo, "worktree", "add", "-b", "feature", worktree)
	for _, name := range []string{"go.mod", "README.md"} {
		write(worktree, filepath.Join(project, name), "base\nfeature\n")
	}
	write(worktree, filepath.Join(project, "internal/config/config.go"), "package config\n")
	write(worktree, filepath.Join(project, "internal/configx/extra.go"), "package configx\n")
	if project != "" {
		write(worktree, "internal/config/outside.go", "package config\n")
	}
	git(worktree, "add", ".")
	git(worktree, "commit", "-m", "feature change")
	t.Chdir(filepath.Join(repo, project))
}

func TestCriticalChangesFiles(t *testing.T) {
	setupCriticalRepo(t, "")

	tests := []struct {
		name     string
		critical config.Critical
		want     []string
	}{
		{
			name:     "empty configuration",
			critical: config.Critical{},
			want:     []string{},
		},
		{
			name:     "one critical file changed",
			critical: config.Critical{Files: []string{"go.mod"}},
			want:     []string{"go.mod"},
		},
		{
			name:     "several critical files changed",
			critical: config.Critical{Files: []string{"go.mod", "README.md"}},
			want:     []string{"README.md", "go.mod"},
		},
		{
			name:     "file in a subfolder",
			critical: config.Critical{Files: []string{"internal/config/config.go"}},
			want:     []string{"internal/config/config.go"},
		},
		{
			name:     "critical file not changed",
			critical: config.Critical{Files: []string{"LICENSE"}},
			want:     []string{},
		},
		{
			name:     "folder path in files does not match",
			critical: config.Critical{Files: []string{"internal/config"}},
			want:     []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := CriticalChanges(context.Background(), "feature", tt.critical)
			require.NoError(t, err)
			require.Equal(t, tt.want, files)
		})
	}
}

func TestCriticalSection(t *testing.T) {
	require.Empty(t, CriticalSection(nil))
	require.Equal(t,
		"## ⚠️ Critical files changed\n"+
			"The following files are marked as critical in software-factory.json:\n"+
			"- `go.mod`\n"+
			"- `internal/config/config.go`\n",
		CriticalSection([]string{"go.mod", "internal/config/config.go"}))
}

func TestCriticalChangesFromProjectSubdirectory(t *testing.T) {
	setupCriticalRepo(t, "soft-factory")

	files, err := CriticalChanges(context.Background(), "feature", config.Critical{
		Files:   []string{"go.mod"},
		Folders: []string{"internal/config"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"go.mod", "internal/config/config.go"}, files)
}

func TestCriticalChangesFolders(t *testing.T) {
	setupCriticalRepo(t, "")

	tests := []struct {
		name     string
		critical config.Critical
		want     []string
	}{
		{
			name:     "folder matches files inside it only",
			critical: config.Critical{Folders: []string{"internal/config"}},
			want:     []string{"internal/config/config.go"},
		},
		{
			name:     "trailing slash",
			critical: config.Critical{Folders: []string{"internal/config/"}},
			want:     []string{"internal/config/config.go"},
		},
		{
			name:     "parent folder matches nested files",
			critical: config.Critical{Folders: []string{"internal"}},
			want:     []string{"internal/config/config.go", "internal/configx/extra.go"},
		},
		{
			name:     "nested folders do not duplicate files",
			critical: config.Critical{Folders: []string{"internal", "internal/config"}},
			want:     []string{"internal/config/config.go", "internal/configx/extra.go"},
		},
		{
			name:     "folder not changed",
			critical: config.Critical{Folders: []string{"docs"}},
			want:     []string{},
		},
		{
			name:     "file path in folders does not match",
			critical: config.Critical{Folders: []string{"go.mod"}},
			want:     []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := CriticalChanges(context.Background(), "feature", tt.critical)
			require.NoError(t, err)
			require.Equal(t, tt.want, files)
		})
	}
}
