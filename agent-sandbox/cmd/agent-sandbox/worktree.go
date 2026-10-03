package main

import (
	"errors"

	"github.com/spf13/cobra"

	"agent-sandbox/internal/sandbox"
)

// newResumeCmd runs another agent session in a sandbox worktree named by its
// branch. The run command keeps creating worktrees, so reuse remains an
// explicit operation and cannot happen by accident on a normal run.
func newResumeCmd(state *commandState) *cobra.Command {
	var (
		branchName string
		flags      runFlags
	)

	cmd := &cobra.Command{
		Use:   "resume -b <branch-name> -a <agent> [flags] (-q <query> | -f <prompt-file>)",
		Short: "Run a coding agent again in a sandbox worktree",
		Long: "Run a coding agent in a sandbox worktree recorded for this repository.\n" +
			"The branch name is shown by worktree-list; its committed and uncommitted\n" +
			"work stays in place for the new session.",
		Example: "  agent-sandbox resume -b fix-login -a codex -q \"add a regression test\"\n" +
			"  agent-sandbox resume -b fix-login -a claude --push -f next-task.md",
		Args: configRunArgs("resume", &branchName, &flags),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if branchName == "" {
				return sandbox.NewUsageError(errors.New("-b <branch-name> is required"))
			}
			opts, err := flags.options(branchName)
			if err != nil {
				return err
			}

			state.status, err = sandbox.Resume(cmd.Context(), opts, cmd.OutOrStdout())
			return err
		},
	}

	cmd.Flags().StringVarP(&branchName, "branch", "b", "", "recorded worktree branch name to resume")
	flags.bind(cmd)

	completeFlag(cmd, "branch", func(string) ([]string, error) {
		return sandbox.WorktreeBranches()
	})
	return cmd
}

// newWorktreeListCmd is the verb that shows what the sandbox has left behind:
// the worktrees it added to this repository, one per line.
func newWorktreeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "worktree-list",
		Short: "List the worktrees the sandbox added to this repository",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return sandbox.WorktreeList(cmd.OutOrStdout())
		},
	}
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

	cmd.Flags().StringVarP(&branch, "branch", "b", "", "branch whose worktree to delete")
	cmd.Flags().BoolVar(&force, "force", false, "delete even if the worktree has uncommitted changes")

	completeFlag(cmd, "branch", func(string) ([]string, error) {
		return sandbox.WorktreeBranches()
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

	cmd.Flags().StringVarP(&branch, "branch", "b", "", "branch whose worktree to open")

	return cmd
}
