package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"agent-sandbox/internal/sandbox"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "List host dependencies and their installation status",
		Long: "Check PATH for git, docker, gh (optional for PRs), and code (optional for\n" +
			"worktree-editor). Does not run tools or check service health. Works from\n" +
			"any directory, ignores configuration, and succeeds even if tools are missing.\n" +
			"Installation statuses always use green/red ANSI colors.",
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			for _, dependency := range sandbox.Dependencies() {
				name := dependency.Name
				if dependency.Optional != "" {
					name += " (optional: " + dependency.Optional + ")"
				}
				status, color := "not installed", "\x1b[31m"
				if dependency.Installed {
					status, color = "installed", "\x1b[32m"
				}
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s%s\x1b[0m\n", name, color, status); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
