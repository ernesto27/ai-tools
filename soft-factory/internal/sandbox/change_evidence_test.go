package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"soft-factory/internal/executionlog"
)

func TestChangeEvidenceIncludesBranchAndWorktreeChanges(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	worktree := filepath.Join(root, "feature")
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", output)
	}
	require.NoError(t, os.Mkdir(repo, 0700))
	git(repo, "init", "-b", "main")
	git(repo, "config", "user.name", "Test")
	git(repo, "config", "user.email", "test@example.com")
	tracked := filepath.Join(repo, "tracked.txt")
	require.NoError(t, os.WriteFile(tracked, []byte("base\n"), 0600))
	git(repo, "add", "tracked.txt")
	git(repo, "commit", "-m", "initial")
	git(repo, "worktree", "add", "-b", "feature", worktree)
	tracked = filepath.Join(worktree, "tracked.txt")
	require.NoError(t, os.WriteFile(tracked, []byte("base\ncommitted\n"), 0600))
	git(worktree, "add", "tracked.txt")
	git(worktree, "commit", "-m", "feature change")
	require.NoError(t, os.WriteFile(tracked, []byte("base\ncommitted\nstaged\n"), 0600))
	git(worktree, "add", "tracked.txt")
	require.NoError(t, os.WriteFile(tracked, []byte("base\ncommitted\nstaged\nunstaged\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(worktree, "new file.txt"), []byte("untracked\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "agent-sandbox.json"), []byte(`{"resume":{"branch":"feature"}}`), 0600))
	t.Chdir(repo)
	snapshot, err := changeEvidence(context.Background(), "feature")
	require.NoError(t, err)
	require.Equal(t, worktree, snapshot.Worktree)
	// An explicit continuation branch must override a different configured branch.
	require.NoError(t, os.WriteFile(filepath.Join(repo, "agent-sandbox.json"), []byte(`{"resume":{"branch":"main"}}`), 0600))
	snapshot, err = changeEvidence(context.Background(), "feature")
	require.NoError(t, err)
	require.Equal(t, worktree, snapshot.Worktree)
	reportPath, err := executionlog.StageReportPath(context.Background(), "feature", "review.md")
	require.NoError(t, err)

	want := filepath.Join(worktree, "review.md")
	require.Equal(t, want, reportPath)

	for _, want := range []string{"Comparison: merge base of HEAD and main", "+committed", "+staged", "+unstaged", `"new file.txt"`} {
		assert.Contains(t, snapshot.Text, want)
	}
}
