package sandbox

import (
	"fmt"
	"os/exec"
)

// OpenWorktree finds a sandbox-created worktree in the current repository and
// asks the host's VS Code command to open it.
func OpenWorktree(branch string) error {
	found, err := sandboxWorktrees()
	if err != nil {
		return err
	}

	worktree, exists := findWorktree(found.worktrees, branch)
	if !exists {
		return fmt.Errorf("no sandbox worktree on branch %s; run worktree-list to see the ones there are", branch)
	}

	return exec.Command("code", worktree.Path).Run()
}
