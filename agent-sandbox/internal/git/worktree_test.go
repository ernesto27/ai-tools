package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveWorktree(t *testing.T) {
	tests := []struct {
		name        string
		force       bool
		dirty       bool
		wantRemoved bool
	}{
		{name: "clean worktree", wantRemoved: true},
		{name: "dirty worktree without force", dirty: true},
		{name: "dirty worktree with force", dirty: true, force: true, wantRemoved: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := openTestRepo(t)
			worktreeDir := filepath.Join(t.TempDir(), "worktree")
			if err := repo.AddWorktree(worktreeDir, "feature", true); err != nil {
				t.Fatal(err)
			}
			if tt.dirty {
				if err := os.WriteFile(filepath.Join(worktreeDir, "change.txt"), []byte("change\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			err := repo.RemoveWorktree(worktreeDir, tt.force)
			if tt.wantRemoved {
				if err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(worktreeDir); !os.IsNotExist(err) {
					t.Fatalf("worktree path still exists after removal: %v", err)
				}
				if worktreeRegistered(t, repo.Dir, worktreeDir) {
					t.Fatal("worktree remains registered after removal")
				}
				return
			}

			if err == nil || !strings.Contains(err.Error(), "git worktree remove") {
				t.Fatalf("RemoveWorktree() error = %v, want wrapped git error", err)
			}
			if _, err := os.Stat(worktreeDir); err != nil {
				t.Fatalf("worktree path was removed: %v", err)
			}
			if !worktreeRegistered(t, repo.Dir, worktreeDir) {
				t.Fatal("worktree was unregistered after a refused removal")
			}
		})
	}
}

func TestRemoveWorktreeWrapsMissingPathError(t *testing.T) {
	repo := openTestRepo(t)
	err := repo.RemoveWorktree(filepath.Join(t.TempDir(), "missing"), false)
	if err == nil || !strings.Contains(err.Error(), "git worktree remove") {
		t.Fatalf("RemoveWorktree() error = %v, want wrapped git error", err)
	}
}

func TestPruneWorktrees(t *testing.T) {
	repo := openTestRepo(t)
	worktreeDir := filepath.Join(t.TempDir(), "worktree")
	if err := repo.AddWorktree(worktreeDir, "feature", true); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(worktreeDir); err != nil {
		t.Fatal(err)
	}

	if err := repo.PruneWorktrees(); err != nil {
		t.Fatal(err)
	}
	if worktreeRegistered(t, repo.Dir, worktreeDir) {
		t.Fatal("worktree remains registered after PruneWorktrees")
	}
}

func TestPruneWorktreesWrapsGitError(t *testing.T) {
	repo := &Repo{Dir: filepath.Join(t.TempDir(), "missing")}
	err := repo.PruneWorktrees()
	if err == nil || !strings.Contains(err.Error(), "git worktree prune") {
		t.Fatalf("PruneWorktrees() error = %v, want wrapped git error", err)
	}
}

func worktreeRegistered(t *testing.T, repoDir, worktreeDir string) bool {
	t.Helper()

	command := runGitOutput(t, repoDir, "worktree", "list", "--porcelain")
	return strings.Contains(command, "worktree "+worktreeDir+"\n")
}

func runGitOutput(t *testing.T, repoDir string, args ...string) string {
	t.Helper()

	command := append([]string{"-C", repoDir}, args...)
	output, err := exec.Command("git", command...).Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(output)
}
