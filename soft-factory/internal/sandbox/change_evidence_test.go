package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestChangeEvidenceIncludesBranchAndWorktreeChanges(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	worktree := filepath.Join(root, "feature")
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
		}
	}
	if err := os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	git(repo, "init", "-b", "main")
	git(repo, "config", "user.name", "Test")
	git(repo, "config", "user.email", "test@example.com")
	tracked := filepath.Join(repo, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("base\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git(repo, "add", "tracked.txt")
	git(repo, "commit", "-m", "initial")
	git(repo, "worktree", "add", "-b", "feature", worktree)
	tracked = filepath.Join(worktree, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("base\ncommitted\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git(worktree, "add", "tracked.txt")
	git(worktree, "commit", "-m", "feature change")
	if err := os.WriteFile(tracked, []byte("base\ncommitted\nstaged\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git(worktree, "add", "tracked.txt")
	if err := os.WriteFile(tracked, []byte("base\ncommitted\nstaged\nunstaged\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "new file.txt"), []byte("untracked\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "agent-sandbox.json"), []byte(`{"resume":{"branch":"feature"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)
	snapshot, err := changeEvidence(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Worktree != worktree {
		t.Fatalf("worktree = %q, want %q", snapshot.Worktree, worktree)
	}
	for _, want := range []string{"Comparison: merge base of HEAD and main", "+committed", "+staged", "+unstaged", `"new file.txt"`} {
		if !strings.Contains(snapshot.Text, want) {
			t.Errorf("evidence missing %q", want)
		}
	}
}
