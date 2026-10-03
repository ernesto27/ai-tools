package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreeLockSerializesResumeSessionsAndLeavesSiblingFile(t *testing.T) {
	worktree := worktreeRecord{
		Branch: "feature",
		Path:   filepath.Join(t.TempDir(), "feature"),
	}
	if err := os.Mkdir(worktree.Path, 0o755); err != nil {
		t.Fatal(err)
	}

	first, err := lockWorktree(worktree)
	if err != nil {
		t.Fatal(err)
	}

	second, err := lockWorktree(worktree)
	if second != nil {
		t.Fatal("second lock was acquired while the first session still holds it")
	}
	if err == nil || !strings.Contains(err.Error(), "already in use by another session") {
		t.Fatalf("second lock error = %v, want an in-use error", err)
	}

	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := lockWorktree(worktree)
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	if err := third.Close(); err != nil {
		t.Fatal(err)
	}

	path := worktree.Path + ".agent-sandbox.lock"
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		t.Fatalf("lock file = (%v, %v), want an existing regular sibling file", info, err)
	}
	if filepath.Dir(path) != filepath.Dir(worktree.Path) {
		t.Fatalf("lock path = %q, want a sibling of %q", path, worktree.Path)
	}
}
