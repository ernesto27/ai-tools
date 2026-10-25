package main

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/spf13/cobra"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/sandbox"
)

// newResumeCmd runs another agent session in a sandbox worktree named by its
// branch. The run command keeps creating worktrees, so reuse remains an
// explicit operation and cannot happen by accident on a normal run.
func newResumeCmd(state *commandState) *cobra.Command {
	var flags runFlags

	cmd := &cobra.Command{
		Use:   "resume -b <branch-name> -a <agent> [flags] (-q <query> | -f <prompt-file>)",
		Short: "Resume a sandbox worktree with plain terminal output",
		Long: "Run a coding agent with plain output in a worktree recorded for this repository.\n" +
			"The branch name is shown by worktree-list; its committed and uncommitted\n" +
			"work stays in place for the new session.",
		Example: "  agent-sandbox resume -b fix-login -a codex -q \"add a regression test\"\n" +
			"  agent-sandbox resume -b fix-login -a claude --push -f next-task.md",
		Args: configRunArgs("resume", &flags),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flags.Branch == "" {
				return sandbox.NewUsageError(errors.New("-b <branch-name> is required"))
			}
			opts, err := sandbox.NewOptions(flags.Options)
			if err != nil {
				return err
			}

			state.status, err = sandbox.Resume(cmd.Context(), opts, sandbox.Runtime{
				Output: cmd.OutOrStdout(), FormatStdout: agentOutputFormatter(opts.Agent.Name()),
			})
			return err
		},
	}

	cmd.Flags().StringVarP(&flags.Branch, flagBranch, "b", "", "recorded worktree branch name to resume")
	flags.bind(cmd)

	completeFlag(cmd, flagBranch, func(string) ([]string, error) {
		return sandbox.WorktreeBranches(".")
	})
	return cmd
}

// newWorktreeListCmd lists registered worktrees for the selected repository.
func newWorktreeListCmd() *cobra.Command {
	var repoDir string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "worktree-list [--repo <directory>] [--json]",
		Short: "List the worktrees the sandbox added to a repository",
		Long: "List sandbox worktrees for the repository containing the current directory\n" +
			"or --repo. Text output shows branch and path, including stale records.\n" +
			"With --json, return registered records as objects containing only branch.\n" +
			"No matches returns []. Worktree directories are not checked when listing.",
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !jsonOutput {
				return sandbox.WorktreeList(repoDir, cmd.OutOrStdout())
			}
			branches, err := sandbox.WorktreeBranches(repoDir)
			if err != nil {
				return err
			}
			return writeWorktreeBranchesJSON(cmd.OutOrStdout(), branches)
		},
	}
	cmd.Flags().StringVar(&repoDir, "repo", ".", "directory inside the repository to list (defaults to current directory)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output registered branch objects as a JSON array")
	return cmd
}

func writeWorktreeBranchesJSON(out io.Writer, branches []string) error {
	type branchRecord struct {
		Branch string `json:"branch"`
	}
	records := make([]branchRecord, 0, len(branches))
	for _, branch := range branches {
		records = append(records, branchRecord{Branch: branch})
	}
	return json.NewEncoder(out).Encode(records)
}

// newWorktreeDeleteCmd is the verb that cleans one of them up, freeing the
// branch name for another run.
func newWorktreeDeleteCmd() *cobra.Command {
	var (
		branch string
		force  bool
	)

	cmd := &cobra.Command{
		Use:   "worktree-delete -b <branch-name>",
		Short: "Remove a sandbox worktree and delete the branch with it",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if branch == "" {
				return sandbox.NewUsageError(errors.New("-b <branch-name> is required"))
			}
			return sandbox.WorktreeDelete(branch, force, cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringVarP(&branch, flagBranch, "b", "", "branch whose worktree to delete")
	cmd.Flags().BoolVar(&force, "force", false, "delete even if the worktree has uncommitted changes")

	completeFlag(cmd, flagBranch, func(string) ([]string, error) {
		return sandbox.WorktreeBranches(".")
	})

	return cmd
}

// newWorktreeDeleteAllCmd is the verb that cleans all of them up at once. It
// always deletes, uncommitted changes and all, so it asks first.
func newWorktreeDeleteAllCmd() *cobra.Command {
	var assumeYes bool

	cmd := &cobra.Command{
		Use:   "worktree-delete-all",
		Short: "Remove every sandbox worktree and delete their branches, uncommitted changes and all",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return sandbox.WorktreeDeleteAll(assumeYes, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}

	cmd.Flags().BoolVarP(&assumeYes, "yes", "y", false, "skip the confirmation prompt")

	return cmd
}

func newWorktreeEditorOpenCmd() *cobra.Command {
	var branch string

	cmd := &cobra.Command{
		Use:   "worktree-editor",
		Short: "Open vscode editor for specific worktree -b <branch-name>",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if branch == "" {
				return sandbox.NewUsageError(errors.New("-b <branch-name> is required"))
			}
			return sandbox.OpenWorktree(branch)
		},
	}

	cmd.Flags().StringVarP(&branch, flagBranch, "b", "", "branch whose worktree to open")

	return cmd
}
