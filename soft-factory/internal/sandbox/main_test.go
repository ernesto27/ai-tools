package sandbox

import (
	"os"
	"testing"
)

// TestMain isolates git invocations from repository variables inherited
// from the caller (for example, a git hook or worktree environment), so tests
// and the code under test operate on their temporary repositories only.
func TestMain(m *testing.M) {
	for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE"} {
		os.Unsetenv(name)
	}
	os.Exit(m.Run())
}
