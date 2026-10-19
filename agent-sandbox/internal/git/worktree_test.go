package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
			require.NoError(t, repo.AddWorktree(worktreeDir, "feature", true))
			if tt.dirty {
				require.NoError(t, os.WriteFile(filepath.Join(worktreeDir, "change.txt"), []byte("change\n"), 0o600))
			}

			err := repo.RemoveWorktree(worktreeDir, tt.force)
			if tt.wantRemoved {
				require.NoError(t, err)
				{
					_, err := os.Stat(worktreeDir)
					require.True(t, os.IsNotExist(err), "worktree path still exists after removal: %v", err)
				}
				require.False(t, worktreeRegistered(t, repo.Dir, worktreeDir), "worktree remains registered after removal")
				return
			}

			require.ErrorContains(t, err, "git worktree remove")
			{
				_, err := os.Stat(worktreeDir)
				require.NoError(t, err)
			}
			require.True(t, worktreeRegistered(t, repo.Dir, worktreeDir), "worktree was unregistered after a refused removal")
		})
	}
}

func TestRemoveWorktreeWrapsMissingPathError(t *testing.T) {
	repo := openTestRepo(t)
	err := repo.RemoveWorktree(filepath.Join(t.TempDir(), "missing"), false)
	require.ErrorContains(t, err, "git worktree remove")
}

func TestPruneWorktrees(t *testing.T) {
	repo := openTestRepo(t)
	worktreeDir := filepath.Join(t.TempDir(), "worktree")
	require.NoError(t, repo.AddWorktree(worktreeDir, "feature", true))
	require.NoError(t, os.RemoveAll(worktreeDir))

	require.NoError(t, repo.PruneWorktrees())
	require.False(t, worktreeRegistered(t, repo.Dir, worktreeDir), "worktree remains registered after PruneWorktrees")
}

func TestPruneWorktreesWrapsGitError(t *testing.T) {
	repo := &Repo{Dir: filepath.Join(t.TempDir(), "missing")}
	err := repo.PruneWorktrees()
	require.ErrorContains(t, err, "git worktree prune")
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
	require.NoError(t, err)
	return string(output)
}
