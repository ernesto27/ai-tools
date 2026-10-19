package sandbox

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorktreeDelete(t *testing.T) {
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
			repoDir := setupDeleteTestRepo(t)
			worktree := addDeleteTestWorktree(t, repoDir, "feature")
			if tt.dirty {
				require.NoError(t, os.WriteFile(filepath.Join(worktree.Path, "change.txt"), []byte("change"), 0o600))
			}
			require.NoError(t, recordWorktree(worktree))

			var out bytes.Buffer
			err := WorktreeDelete(worktree.Branch, tt.force, &out)
			if tt.wantRemoved {
				require.NoError(t, err)
				{
					_, err := os.Stat(worktree.Path)
					require.True(t, os.IsNotExist(err), "worktree path still exists after deletion: %v", err)
				}
				require.False(t, branchExists(t, repoDir, worktree.Branch), "branch %q still exists after deletion", worktree.Branch)
				records, err := recordedWorktrees(repoDir)
				require.NoError(t, err)
				require.Empty(t, records)
				return
			}

			want := worktree.Path + " has uncommitted changes; pass --force to delete it anyway"
			require.EqualError(t, err, want)
			{
				_, err := os.Stat(worktree.Path)
				require.NoError(t, err)
			}
			require.True(t, branchExists(t, repoDir, worktree.Branch), "branch %q was deleted", worktree.Branch)
		})
	}
}

func TestWorktreeDeleteRejectsUnrecordedBranch(t *testing.T) {
	setupDeleteTestRepo(t)

	err := WorktreeDelete("missing", false, &bytes.Buffer{})
	want := "no sandbox worktree on branch missing; run worktree-list to see the ones there are"
	require.EqualError(t, err, want)
}

func TestWorktreeDeletePrunesMissingDirectory(t *testing.T) {
	repoDir := setupDeleteTestRepo(t)
	worktree := addDeleteTestWorktree(t, repoDir, "feature")
	require.NoError(t, recordWorktree(worktree))
	require.NoError(t, os.RemoveAll(worktree.Path))

	var out bytes.Buffer
	require.NoError(t, WorktreeDelete(worktree.Branch, false, &out))
	require.Contains(t, out.String(), "Pruned the record of "+worktree.Path)
	require.False(t, branchExists(t, repoDir, worktree.Branch), "branch %q still exists after deletion", worktree.Branch)
}

func TestWorktreeDeleteAll(t *testing.T) {
	tests := []struct {
		name       string
		assumeYes  bool
		input      string
		wantDelete bool
	}{
		{name: "declined", input: "n\n"},
		{name: "yes flag", assumeYes: true, wantDelete: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := setupDeleteTestRepo(t)
			first := addDeleteTestWorktree(t, repoDir, "first")
			second := addDeleteTestWorktree(t, repoDir, "second")
			for _, worktree := range []worktreeRecord{first, second} {
				require.NoError(t, recordWorktree(worktree))
			}

			var out bytes.Buffer
			err := WorktreeDeleteAll(tt.assumeYes, strings.NewReader(tt.input), &out)
			require.NoError(t, err)
			if !tt.wantDelete {
				require.Contains(t, out.String(), "Aborted; nothing was deleted.")
				require.True(t, branchExists(t, repoDir, first.Branch), "WorktreeDeleteAll() deleted a branch after a declined confirmation")
				require.True(t, branchExists(t, repoDir, second.Branch), "WorktreeDeleteAll() deleted a branch after a declined confirmation")
				return
			}

			for _, worktree := range []worktreeRecord{first, second} {
				{
					_, err := os.Stat(worktree.Path)
					require.True(t, os.IsNotExist(err), "worktree path still exists after deletion: %v", err)
				}
				require.False(t, branchExists(t, repoDir, worktree.Branch), "branch %q still exists after deletion", worktree.Branch)
			}
		})
	}
}

func TestFindWorktree(t *testing.T) {
	worktrees := []worktreeRecord{{Branch: "first"}, {Branch: "second"}}

	tests := []struct {
		branch string
		want   worktreeRecord
		found  bool
	}{
		{branch: "first", want: worktrees[0], found: true},
		{branch: "missing"},
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			got, found := findWorktree(worktrees, tt.branch)
			require.Equal(t, tt.want, got)
			require.Equal(t, tt.found, found)
		})
	}
}

func TestIsInside(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	require.NoError(t, os.Mkdir(child, 0o755))
	sibling := t.TempDir()

	tests := []struct {
		name string
		dir  string
		root string
		want bool
	}{
		{name: "root", dir: root, root: root, want: true},
		{name: "child", dir: child, root: root, want: true},
		{name: "sibling", dir: sibling, root: root},
		{name: "missing root", dir: root, root: filepath.Join(root, "missing")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isInside(tt.dir, tt.root)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestResolvePath(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(dir, link))

	require.Equal(t, dir, resolvePath(link))

	missing := filepath.Join(dir, "missing", "path")
	{
		got, want := resolvePath(missing), filepath.Clean(missing)
		require.Equal(t, want, got)
	}
}

func setupDeleteTestRepo(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "--quiet")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("initial\n"), 0o600))
	runGit(t, repoDir, "add", "README.md")
	runGit(t, repoDir,
		"-c", "user.name=Test User",
		"-c", "user.email=test@example.com",
		"commit", "--quiet", "-m", "initial")

	changeToDir(t, repoDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return repoDir
}

func addDeleteTestWorktree(t *testing.T, repoDir, branch string) worktreeRecord {
	t.Helper()

	path := filepath.Join(t.TempDir(), "worktree")
	runGit(t, repoDir, "worktree", "add", "--quiet", "-b", branch, path)
	return worktreeRecord{Repo: repoDir, Path: path, Branch: branch}
}

func branchExists(t *testing.T, repoDir, branch string) bool {
	t.Helper()

	command := exec.Command("git", "-C", repoDir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return command.Run() == nil
}

func runGit(t *testing.T, repoDir string, args ...string) {
	t.Helper()

	command := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
	{
		output, err := command.CombinedOutput()
		require.NoError(t, err, "command output: %s", output)
	}
}
