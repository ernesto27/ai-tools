package git

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	repoDir := setupTestRepo(t)
	subdir := filepath.Join(repoDir, "nested", "directory")
	require.NoError(t, os.MkdirAll(subdir, 0o755))

	repo, err := Open(subdir)
	require.NoError(t, err)
	require.Equal(t, repoDir, repo.Dir)

	outside := t.TempDir()
	_, err = Open(outside)
	require.ErrorContains(t, err, "is not inside a Git repository")
}

func TestRepoAt(t *testing.T) {
	repo := openTestRepo(t)
	otherDir := filepath.Join(repo.Dir, "other")

	at := repo.At(otherDir)
	require.Equal(t, otherDir, at.Dir)
	require.True(t, repo.Stdout == at.Stdout, "At must retain the same stdout writer")
	require.True(t, repo.Stderr == at.Stderr, "At must retain the same stderr writer")
}

func TestCheckBranchName(t *testing.T) {
	tests := []struct {
		name    string
		branch  string
		wantErr bool
	}{
		{name: "simple", branch: "feature"},
		{name: "slash", branch: "feature/login"},
		{name: "space", branch: "feature login", wantErr: true},
		{name: "double dot", branch: "feature..login", wantErr: true},
		{name: "leading dash", branch: "-feature", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckBranchName(tt.branch)
			require.Equal(t, tt.wantErr, (err != nil))
		})
	}
}

func TestBranchExistsAndDeleteBranch(t *testing.T) {
	repo := openTestRepo(t)
	require.False(t, repo.BranchExists("feature"), "feature branch exists before creation")
	runTestGit(t, repo.Dir, "branch", "feature")
	require.True(t, repo.BranchExists("feature"), "feature branch does not exist after creation")

	require.NoError(t, repo.DeleteBranch("feature"))
	require.False(t, repo.BranchExists("feature"), "feature branch exists after deletion")

	err := repo.DeleteBranch("missing")
	require.ErrorContains(t, err, "git branch -D")
}

func TestRemoteBranchExists(t *testing.T) {
	tests := []struct {
		name       string
		remoteRef  string
		localOnly  bool
		noOrigin   bool
		wantExists bool
		wantErr    bool
	}{
		{
			name:       "remote branch exists",
			remoteRef:  "refs/heads/feature",
			wantExists: true,
		},
		{
			name: "remote branch missing",
		},
		{
			name:      "local branch alone does not count",
			localOnly: true,
		},
		{
			name:      "tag alone does not count",
			remoteRef: "refs/tags/feature",
		},
		{
			name:     "missing origin returns error",
			noOrigin: true,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := openTestRepo(t)

			if !tt.noOrigin {
				remoteDir := setupTestRepo(t)
				runTestGit(t, repo.Dir, "remote", "add", "origin", remoteDir)

				if tt.remoteRef != "" {
					runTestGit(t, remoteDir, "update-ref", tt.remoteRef, "HEAD")
				}
			}

			if tt.localOnly {
				runTestGit(t, repo.Dir, "branch", "feature")
			}

			exists, err := repo.RemoteBranchExists("feature")
			require.Equal(t, tt.wantErr, (err != nil))
			require.Equal(t, tt.wantExists, exists)
			if tt.wantErr {
				require.ErrorContains(t, err, `checking remote branch "feature"`)
			}
		})
	}
}

func TestWorkingTreeLifecycle(t *testing.T) {
	repo := openTestRepo(t)

	clean, err := repo.IsClean()
	require.NoError(t, err)
	require.True(t, clean, "new repository is not clean")

	require.NoError(t, os.WriteFile(filepath.Join(repo.Dir, "change.txt"), []byte("change\n"), 0o600))
	clean, err = repo.IsClean()
	require.NoError(t, err)
	require.False(t, clean, "repository with an untracked file is clean")

	require.NoError(t, repo.StageAll())
	require.True(t, repo.HasStagedChanges(), "StageAll() did not stage the new file")
	require.NoError(t, repo.Commit("add change"))
	require.False(t, repo.HasStagedChanges(), "Commit() left staged changes")
	clean, err = repo.IsClean()
	require.NoError(t, err)
	require.True(t, clean, "repository is not clean after commit")
}

func TestAddWorktree(t *testing.T) {
	repo := openTestRepo(t)

	tests := []struct {
		name   string
		branch string
		create bool
	}{
		{name: "new branch", branch: "new-branch", create: true},
		{name: "existing branch", branch: "existing-branch"},
	}
	runTestGit(t, repo.Dir, "branch", "existing-branch")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worktreeDir := filepath.Join(t.TempDir(), "worktree")
			require.NoError(t, repo.AddWorktree(worktreeDir, tt.branch, tt.create))
			{
				info, err := os.Stat(worktreeDir)
				require.NoError(t, err)
				require.True(t, info.IsDir(), "worktree directory = (%v, %v), want existing directory", info, err)
			}
			require.True(t, repo.BranchExists(tt.branch), "branch %q does not exist after AddWorktree", tt.branch)
		})
	}
}

func TestPush(t *testing.T) {
	repo := openTestRepo(t)
	remoteDir := t.TempDir()
	command := exec.Command("git", "init", "--bare", "--quiet", remoteDir)
	{
		output, err := command.CombinedOutput()
		require.NoError(t, err, "command output: %s", output)
	}
	runTestGit(t, repo.Dir, "remote", "add", "origin", remoteDir)

	require.NoError(t, repo.Push("main"))

	command = exec.Command("git", "--git-dir", remoteDir, "rev-parse", "--verify", "refs/heads/main")
	{
		output, err := command.CombinedOutput()
		require.NoError(t, err, "command output: %s", output)
	}
}

func TestRunAndOutput(t *testing.T) {
	repo := openTestRepo(t)

	require.NoError(t, repo.run("status", "--porcelain"))
	require.Error(t, repo.run("rev-parse", "--verify", "missing-ref"))

	got, err := repo.output("rev-parse", "--show-toplevel")
	require.NoError(t, err)
	require.Equal(t, repo.Dir, got)
}

func openTestRepo(t *testing.T) *Repo {
	t.Helper()

	repo, err := Open(setupTestRepo(t))
	require.NoError(t, err)
	repo.Stdout = io.Discard
	repo.Stderr = io.Discard
	return repo
}

func setupTestRepo(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()
	runTestGit(t, repoDir, "init", "--quiet", "--initial-branch=main")
	runTestGit(t, repoDir, "config", "user.name", "Test User")
	runTestGit(t, repoDir, "config", "user.email", "test@example.com")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("initial\n"), 0o600))
	runTestGit(t, repoDir, "add", "README.md")
	runTestGit(t, repoDir, "commit", "--quiet", "-m", "initial")
	return repoDir
}

func runTestGit(t *testing.T, repoDir string, args ...string) {
	t.Helper()

	command := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
	{
		output, err := command.CombinedOutput()
		require.NoError(t, err, "command output: %s", output)
	}
}
