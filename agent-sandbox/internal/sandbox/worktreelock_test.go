package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorktreeLockSerializesResumeSessionsAndLeavesSiblingFile(t *testing.T) {
	worktree := worktreeRecord{
		Branch: "feature",
		Path:   filepath.Join(t.TempDir(), "feature"),
	}
	require.NoError(t, os.Mkdir(worktree.Path, 0o755))

	first, err := lockWorktree(worktree)
	require.NoError(t, err)

	second, err := lockWorktree(worktree)
	require.Nil(t, second)
	require.ErrorContains(t, err, "already in use by another session")

	require.NoError(t, first.Close())
	third, err := lockWorktree(worktree)
	require.NoError(t, err)
	require.NoError(t, third.Close())

	path := worktree.Path + ".agent-sandbox.lock"
	{
		info, err := os.Stat(path)
		require.NoError(t, err)
		require.False(t, info.IsDir(), "lock file = (%v, %v), want an existing regular sibling file", info, err)
	}
	require.Equal(t, filepath.Dir(worktree.Path), filepath.Dir(path))
}
