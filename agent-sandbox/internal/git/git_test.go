package git

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpen(t *testing.T) {
	repoDir := setupTestRepo(t)
	subdir := filepath.Join(repoDir, "nested", "directory")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	repo, err := Open(subdir)
	if err != nil {
		t.Fatal(err)
	}
	if repo.Dir != repoDir {
		t.Fatalf("Open(%q).Dir = %q, want %q", subdir, repo.Dir, repoDir)
	}

	outside := t.TempDir()
	_, err = Open(outside)
	if err == nil || !strings.Contains(err.Error(), "is not inside a Git repository") {
		t.Fatalf("Open(%q) error = %v, want repository error", outside, err)
	}
}

func TestRepoAt(t *testing.T) {
	repo := openTestRepo(t)
	otherDir := filepath.Join(repo.Dir, "other")

	at := repo.At(otherDir)
	if at.Dir != otherDir {
		t.Fatalf("At(%q).Dir = %q", otherDir, at.Dir)
	}
	if at.Stdout != repo.Stdout || at.Stderr != repo.Stderr {
		t.Fatal("At() did not preserve the repository streams")
	}
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
			if (err != nil) != tt.wantErr {
				t.Fatalf("CheckBranchName(%q) error = %v, want error: %t", tt.branch, err, tt.wantErr)
			}
		})
	}
}

func TestBranchExistsAndDeleteBranch(t *testing.T) {
	repo := openTestRepo(t)
	if repo.BranchExists("feature") {
		t.Fatal("feature branch exists before creation")
	}
	runTestGit(t, repo.Dir, "branch", "feature")
	if !repo.BranchExists("feature") {
		t.Fatal("feature branch does not exist after creation")
	}

	if err := repo.DeleteBranch("feature"); err != nil {
		t.Fatal(err)
	}
	if repo.BranchExists("feature") {
		t.Fatal("feature branch exists after deletion")
	}

	err := repo.DeleteBranch("missing")
	if err == nil || !strings.Contains(err.Error(), "git branch -D") {
		t.Fatalf("DeleteBranch(missing) error = %v, want wrapped git error", err)
	}
}

func TestWorkingTreeLifecycle(t *testing.T) {
	repo := openTestRepo(t)

	clean, err := repo.IsClean()
	if err != nil {
		t.Fatal(err)
	}
	if !clean {
		t.Fatal("new repository is not clean")
	}

	if err := os.WriteFile(filepath.Join(repo.Dir, "change.txt"), []byte("change\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	clean, err = repo.IsClean()
	if err != nil {
		t.Fatal(err)
	}
	if clean {
		t.Fatal("repository with an untracked file is clean")
	}

	if err := repo.StageAll(); err != nil {
		t.Fatal(err)
	}
	if !repo.HasStagedChanges() {
		t.Fatal("StageAll() did not stage the new file")
	}
	if err := repo.Commit("add change"); err != nil {
		t.Fatal(err)
	}
	if repo.HasStagedChanges() {
		t.Fatal("Commit() left staged changes")
	}
	clean, err = repo.IsClean()
	if err != nil {
		t.Fatal(err)
	}
	if !clean {
		t.Fatal("repository is not clean after commit")
	}
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
			if err := repo.AddWorktree(worktreeDir, tt.branch, tt.create); err != nil {
				t.Fatal(err)
			}
			if info, err := os.Stat(worktreeDir); err != nil || !info.IsDir() {
				t.Fatalf("worktree directory = (%v, %v), want existing directory", info, err)
			}
			if !repo.BranchExists(tt.branch) {
				t.Fatalf("branch %q does not exist after AddWorktree", tt.branch)
			}
		})
	}
}

func TestPush(t *testing.T) {
	repo := openTestRepo(t)
	remoteDir := t.TempDir()
	command := exec.Command("git", "init", "--bare", "--quiet", remoteDir)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, output)
	}
	runTestGit(t, repo.Dir, "remote", "add", "origin", remoteDir)

	if err := repo.Push("main"); err != nil {
		t.Fatal(err)
	}

	command = exec.Command("git", "--git-dir", remoteDir, "rev-parse", "--verify", "refs/heads/main")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("remote branch was not pushed: %v\n%s", err, output)
	}
}

func TestRunAndOutput(t *testing.T) {
	repo := openTestRepo(t)

	if err := repo.run("status", "--porcelain"); err != nil {
		t.Fatal(err)
	}
	if err := repo.run("rev-parse", "--verify", "missing-ref"); err == nil {
		t.Fatal("run() returned no error for a missing ref")
	}

	got, err := repo.output("rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	if got != repo.Dir {
		t.Fatalf("output(show-toplevel) = %q, want %q", got, repo.Dir)
	}
}

func openTestRepo(t *testing.T) *Repo {
	t.Helper()

	repo, err := Open(setupTestRepo(t))
	if err != nil {
		t.Fatal(err)
	}
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
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("initial\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, repoDir, "add", "README.md")
	runTestGit(t, repoDir, "commit", "--quiet", "-m", "initial")
	return repoDir
}

func runTestGit(t *testing.T, repoDir string, args ...string) {
	t.Helper()

	command := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
