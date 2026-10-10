package main

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	agentsandbox "github.com/ernesto27/ai-tools/agent-sandbox"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Install the latest agent-sandbox release",
		Long: "Run the embedded installer to download and verify the latest Linux x86_64\n" +
			"release. Installs to ~/.local/bin, or INSTALL_DIR when set. Requires Bash,\n" +
			"curl or wget, and sha256sum. Works from any directory and ignores configuration.",
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			process := exec.CommandContext(cmd.Context(), "bash", "-c", agentsandbox.InstallScript)
			process.Stdin = cmd.InOrStdin()
			process.Stdout = cmd.OutOrStdout()
			process.Stderr = cmd.ErrOrStderr()

			if err := process.Run(); err != nil {
				return fmt.Errorf("run installer: %w", err)
			}
			return nil
		},
	}
}
