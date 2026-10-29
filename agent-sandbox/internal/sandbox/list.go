package sandbox

import (
	"fmt"
	"io"
	"path/filepath"
	"slices"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/git"
)

// WorktreeList prints branch and path in reverse registration order, newest first.
// Stale records remain visible so users can find and clean them up with the
// worktree management commands.
func WorktreeList(dir string, out io.Writer) error {
	found, err := sandboxWorktrees(dir)
	if err != nil {
		return err
	}

	slices.Reverse(found.worktrees)
	printWorktrees(out, found.worktrees)
	return nil
}

// WorktreeBranches returns registered branch names for the repository containing dir.
// Branches are in reverse registration order, newest first.
func WorktreeBranches(dir string) ([]string, error) {
	found, err := sandboxWorktrees(dir)
	if err != nil {
		return nil, err
	}

	var branches []string
	for _, worktree := range found.worktrees {
		branches = append(branches, worktree.Branch)
	}
	slices.Reverse(branches)
	return branches, nil
}

// repoWorktrees is a repository and the worktrees the sandbox created in it,
// kept together because deleting one needs the repository it belongs to.
type repoWorktrees struct {
	repo      *git.Repo
	worktrees []worktreeRecord
}

// sandboxWorktrees finds registered worktrees for the repository containing dir.
// Only the ones the sandbox created are there to find: a worktree the user made
// by hand was never written down, so no verb here can reach it.
func sandboxWorktrees(dir string) (repoWorktrees, error) {
	absoluteDir, err := filepath.Abs(dir)
	if err != nil {
		return repoWorktrees{}, err
	}

	repo, err := git.Open(absoluteDir)
	if err != nil {
		return repoWorktrees{}, err
	}

	records, err := recordedWorktrees(repo.Dir)
	if err != nil {
		return repoWorktrees{}, err
	}
	return repoWorktrees{repo: repo, worktrees: records}, nil
}

// printWorktrees writes the branch and path of each worktree in aligned
// columns. worktree-delete-all shows the same table before it asks.
func printWorktrees(out io.Writer, worktrees []worktreeRecord) {
	width := 0
	for _, worktree := range worktrees {
		if length := len(worktree.Branch); length > width {
			width = length
		}
	}
	for _, worktree := range worktrees {
		fmt.Fprintf(out, "%-*s  %s\n", width, worktree.Branch, worktree.Path)
	}
}
