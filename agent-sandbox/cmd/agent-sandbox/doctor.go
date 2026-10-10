package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/sandbox"
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
			var report strings.Builder
			report.WriteString("Dependency checks:\n\n")
			report.WriteString("Dependency  Status         Usage\n")
			report.WriteString("----------  -------------  -----\n")
			for _, dependency := range sandbox.Dependencies() {
				usage := "Required"
				if dependency.Optional != "" {
					usage = "Optional: " + dependency.Optional
				}
				status, color := "not installed", "\x1b[31m"
				if dependency.Installed {
					status, color = "installed", "\x1b[32m"
				}
				// Pad the visible text separately so ANSI escapes do not shift columns.
				padding := strings.Repeat(" ", len("not installed")-len(status))
				fmt.Fprintf(&report, "%-10s  %s%s\x1b[0m%s  %s\n", dependency.Name, color, status, padding, usage)
			}
			_, err := fmt.Fprint(cmd.OutOrStdout(), report.String())
			return err
		},
	}
}
