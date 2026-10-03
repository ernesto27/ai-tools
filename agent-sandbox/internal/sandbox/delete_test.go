package sandbox

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
				if err := os.WriteFile(filepath.Join(worktree.Path, "change.txt"), []byte("change"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := recordWorktree(worktree); err != nil {
				t.Fatal(err)
			}

			var out bytes.Buffer
			err := WorktreeDelete(worktree.Branch, tt.force, &out)
			if tt.wantRemoved {
				if err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(worktree.Path); !os.IsNotExist(err) {
					t.Fatalf("worktree path still exists after deletion: %v", err)
				}
				if branchExists(t, repoDir, worktree.Branch) {
					t.Fatalf("branch %q still exists after deletion", worktree.Branch)
				}
				records, err := recordedWorktrees(repoDir)
				if err != nil {
					t.Fatal(err)
				}
				if len(records) != 0 {
					t.Fatalf("records after deletion = %#v, want none", records)
				}
				return
			}

			want := worktree.Path + " has uncommitted changes; pass --force to delete it anyway"
			if err == nil || err.Error() != want {
				t.Fatalf("WorktreeDelete() error = %v, want %q", err, want)
			}
			if _, err := os.Stat(worktree.Path); err != nil {
				t.Fatalf("worktree path was removed: %v", err)
			}
			if !branchExists(t, repoDir, worktree.Branch) {
				t.Fatalf("branch %q was deleted", worktree.Branch)
			}
		})
	}
}

func TestWorktreeDeleteRejectsUnrecordedBranch(t *testing.T) {
	setupDeleteTestRepo(t)

	err := WorktreeDelete("missing", false, &bytes.Buffer{})
	want := "no sandbox worktree on branch missing; run worktree-list to see the ones there are"
	if err == nil || err.Error() != want {
		t.Fatalf("WorktreeDelete() error = %v, want %q", err, want)
	}
}

func TestWorktreeDeletePrunesMissingDirectory(t *testing.T) {
	repoDir := setupDeleteTestRepo(t)
	worktree := addDeleteTestWorktree(t, repoDir, "feature")
	if err := recordWorktree(worktree); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(worktree.Path); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := WorktreeDelete(worktree.Branch, false, &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "Pruned the record of "+worktree.Path) {
		t.Fatalf("WorktreeDelete() output = %q, want prune notice", got)
	}
	if branchExists(t, repoDir, worktree.Branch) {
		t.Fatalf("branch %q still exists after deletion", worktree.Branch)
	}
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
				if err := recordWorktree(worktree); err != nil {
					t.Fatal(err)
				}
			}

			var out bytes.Buffer
			err := WorktreeDeleteAll(tt.assumeYes, strings.NewReader(tt.input), &out)
			if err != nil {
				t.Fatal(err)
			}
			if !tt.wantDelete {
				if !strings.Contains(out.String(), "Aborted; nothing was deleted.") {
					t.Fatalf("WorktreeDeleteAll() output = %q, want abort notice", out.String())
				}
				if !branchExists(t, repoDir, first.Branch) || !branchExists(t, repoDir, second.Branch) {
					t.Fatal("WorktreeDeleteAll() deleted a branch after a declined confirmation")
				}
				return
			}

			for _, worktree := range []worktreeRecord{first, second} {
				if _, err := os.Stat(worktree.Path); !os.IsNotExist(err) {
					t.Fatalf("worktree path still exists after deletion: %v", err)
				}
				if branchExists(t, repoDir, worktree.Branch) {
					t.Fatalf("branch %q still exists after deletion", worktree.Branch)
				}
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
			if got != tt.want || found != tt.found {
				t.Fatalf("findWorktree(%q) = (%#v, %t), want (%#v, %t)", tt.branch, got, found, tt.want, tt.found)
			}
		})
	}
}

func TestIsInside(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
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
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("isInside(%q, %q) = %t, want %t", tt.dir, tt.root, got, tt.want)
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}

	if got := resolvePath(link); got != dir {
		t.Fatalf("resolvePath(%q) = %q, want %q", link, got, dir)
	}

	missing := filepath.Join(dir, "missing", "path")
	if got, want := resolvePath(missing), filepath.Clean(missing); got != want {
		t.Fatalf("resolvePath(%q) = %q, want %q", missing, got, want)
	}
}

func setupDeleteTestRepo(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "--quiet")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("initial\n"), 0o600); err != nil {
		t.Fatal(err)
	}
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
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
