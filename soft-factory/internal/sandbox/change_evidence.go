package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
)

// changeEvidence gathers Git facts on the host. A non-publishing agent-sandbox
// container mounts the worktree without its Git administration directory.
type changeSnapshot struct {
	Text     string
	Worktree string
}

func changeEvidence(ctx context.Context) (changeSnapshot, error) {
	settings, err := config.LoadSandbox()
	if err != nil {
		return changeSnapshot{}, fmt.Errorf("load configuration for change evidence: %w", err)
	}
	branch, err := settings.Branch("resume")
	if err != nil {
		return changeSnapshot{}, err
	}
	worktree, err := executionlog.WorktreeForBranch(ctx, branch)
	if err != nil {
		return changeSnapshot{}, err
	}
	base, baseNote := comparisonBase(ctx, worktree, branch)
	status, err := gitOutput(ctx, worktree, "status", "--short", "--untracked-files=all")
	if err != nil {
		return changeSnapshot{}, err
	}
	patch, err := gitOutput(ctx, worktree, "diff", "--no-ext-diff", "--no-renames", base, "--")
	if err != nil {
		return changeSnapshot{}, err
	}
	summary, err := gitOutput(ctx, worktree, "diff", "--summary", "--find-renames", base, "--")
	if err != nil {
		return changeSnapshot{}, err
	}
	commits, err := gitOutput(ctx, worktree, "log", "--oneline", base+"..HEAD")
	if err != nil {
		return changeSnapshot{}, err
	}
	untracked, err := gitOutput(ctx, worktree, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return changeSnapshot{}, err
	}
	snapshot := changeSnapshot{Worktree: worktree}
	var evidence strings.Builder
	fmt.Fprintf(&evidence, "Branch: %s\nComparison: %s\n", branch, baseNote)
	fmt.Fprintf(&evidence, "Git status (one entry per file):\n%s\n", emptyLabel(status))
	fmt.Fprintf(&evidence, "Commits since base:\n%s\n", emptyLabel(commits))
	fmt.Fprintf(&evidence, "File operation summary (including renames):\n%s\n", emptyLabel(summary))
	evidence.WriteString("Untracked paths (inspect their contents in the worktree):\n")
	if len(untracked) == 0 {
		evidence.WriteString("(none)\n")
	} else {
		for _, name := range strings.Split(string(untracked), "\x00") {
			if name != "" {
				fmt.Fprintf(&evidence, "%s\n", strconv.Quote(name))
			}
		}
	}
	fmt.Fprintf(&evidence, "Tracked file patch against base:\n%s\n", emptyLabel(patch))
	snapshot.Text = evidence.String()
	return snapshot, nil
}

func comparisonBase(ctx context.Context, worktree, branch string) (string, string) {
	var defaultBranch string
	if out, err := gitOutput(ctx, worktree, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		defaultBranch = strings.TrimSpace(string(out))
	}
	if defaultBranch == "" {
		for _, name := range []string{"main", "master"} {
			if _, err := gitOutput(ctx, worktree, "rev-parse", "--verify", "--quiet", "refs/heads/"+name); err == nil {
				defaultBranch = name
				break
			}
		}
	}
	if defaultBranch == branch || strings.TrimPrefix(defaultBranch, "origin/") == branch {
		if out, err := gitOutput(ctx, worktree, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); err == nil {
			defaultBranch = strings.TrimSpace(string(out))
		} else {
			defaultBranch = ""
		}
	}
	if defaultBranch != "" {
		if out, err := gitOutput(ctx, worktree, "merge-base", "HEAD", defaultBranch); err == nil {
			base := strings.TrimSpace(string(out))
			return base, fmt.Sprintf("merge base of HEAD and %s (%s)", defaultBranch, base)
		}
	}
	return "HEAD", "HEAD (no reliable branch base; committed changes before HEAD are not classified)"
}

func gitOutput(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func emptyLabel(data []byte) string {
	if len(data) == 0 {
		return "(none)"
	}
	return string(data)
}
