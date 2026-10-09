package sandbox

import (
	"context"
	"fmt"
	"io"
	"strings"

	"agent-sandbox/internal/git"
	"agent-sandbox/internal/github"
)

// appendPRChecklist preserves the original body as an exact prefix.
func appendPRChecklist(existing, checklist string) string {
	if existing == "" {
		return checklist
	}
	normalized := strings.ReplaceAll(existing, "\r\n", "\n")
	separator := "\n\n"
	if strings.HasSuffix(normalized, "\n\n") {
		separator = ""
	} else if strings.HasSuffix(normalized, "\n") {
		separator = "\n"
	}
	return existing + separator + checklist
}

func publishResumedPR(ctx context.Context, opts Options, record worktreeRecord,
	repo *git.Repo, client *github.Client, out io.Writer) error {
	worktree := repo.At(record.Path)
	branch, err := worktree.CurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("checking resume branch before pushing: %w", err)
	}
	if branch != opts.Branch {
		return fmt.Errorf("resume worktree must remain on branch %s, found %q", opts.Branch, branch)
	}
	if opts.prURL == "" {
		return fmt.Errorf("missing existing PR identity")
	}
	content, err := consumePRContent(ctx, worktree)
	if err != nil {
		return fmt.Errorf("reading resume PR checklist before pushing: %w", err)
	}
	if content.HasChanges == nil {
		return fmt.Errorf("resume PR artifact must include a boolean has_changes decision")
	}
	if err := requireCommittedWorktree(worktree); err != nil {
		return err
	}
	if err := worktree.PushHead(ctx); err != nil {
		return fmt.Errorf("pushing resumed PR branch: %w", err)
	}
	if !*content.HasChanges {
		fmt.Fprintf(out, "No net session changes. PR description unchanged: %s\n", opts.prURL)
		return nil
	}

	// Read after the push to retain description edits made during the session.
	existing, err := client.ReadOpenBody(ctx, opts.prURL, opts.Branch, record.BaseBranch)
	if err != nil {
		return fmt.Errorf("reading PR description after successful push: %w", err)
	}
	body := appendPRChecklist(existing, content.Body)
	if err := client.UpdateBody(ctx, opts.prURL, body); err != nil {
		return fmt.Errorf("updating PR description after successful push: %w", err)
	}
	fmt.Fprintf(out, "PR description updated: %s\n", opts.prURL)
	return nil
}
