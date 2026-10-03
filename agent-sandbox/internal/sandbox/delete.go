package sandbox

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"agent-sandbox/internal/git"
)

func WorktreeDelete(branch string, force bool, out io.Writer) error {
	found, err := sandboxWorktrees()
	if err != nil {
		return err
	}

	worktree, ok := findWorktree(found.worktrees, branch)
	if !ok {
		return fmt.Errorf("no sandbox worktree on branch %s; run worktree-list to see the ones there are", branch)
	}

	executionDir, err := os.Getwd()
	if err != nil {
		return err
	}

	inside, err := isInside(executionDir, worktree.Path)
	if err != nil {
		return err
	}
	if inside {
		return fmt.Errorf("cannot delete %s from inside it; run this from the repository instead", worktree.Path)
	}

	if err := deleteWorktree(found.repo, worktree, force, out); err != nil {
		return err
	}
	return forgetWorktrees(found.repo.Dir, []string{worktree.Path})
}

// WorktreeDeleteAll removes every sandbox worktree and the branches they hold.
// The removal is always forced, so the confirmation is the only thing standing
// between the question and the uncommitted work it discards.
func WorktreeDeleteAll(assumeYes bool, in io.Reader, out io.Writer) error {
	found, err := sandboxWorktrees()
	if err != nil {
		return err
	}
	if len(found.worktrees) == 0 {
		fmt.Fprintln(out, "No sandbox worktrees to delete.")
		return nil
	}

	executionDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// The whole command is refused when it would delete the directory it is
	// running in, rather than prompting and then deleting all but one. --yes
	// waives the question, not this.
	for _, worktree := range found.worktrees {
		inside, err := isInside(executionDir, worktree.Path)
		if err != nil {
			return err
		}
		if inside {
			return fmt.Errorf("cannot delete %s from inside it; run this from the repository instead", worktree.Path)
		}
	}

	printWorktrees(out, found.worktrees)

	if !assumeYes {
		noun := "worktrees and their branches"
		if len(found.worktrees) == 1 {
			noun = "worktree and its branch"
		}

		question := fmt.Sprintf("Delete %d %s? [y/N]: ", len(found.worktrees), noun)
		confirmed, err := confirm(in, out, question)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintln(out, "Aborted; nothing was deleted.")
			return nil
		}
	}

	// One worktree that will not go is no reason to leave the rest, so the
	// failures are collected and reported together at the end. Only the ones
	// that went are forgotten; a failure keeps its record.
	var (
		deleted  []string
		failures []error
	)
	for _, worktree := range found.worktrees {
		if err := deleteWorktree(found.repo, worktree, true, out); err != nil {
			failures = append(failures, err)
			continue
		}
		deleted = append(deleted, worktree.Path)
	}

	if err := forgetWorktrees(found.repo.Dir, deleted); err != nil {
		failures = append(failures, err)
	}
	return errors.Join(failures...)
}

// deleteWorktree removes one worktree and deletes the branch it held.
func deleteWorktree(repo *git.Repo, worktree worktreeRecord, force bool, out io.Writer) error {
	if err := removeWorktree(repo, worktree.Path, force, out); err != nil {
		return err
	}

	if err := repo.DeleteBranch(worktree.Branch); err != nil {
		return err
	}
	fmt.Fprintf(out, "Deleted branch %s\n", worktree.Branch)
	return nil
}

// removeWorktree detaches one worktree from the repository: by removing its
// directory, or, when the directory is already gone, by pruning the record git
// kept of it.
func removeWorktree(repo *git.Repo, path string, force bool, out io.Writer) error {
	// A directory deleted by hand leaves the administrative record behind, and
	// there is nothing for git worktree remove to remove. Pruning is the way
	// back from that, and it is the only case where the directory is not what
	// gets deleted.
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := repo.PruneWorktrees(); err != nil {
			return err
		}
		fmt.Fprintf(out, "Pruned the record of %s, whose directory was already gone\n", path)
		return nil
	}

	// Uncommitted work is the other thing no reflog brings back. The check comes
	// before the removal so that a refusal costs nothing, and it says --force
	// where git's own refusal would arrive wrapped in an exit status.
	if !force {
		clean, err := repo.At(path).IsClean()
		if err != nil {
			return err
		}
		if !clean {
			return fmt.Errorf("%s has uncommitted changes; pass --force to delete it anyway", path)
		}
	}

	if err := repo.RemoveWorktree(path, force); err != nil {
		return err
	}
	fmt.Fprintf(out, "Removed worktree %s\n", path)
	return nil
}

// isInside reports whether dir is the directory at root or somewhere under it.
// Both are resolved first, because the recorded worktree paths may not have
// their symlinks resolved and the current directory may not either.
func isInside(dir, root string) (bool, error) {
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return false, err
	}

	// A root that is not there at all — the worktree whose directory was
	// deleted by hand — contains nothing, least of all the directory this
	// command is running in.
	root, err = filepath.EvalSymlinks(root)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	relative, err := filepath.Rel(root, dir)
	if err != nil {
		// The two are on different volumes, so one cannot be inside the other.
		return false, nil
	}

	// Rel answers "." for the directory itself and a path starting with ".."
	// for anything outside it; everything else is somewhere underneath.
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false, nil
	}
	return true, nil
}
