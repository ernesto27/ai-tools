package sandbox

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

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

	found, err := sandboxWorktrees(".")
	require.NoError(t, err)
	require.Equal(t, repoDir, found.repo.Dir)
	wantWorktrees := records[:2]
	require.Equal(t, wantWorktrees, found.worktrees)

	var out bytes.Buffer
	require.NoError(t, WorktreeList(repoDir, &out))
	wantOutput := "much-longer  /worktrees/much-longer\n" +
		"short        /worktrees/short\n"
	require.Equal(t, wantOutput, out.String())

	branches, err := WorktreeBranches(".")
	require.NoError(t, err)
	require.Equal(t, []string{"much-longer", "short"}, branches)
}

func TestPrintWorktrees(t *testing.T) {
	for _, tt := range []struct {
		name      string
		worktrees []worktreeRecord
		want      string
	}{
		{"empty", nil, ""},
		{"single", []worktreeRecord{{Branch: "a", Path: "/worktrees/a"}}, "a  /worktrees/a\n"},
		{"aligned input order", []worktreeRecord{
			{Branch: "a", Path: "/worktrees/a"},
			{Branch: "long", Path: "/worktrees/long"},
		}, "a     /worktrees/a\nlong  /worktrees/long\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			printWorktrees(&out, tt.worktrees)
			require.Equal(t, tt.want, out.String())
		})
	}
}

func TestSandboxWorktreesRequiresGitRepository(t *testing.T) {
	dir := t.TempDir()
	changeToDir(t, dir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	for _, tt := range []struct{ name, input string }{
		{"current directory", "."},
		{"empty directory", ""},
		{"relative directory", "missing"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			wantDir := dir
			if tt.input == "missing" {
				wantDir = filepath.Join(dir, tt.input)
			}
			_, err := sandboxWorktrees(tt.input)
			require.EqualError(t, err, wantDir+" is not inside a Git repository")
		})
	}
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
	repoDir := setupListTestRepo(t)

	var out bytes.Buffer
	require.NoError(t, WorktreeList(repoDir, &out))
	require.Equal(t, "", out.String())
}

func TestWorktreeBranchesEmpty(t *testing.T) {
	setupListTestRepo(t)

	branches, err := WorktreeBranches(".")
	require.NoError(t, err)
	require.Nil(t, branches)
}

func TestSandboxWorktreesResolvesRepositoryPath(t *testing.T) {
	repoDir := setupListTestRepo(t)
	link := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.Symlink(repoDir, link))
	changeToDir(t, link)

	found, err := sandboxWorktrees(".")
	require.NoError(t, err)
	require.Equal(t, repoDir, found.repo.Dir)
}

func TestSandboxWorktreesExplicitDirectory(t *testing.T) {
	repoDir := setupListTestRepo(t)
	nested := filepath.Join(repoDir, "nested folder")
	require.NoError(t, os.Mkdir(nested, 0o755))
	link := filepath.Join(t.TempDir(), "repo link")
	require.NoError(t, os.Symlink(repoDir, link))
	require.NoError(t, recordWorktree(worktreeRecord{Repo: repoDir, Branch: "chosen"}))
	otherRepo := t.TempDir()
	output, err := exec.Command("git", "-C", otherRepo, "init", "--quiet").CombinedOutput()
	require.NoError(t, err, "%s", output)
	require.NoError(t, recordWorktree(worktreeRecord{Repo: otherRepo, Branch: "other"}))
	outside := t.TempDir()
	relative, err := filepath.Rel(outside, nested)
	require.NoError(t, err)
	for _, tt := range []struct{ name, cwd, dir string }{
		{"absolute outside Git", outside, repoDir},
		{"relative nested", outside, relative},
		{"symlink", outside, link},
		{"from another repository", otherRepo, nested},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(tt.cwd)
			found, err := sandboxWorktrees(tt.dir)
			require.NoError(t, err)
			require.Equal(t, repoDir, found.repo.Dir)
			require.Len(t, found.worktrees, 1)
			require.Equal(t, "chosen", found.worktrees[0].Branch)
			cwd, err := os.Getwd()
			require.NoError(t, err)
			require.Equal(t, tt.cwd, cwd)
		})
	}
}

func TestWorktreeBranchesReturnsRegistryValues(t *testing.T) {
	for _, kind := range []string{"directory", "symlink directory", "missing", "file", "broken symlink", "symlink loop", "other repository", "no registry"} {
		t.Run(kind, func(t *testing.T) {
			repoDir := setupListTestRepo(t)
			path := filepath.Join(t.TempDir(), "worktree")
			record := worktreeRecord{Repo: repoDir, Path: path, Branch: "feature"}
			want := []string{"feature"}
			switch kind {
			case "directory":
				require.NoError(t, os.Mkdir(path, 0o755))
			case "symlink directory":
				require.NoError(t, os.Symlink(t.TempDir(), path))
			case "file":
				require.NoError(t, os.WriteFile(path, nil, 0o600))
			case "broken symlink":
				require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "missing"), path))
			case "symlink loop":
				require.NoError(t, os.Symlink(path, path))
			case "other repository":
				require.NoError(t, os.Mkdir(path, 0o755))
				record.Repo = t.TempDir()
				want = nil
			}
			if kind != "no registry" {
				require.NoError(t, recordWorktree(record))
			} else {
				want = nil
			}
			branches, err := WorktreeBranches(repoDir)
			require.NoError(t, err)
			require.Equal(t, want, branches)
		})
	}
}

func TestWorktreeBranchesNewestFirst(t *testing.T) {
	repoDir := setupListTestRepo(t)
	for _, record := range []worktreeRecord{
		{Repo: repoDir, Branch: "z-last", Path: t.TempDir()},
		{Repo: repoDir, Branch: "stale", Path: filepath.Join(t.TempDir(), "missing")},
		{Repo: repoDir, Branch: "a-first", Path: t.TempDir()},
	} {
		require.NoError(t, recordWorktree(record))
	}
	branches, err := WorktreeBranches(".")
	require.NoError(t, err)
	require.Equal(t, []string{"a-first", "stale", "z-last"}, branches)
}

func TestWorktreeListingIgnoresTimestamps(t *testing.T) {
	for _, tt := range []struct {
		name    string
		created []time.Time
	}{
		{"missing", []time.Time{{}, {}, {}}},
		{"equal", []time.Time{time.Unix(10, 0), time.Unix(10, 0), time.Unix(10, 0)}},
		{"out of order", []time.Time{time.Unix(30, 0), time.Unix(10, 0), time.Unix(20, 0)}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := setupListTestRepo(t)
			for i, branch := range []string{"middle", "z-last", "a-first"} {
				require.NoError(t, recordWorktree(worktreeRecord{
					Repo: repoDir, Branch: branch, Path: "/worktrees/" + branch, Created: tt.created[i],
				}))
			}
			for range 2 {
				branches, err := WorktreeBranches(repoDir)
				require.NoError(t, err)
				require.Equal(t, []string{"a-first", "z-last", "middle"}, branches)
				var out bytes.Buffer
				require.NoError(t, WorktreeList(repoDir, &out))
				require.Equal(t, "a-first  /worktrees/a-first\nz-last   /worktrees/z-last\nmiddle   /worktrees/middle\n", out.String())
			}
			found, err := sandboxWorktrees(repoDir)
			require.NoError(t, err)
			require.Equal(t, "middle", found.worktrees[0].Branch)
		})
	}
}
