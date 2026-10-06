package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// CurrentBranch returns an empty name for a detached HEAD. Ordinary sandbox
// runs still support detached starts, but there is no named PR base to record.
func (r *Repo) CurrentBranch(ctx context.Context) (string, error) {
	// Git's --short disambiguates a branch from a same-named tag by returning
	// heads/<name>. Strip the full branch prefix instead to keep the exact name.
	name, err := r.outputContext(ctx, "symbolic-ref", "--quiet", "HEAD")
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 && ctx.Err() == nil {
		return "", nil
	}
	return strings.TrimPrefix(strings.TrimSpace(name), "refs/heads/"), err
}

// OriginURLs returns effective fetch and push destinations, including Git URL
// rewrites. Publication checks both so a separate pushurl cannot send a branch
// to a different repository from the one where gh creates the PR.
func (r *Repo) OriginURLs(ctx context.Context) (string, string, error) {
	fetch, err := r.outputContext(ctx, "remote", "get-url", "origin")
	if err != nil {
		return "", "", err
	}
	push, err := r.outputContext(ctx, "remote", "get-url", "--push", "--all", "origin")
	if err != nil {
		return "", "", err
	}
	urls := strings.Split(strings.TrimSpace(push), "\n")
	if len(urls) != 1 {
		return "", "", fmt.Errorf("origin must have exactly one push destination for --pr")
	}
	return strings.TrimSpace(fetch), urls[0], nil
}

// RemoteBranch resolves the actual origin branch instead of trusting a stale
// remote-tracking ref, which can refer to a branch already deleted on GitHub.
func (r *Repo) RemoteBranch(ctx context.Context, branch string) (string, error) {
	ref := "refs/heads/" + branch
	output, err := r.outputContext(ctx, "ls-remote", "--exit-code", "--heads", "origin", ref)
	if err != nil {
		return "", fmt.Errorf("resolving base branch %s on origin: %w", branch, err)
	}
	fields := strings.Fields(output)
	if len(fields) != 2 || fields[1] != ref {
		return "", fmt.Errorf("base branch %s is unavailable on origin", branch)
	}
	return fields[0], nil
}

// FetchBase pins the comparison to one remote snapshot. Fetching the object
// by SHA avoids relying on FETCH_HEAD, which another worktree's fetch can change.
func (r *Repo) FetchBase(ctx context.Context, branch string) (string, error) {
	sha, err := r.RemoteBranch(ctx, branch)
	if err != nil {
		return "", err
	}
	if err := r.runContext(ctx, "fetch", "--no-tags", "origin", sha); err != nil {
		return "", err
	}
	return sha, nil
}

// CommitPending does not create an empty commit, but a clean worktree remains
// eligible for publication when its existing commits have not been pushed yet.
func (r *Repo) CommitPending(ctx context.Context, message string) error {
	if err := r.stageAll(ctx); err != nil {
		return err
	}
	_, err := r.outputContext(ctx, "diff", "--cached", "--quiet", "--exit-code")
	if err == nil {
		return nil
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || ctx.Err() != nil {
		return err
	}
	return r.runContext(ctx, "commit", "-m", message)
}

// PushHead selects origin explicitly so GitHub CLI never needs to choose
// between pushing to this repository and creating a fork.
func (r *Repo) PushHead(ctx context.Context) error {
	return r.runContext(ctx, "push", "-u", "origin", "HEAD")
}

// IsTracked checks both the index and HEAD because a staged deletion still
// belongs to the branch. Publication must not remove tracked source files.
func (r *Repo) IsTracked(ctx context.Context, path string) (bool, error) {
	output, err := r.outputContext(ctx, "ls-files", "-z", "--", path)
	if err != nil || output != "" {
		return output != "", err
	}
	output, err = r.outputContext(ctx, "ls-tree", "--name-only", "-z", "HEAD", "--", path)
	return output != "", err
}

// Comparison contains the complete PR input, with binary changes represented
// by Git's path/status and binary markers rather than decoded file contents.
type Comparison struct {
	Diff           string `json:"diff"`
	ChangedPaths   string `json:"changed_paths_nul_separated"`
	Statistics     string `json:"statistics"`
	CommitMessages string `json:"commit_messages"`
}

func (r *Repo) ReviewComparison(ctx context.Context, base string) (Comparison, bool, error) {
	head, err := r.outputContext(ctx, "rev-parse", "HEAD")
	if err != nil {
		return Comparison{}, false, err
	}
	head = strings.TrimSpace(head)
	mergeBase, err := r.outputContext(ctx, "merge-base", base, head)
	if err != nil {
		return Comparison{}, false, fmt.Errorf("finding PR merge base: %w", err)
	}
	mergeBase = strings.TrimSpace(mergeBase)
	commits, err := r.outputContext(ctx, "log", "--format=%B%x00", base+".."+head)
	if err != nil {
		return Comparison{}, false, err
	}
	paths, err := r.outputContext(ctx, "diff", "--no-ext-diff", "--no-textconv", "--name-status", "-z", mergeBase, head, "--")
	if err != nil {
		return Comparison{}, false, err
	}
	if paths == "" || commits == "" {
		return Comparison{}, false, nil
	}
	diff, err := r.outputContext(ctx, "diff", "--no-ext-diff", "--no-textconv", "--no-color", mergeBase, head, "--")
	if err != nil {
		return Comparison{}, false, err
	}
	stats, err := r.outputContext(ctx, "diff", "--no-ext-diff", "--no-textconv", "--stat", mergeBase, head, "--")
	if err != nil {
		return Comparison{}, false, err
	}
	return Comparison{Diff: diff, ChangedPaths: paths, Statistics: stats, CommitMessages: commits}, true, nil
}

func (r *Repo) runContext(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", r.Dir}, args...)...)
	cmd.Stdout, cmd.Stderr = r.Stdout, r.Stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("git %s: %w", args[0], err)
	}
	return nil
}

// outputContext preserves every byte of a diff and returns cancellation itself
// so the CLI continues to map an interrupted publication to exit status 130.
func (r *Repo) outputContext(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", r.Dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return string(output), nil
}
