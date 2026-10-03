// Package git wraps the git CLI calls the sandbox needs to set up a worktree
// for the agent and to publish its result.
package git

import (
	"agent-sandbox/internal/utils"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Repo is a git repository addressed by one of its working directories.
type Repo struct {
	Dir string

	Stdout io.Writer
	Stderr io.Writer
}

// Open returns the repository containing dir, resolved to its top level.
func Open(dir string) (*Repo, error) {
	repo := &Repo{Dir: dir, Stdout: os.Stdout, Stderr: os.Stderr}

	root, err := repo.output("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("%s is not inside a Git repository", dir)
	}

	repo.Dir = root
	return repo, nil
}

// At returns the same repository addressed through another working directory,
// such as a worktree created by AddWorktree.
func (r *Repo) At(dir string) *Repo {
	return &Repo{Dir: dir, Stdout: r.Stdout, Stderr: r.Stderr}
}

// CheckBranchName reports whether name is a valid branch name.
func CheckBranchName(name string) error {
	cmd := exec.Command("git", "check-ref-format", "--branch", name)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("invalid branch name: %s", name)
	}
	return nil
}

// BranchExists reports whether the branch is already present in the repository.
func (r *Repo) BranchExists(branch string) bool {
	return r.run("show-ref", "--verify", "--quiet", "refs/heads/"+branch) == nil
}

// DeleteBranch deletes the branch whether or not its commits are merged
// anywhere else, which is what -D means. Git's -d would refuse a sandbox
// branch in the ordinary case, since such a branch is meant to be pushed and
// reviewed rather than merged locally.
func (r *Repo) DeleteBranch(branch string) error {
	if err := r.run("branch", "-D", "--", branch); err != nil {
		return fmt.Errorf("git branch -D: %w", err)
	}
	return nil
}

// IsClean reports whether the working directory has no changes at all, staged
// or not, tracked or not. Unlike HasStagedChanges it speaks for the whole
// worktree, which is what decides whether removing it would discard work.
func (r *Repo) IsClean() (bool, error) {
	out, err := r.output("status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return out == "", nil
}

// AddWorktree checks the branch out at dir, creating the branch when create is set.
func (r *Repo) AddWorktree(dir, branch string, create bool) error {
	args := []string{"worktree", "add"}
	if create {
		args = append(args, "-b", branch, dir)
	} else {
		args = append(args, dir, branch)
	}

	if err := r.run(args...); err != nil {
		return fmt.Errorf("git worktree add: %w", err)
	}
	return nil
}

// StageAll stages changes except the generated commit-message artifact.
func (r *Repo) StageAll() error {
	return r.stageAll(context.Background())
}

// stageAll shares the artifact exclusion between publishing paths while
// allowing callers with a context to cancel the Git command.
func (r *Repo) stageAll(ctx context.Context) error {
	return r.runContext(ctx, "add", "-A", "--", ".", ":(top,exclude)"+utils.CommitMessageFile)
}

// HasStagedChanges reports whether anything is staged for commit.
func (r *Repo) HasStagedChanges() bool {
	return r.run("diff", "--cached", "--quiet") != nil
}

// Commit commits the staged changes.
func (r *Repo) Commit(message string) error {
	if err := r.run("commit", "-m", message); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}

// Push pushes the branch and sets its upstream to origin.
func (r *Repo) Push(branch string) error {
	if err := r.run("push", "--set-upstream", "origin", branch); err != nil {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}

func (r *Repo) run(args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", r.Dir}, args...)...)
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr
	return cmd.Run()
}

// output runs git and returns its trimmed stdout.
func (r *Repo) output(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", r.Dir}, args...)...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
