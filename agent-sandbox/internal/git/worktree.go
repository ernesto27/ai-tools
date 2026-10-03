package git

import (
	"fmt"
)

// RemoveWorktree detaches the worktree at dir and deletes its directory. Git
// refuses a worktree with uncommitted changes unless force is set; the caller
// asks the same question first, so that refusal is a backstop rather than the
// message a user sees.
func (r *Repo) RemoveWorktree(dir string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	// The path goes after --, so a directory whose name begins with a dash is
	// still read as a path and not as an option.
	args = append(args, "--", dir)

	if err := r.run(args...); err != nil {
		return fmt.Errorf("git worktree remove: %w", err)
	}
	return nil
}

// PruneWorktrees forgets the worktrees whose directories are no longer on disk.
// It is the way back from a worktree deleted by hand, where the administrative
// record outlives the directory and RemoveWorktree has nothing left to remove.
func (r *Repo) PruneWorktrees() error {
	if err := r.run("worktree", "prune"); err != nil {
		return fmt.Errorf("git worktree prune: %w", err)
	}
	return nil
}
