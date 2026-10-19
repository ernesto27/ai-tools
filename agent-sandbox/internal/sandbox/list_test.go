package sandbox

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorktreeListAndBranchesUseCurrentRepositoryRecords(t *testing.T) {
	repoDir := setupListTestRepo(t)
	records := []worktreeRecord{
		{Repo: repoDir, Branch: "short", Path: "/worktrees/short"},
		{Repo: repoDir, Branch: "much-longer", Path: "/worktrees/much-longer"},
		{Repo: t.TempDir(), Branch: "other-repo", Path: "/worktrees/other"},
	}
	for _, record := range records {
		require.NoError(t, recordWorktree(record))
	}

	found, err := sandboxWorktrees()
	require.NoError(t, err)
	require.Equal(t, repoDir, found.repo.Dir)
	wantWorktrees := records[:2]
	require.Equal(t, wantWorktrees, found.worktrees)

	var out bytes.Buffer
	require.NoError(t, WorktreeList(&out))
	wantOutput := "short        /worktrees/short\n" +
		"much-longer  /worktrees/much-longer\n"
	require.Equal(t, wantOutput, out.String())

	branches, err := WorktreeBranches()
	require.NoError(t, err)
	require.Equal(t, []string{"short", "much-longer"}, branches)
}

func TestPrintWorktrees(t *testing.T) {
	worktrees := []worktreeRecord{
		{Branch: "a", Path: "/worktrees/a"},
		{Branch: "long", Path: "/worktrees/long"},
	}

	var out bytes.Buffer
	printWorktrees(&out, worktrees)

	want := "a     /worktrees/a\nlong  /worktrees/long\n"
	require.Equal(t, want, out.String())
}

func TestSandboxWorktreesRequiresGitRepository(t *testing.T) {
	dir := t.TempDir()
	changeToDir(t, dir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, err := sandboxWorktrees()
	require.Error(t, err)
}

func setupListTestRepo(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()
	command := exec.Command("git", "-C", repoDir, "init", "--quiet")
	{
		output, err := command.CombinedOutput()
		require.NoError(t, err, "command output: %s", output)
	}

	changeToDir(t, repoDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return repoDir
}

func changeToDir(t *testing.T, dir string) {
	t.Helper()

	previous, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		assert.NoError(t, os.Chdir(previous))
	})
}

func TestWorktreeListEmpty(t *testing.T) {
	setupListTestRepo(t)

	var out bytes.Buffer
	require.NoError(t, WorktreeList(&out))
	require.Equal(t, "", out.String())
}

func TestWorktreeBranchesEmpty(t *testing.T) {
	setupListTestRepo(t)

	branches, err := WorktreeBranches()
	require.NoError(t, err)
	require.Nil(t, branches)
}

func TestSandboxWorktreesResolvesRepositoryPath(t *testing.T) {
	repoDir := setupListTestRepo(t)
	link := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.Symlink(repoDir, link))
	changeToDir(t, link)

	found, err := sandboxWorktrees()
	require.NoError(t, err)
	require.Equal(t, repoDir, found.repo.Dir)
}
