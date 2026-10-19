package sandbox

import (
	"context"
	"fmt"
	"strings"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/clierror"
	"github.com/ernesto27/ai-tools/agent-sandbox/internal/git"
)

func missingWorktreeError(branch string) error {
	return &clierror.Error{
		Category: clierror.Worktree,
		Summary:  "No registered sandbox worktree matches this branch.",
		Details:  []clierror.Detail{{Label: "Branch", Value: branch}},
		Hints:    []string{"Run agent-sandbox worktree-list to see registered worktrees."},
		Cause:    fmt.Errorf("no sandbox worktree on branch %s; run worktree-list to see the ones there are", branch),
	}
}

func staleWorktreeError(record worktreeRecord) error {
	branch := "'" + strings.ReplaceAll(record.Branch, "'", "'\\''") + "'"
	return &clierror.Error{
		Category: clierror.Worktree,
		Summary:  "The registered worktree directory is missing.",
		Details:  []clierror.Detail{{Label: "Branch", Value: record.Branch}, {Label: "Path", Value: record.Path}},
		Hints:    []string{fmt.Sprintf("Run agent-sandbox worktree-delete -b %s to remove the stale record and its branch.", branch)},
		Cause:    fmt.Errorf("sandbox worktree %s at %s is stale; run worktree-delete -b %s to remove its record", record.Branch, record.Path, branch),
	}
}

func existingWorktreeError(ctx context.Context, repo *git.Repo, branch, path string) error {
	diagnostic := &clierror.Error{
		Category: clierror.Worktree,
		Summary:  "Cannot create a worktree because its target path already exists.",
		Details:  []clierror.Detail{{Label: "Branch", Value: branch}, {Label: "Path", Value: path}},
		Hints: []string{
			"Run agent-sandbox worktree-list and inspect the existing path before deciding how to proceed.",
			"For a separate task, choose a different --branch or omit --branch to generate a name.",
		},
		Cause: fmt.Errorf("worktree path already exists: %s", path),
	}
	// A directory alone is not authorization to resume. Confirm both recorded
	// ownership and Git's working-tree root before recommending reuse.
	recorded, err := recordedWorktreePath(repo.Dir, branch)
	if err != nil || recorded == "" || resolvePath(recorded) != resolvePath(path) {
		return diagnostic
	}
	worktree, err := git.Open(path)
	if err != nil || resolvePath(worktree.Dir) != resolvePath(path) {
		return diagnostic
	}
	_, common, err := worktree.WorktreeGitDirs()
	if err != nil {
		return diagnostic
	}
	_, expected, err := repo.WorktreeGitDirs()
	if err != nil || resolvePath(common) != resolvePath(expected) {
		return diagnostic
	}
	current, err := worktree.CurrentBranch(ctx)
	if err == nil && current == branch {
		diagnostic.Summary = "A registered sandbox worktree already exists for this branch."
		diagnostic.Hints[0] = "Continue it with agent-sandbox resume, using the branch above, an agent, and your instructions."
	}
	return diagnostic
}
