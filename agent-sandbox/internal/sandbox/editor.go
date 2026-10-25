package sandbox

import (
	"errors"
	"os/exec"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/clierror"
)

// OpenWorktree finds a sandbox-created worktree in the current repository and
// asks the host's VS Code command to open it.
func OpenWorktree(branch string) error {
	found, err := sandboxWorktrees(".")
	if err != nil {
		return err
	}

	worktree, exists := findWorktree(found.worktrees, branch)
	if !exists {
		return missingWorktreeError(branch)
	}

	err = exec.Command("code", worktree.Path).Run()
	if errors.Is(err, exec.ErrNotFound) {
		return clierror.MissingDependency("code", "open the worktree in VS Code", err)
	}
	return err
}
