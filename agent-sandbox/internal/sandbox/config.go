package sandbox

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

// Config holds the filesystem locations owned by the sandbox.
type Config struct {
	StateFile    string
	WorktreesDir string
}

// configPaths computes the sandbox's configuration paths without changing the
// filesystem. Read-only commands can therefore inspect an empty installation
// without creating directories that no run has needed yet.
func configPaths() (Config, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return Config{}, err
	}

	dir := filepath.Join(configDir, "agent-sandbox")
	worktreesDir := filepath.Join(dir, "worktrees")
	return Config{
		StateFile:    filepath.Join(dir, "worktrees.jsonl"),
		WorktreesDir: worktreesDir,
	}, nil
}

// repoSlug gives each repository its own stable worktree directory. A branch
// name alone is not enough because common names such as "fix-tests" can exist
// in unrelated repositories. The readable basename is paired with a short
// hash of the resolved path, so repositories with the same name still differ.
func repoSlug(repoDir string) string {
	resolved := resolvePath(repoDir)
	sum := sha256.Sum256([]byte(resolved))
	return filepath.Base(resolved) + "-" + hex.EncodeToString(sum[:6])
}

// resolvePath resolves the symlinks in a path so that two spellings of the same
// directory compare equal. A path that is no longer there keeps what it says.
func resolvePath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return resolved
}
