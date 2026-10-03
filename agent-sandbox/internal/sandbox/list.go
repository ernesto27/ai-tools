package sandbox

import (
	"fmt"
	"io"
	"os"

	"agent-sandbox/internal/git"
)

// WorktreeList prints the worktrees the sandbox created in the repository the
// command was run in, one per line: the branch and the path. It reads the state
// file and git, so it needs neither Docker nor the network.
func WorktreeList(out io.Writer) error {
	found, err := sandboxWorktrees()
	if err != nil {
		return err
	}

	printWorktrees(out, found.worktrees)
	return nil
}

// WorktreeBranches names the branches the sandbox worktrees hold, for the shell
// completion of the flags that take one.
func WorktreeBranches() ([]string, error) {
	found, err := sandboxWorktrees()
	if err != nil {
		return nil, err
	}

	var branches []string
	for _, worktree := range found.worktrees {
		branches = append(branches, worktree.Branch)
	}
	return branches, nil
}

// repoWorktrees is a repository and the worktrees the sandbox created in it,
// kept together because deleting one needs the repository it belongs to.
type repoWorktrees struct {
	repo      *git.Repo
	worktrees []worktreeRecord
}

// sandboxWorktrees are the worktrees of the repository the command was run in.
// Only the ones the sandbox created are there to find: a worktree the user made
// by hand was never written down, so no verb here can reach it.
func sandboxWorktrees() (repoWorktrees, error) {
	executionDir, err := os.Getwd()
	if err != nil {
		return repoWorktrees{}, err
	}

	repo, err := git.Open(executionDir)
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
