package sandbox

import (
	"context"
	"fmt"
	"io"
	"strings"

	"agent-sandbox/internal/git"
	"agent-sandbox/internal/github"
)

// preparePullRequest runs before image work or worktree creation. The recorded
// base is deliberately required on resume: a current or default branch would
// silently change the target of a previously started sandbox.
func preparePullRequest(ctx context.Context, opts Options, repo *git.Repo, base string) (*github.Client, error) {
	if !opts.PR {
		return nil, nil
	}
	if base == "" {
		return nil, fmt.Errorf("--pr requires a named, recorded base branch; detached starts and legacy worktree records without base_branch cannot publish a PR")
	}
	if opts.Branch == base {
		return nil, fmt.Errorf("pull request head and base must differ: %s", base)
	}
	if err := git.CheckBranchName(opts.Branch); err != nil {
		return nil, StatusError{Status: ExitUsage, err: err}
	}
	if err := git.CheckBranchName(base); err != nil {
		return nil, fmt.Errorf("invalid recorded PR base: %w", err)
	}
	fetch, push, err := repo.OriginURLs(ctx)
	if err != nil {
		return nil, fmt.Errorf("checking PR origin: %w", err)
	}
	client, err := github.New(repo.Dir, fetch)
	if err != nil {
		return nil, err
	}
	if !client.MatchesRemote(push) {
		return nil, fmt.Errorf("--pr requires origin fetch and push URLs to identify the same GitHub repository")
	}
	if err := client.Validate(ctx); err != nil {
		return nil, err
	}
	if _, err := repo.RemoteBranch(ctx, base); err != nil {
		return nil, err
	}
	return client, nil
}

// publishResult only publishes a completed agent session. Both publishing
// modes require the agent to commit first; PR mode additionally creates a PR.
func publishResult(ctx context.Context, opts Options, record worktreeRecord, repo *git.Repo, githubClient *github.Client, status int, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if status != 0 && (opts.PR || opts.Push) {
		fmt.Fprintf(out, "Agent exited with status %d. Skipping publication; changes left in %s\n", status, record.Path)
		return nil
	}
	if !opts.PR {
		return publish(ctx, opts, record.Path, repo, out)
	}
	return publishPullRequest(ctx, opts, record, repo, githubClient, out)
}

func publishPullRequest(ctx context.Context, opts Options, record worktreeRecord, repo *git.Repo, githubClient *github.Client, out io.Writer) error {
	worktree := repo.At(record.Path)
	branch, err := worktree.CurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("checking PR head before publishing; branch has not been pushed: %w", err)
	}
	if branch != opts.Branch {
		return fmt.Errorf("PR worktree must remain on branch %s, found %q; branch has not been pushed", opts.Branch, branch)
	}
	content, err := consumePRContent(ctx, worktree)
	if err != nil {
		return fmt.Errorf("reading agent PR text; branch has not been pushed: %w", err)
	}
	if err := requireCommittedWorktree(worktree); err != nil {
		return err
	}
	// Execution fetched this snapshot before the agent started so its summary
	// and the host comparison use the same base even if origin advances.
	base := opts.prBase
	if base == "" {
		return fmt.Errorf("missing PR base snapshot; branch has not been pushed")
	}
	_, changed, err := worktree.ReviewComparison(ctx, base)
	if err != nil {
		return fmt.Errorf("comparing PR changes; branch has not been pushed: %w", err)
	}
	if !changed {
		fmt.Fprintf(out, "No changes to review against %s. Skipping pull request creation.\n", record.BaseBranch)
		return nil
	}
	if err := worktree.PushHead(ctx); err != nil {
		return fmt.Errorf("pushing PR branch; push did not complete successfully: %w", err)
	}
	pr, err := githubClient.FindOpen(ctx, opts.Branch, record.BaseBranch)
	if err != nil {
		return fmt.Errorf("looking up PR after successful push: %w", err)
	}
	if pr != nil {
		printExistingPR(out, pr)
		return nil
	}
	prURL, createErr := githubClient.Create(ctx, opts.Branch, record.BaseBranch, content.Title, content.Body, opts.Reviewers)
	if createErr == nil {
		fmt.Fprintf(out, "Pull request ready for review: %s\n", prURL)
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// A concurrent publisher or a lost HTTP response may have created the PR.
	// Rechecking the exact repository/head/base avoids reporting a false failure.
	pr, lookupErr := githubClient.FindOpen(ctx, opts.Branch, record.BaseBranch)
	if lookupErr != nil {
		return fmt.Errorf("creating PR after successful push: %w; recovery lookup failed: %v", createErr, lookupErr)
	}
	if pr != nil {
		if len(opts.Reviewers) > 0 {
			missing, err := githubClient.MissingReviewers(ctx, pr.URL, opts.Reviewers)
			if err != nil {
				return fmt.Errorf("creating PR after successful push: %w; checking reviewers on %s: %w", createErr, pr.URL, err)
			}
			if len(missing) > 0 {
				return fmt.Errorf("creating PR after successful push: %w; PR %s exists but review requests are missing for: %s", createErr, pr.URL, strings.Join(missing, ", "))
			}
		}
		printExistingPR(out, pr)
		return nil
	}
	return fmt.Errorf("creating PR after successful push: %w", createErr)
}

// requireCommittedWorktree prevents publication of a partial agent result.
// A clean branch may already hold commits from an earlier session, so it does
// not require this particular invocation to have created another commit.
func requireCommittedWorktree(worktree *git.Repo) error {
	clean, err := worktree.IsClean()
	if err != nil {
		return fmt.Errorf("checking PR worktree after agent commit: %w", err)
	}
	if !clean {
		return fmt.Errorf("agent left uncommitted changes in %s; branch has not been pushed", worktree.Dir)
	}
	return nil
}

func printExistingPR(out io.Writer, pr *github.PullRequest) {
	state := ""
	if pr.IsDraft {
		state = " (draft)"
	}
	fmt.Fprintf(out, "Existing pull request%s: %s\n", state, strings.TrimSpace(pr.URL))
}
