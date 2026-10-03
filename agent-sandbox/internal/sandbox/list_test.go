package sandbox

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWorktreeListAndBranchesUseCurrentRepositoryRecords(t *testing.T) {
	repoDir := setupListTestRepo(t)
	records := []worktreeRecord{
		{Repo: repoDir, Branch: "short", Path: "/worktrees/short"},
		{Repo: repoDir, Branch: "much-longer", Path: "/worktrees/much-longer"},
		{Repo: t.TempDir(), Branch: "other-repo", Path: "/worktrees/other"},
	}
	for _, record := range records {
		if err := recordWorktree(record); err != nil {
			t.Fatal(err)
		}
	}

	found, err := sandboxWorktrees()
	if err != nil {
		t.Fatal(err)
	}
	if found.repo.Dir != repoDir {
		t.Fatalf("sandboxWorktrees() repository = %q, want %q", found.repo.Dir, repoDir)
	}
	wantWorktrees := records[:2]
	if !reflect.DeepEqual(found.worktrees, wantWorktrees) {
		t.Fatalf("sandboxWorktrees() worktrees = %#v, want %#v", found.worktrees, wantWorktrees)
	}

	var out bytes.Buffer
	if err := WorktreeList(&out); err != nil {
		t.Fatal(err)
	}
	wantOutput := "short        /worktrees/short\n" +
		"much-longer  /worktrees/much-longer\n"
	if got := out.String(); got != wantOutput {
		t.Fatalf("WorktreeList() output = %q, want %q", got, wantOutput)
	}

	branches, err := WorktreeBranches()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"short", "much-longer"}; !reflect.DeepEqual(branches, want) {
		t.Fatalf("WorktreeBranches() = %q, want %q", branches, want)
	}
}

func TestPrintWorktrees(t *testing.T) {
	worktrees := []worktreeRecord{
		{Branch: "a", Path: "/worktrees/a"},
		{Branch: "long", Path: "/worktrees/long"},
	}

	var out bytes.Buffer
	printWorktrees(&out, worktrees)

	want := "a     /worktrees/a\nlong  /worktrees/long\n"
	if got := out.String(); got != want {
		t.Fatalf("printWorktrees() output = %q, want %q", got, want)
	}
}

func TestSandboxWorktreesRequiresGitRepository(t *testing.T) {
	dir := t.TempDir()
	changeToDir(t, dir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, err := sandboxWorktrees()
	if err == nil {
		t.Fatal("sandboxWorktrees() returned no error outside a Git repository")
	}
}

func setupListTestRepo(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()
	command := exec.Command("git", "-C", repoDir, "init", "--quiet")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}

	changeToDir(t, repoDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return repoDir
}

func changeToDir(t *testing.T, dir string) {
	t.Helper()

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restoring working directory: %v", err)
		}
	})
}

func TestWorktreeListEmpty(t *testing.T) {
	setupListTestRepo(t)

	var out bytes.Buffer
	if err := WorktreeList(&out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "" {
		t.Fatalf("WorktreeList() output = %q, want empty output", got)
	}
}

func TestWorktreeBranchesEmpty(t *testing.T) {
	setupListTestRepo(t)

	branches, err := WorktreeBranches()
	if err != nil {
		t.Fatal(err)
	}
	if branches != nil {
		t.Fatalf("WorktreeBranches() = %q, want nil", branches)
	}
}

func TestSandboxWorktreesResolvesRepositoryPath(t *testing.T) {
	repoDir := setupListTestRepo(t)
	link := filepath.Join(t.TempDir(), "repo")
	if err := os.Symlink(repoDir, link); err != nil {
		t.Fatal(err)
	}
	changeToDir(t, link)

	found, err := sandboxWorktrees()
	if err != nil {
		t.Fatal(err)
	}
	if found.repo.Dir != repoDir {
		t.Fatalf("sandboxWorktrees() repository = %q, want %q", found.repo.Dir, repoDir)
	}
}
