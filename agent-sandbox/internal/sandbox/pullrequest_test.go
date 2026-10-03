package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-sandbox/internal/git"
)

func TestRequireCommittedPRWorktree(t *testing.T) {
	for _, tc := range []struct {
		name  string
		dirty bool
	}{
		{name: "committed branch"},
		{name: "agent left changes", dirty: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoDir := setupDeleteTestRepo(t)
			worktree := addDeleteTestWorktree(t, repoDir, "feature")
			if tc.dirty {
				if err := os.WriteFile(filepath.Join(worktree.Path, "change.txt"), []byte("pending\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := requireCommittedWorktree(&git.Repo{Dir: worktree.Path})
			if tc.dirty && (err == nil || !strings.Contains(err.Error(), "uncommitted changes")) {
				t.Fatalf("expected uncommitted changes error, got %v", err)
			}
			if !tc.dirty && err != nil {
				t.Fatal(err)
			}
		})
	}
}
