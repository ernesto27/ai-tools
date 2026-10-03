package main

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	installscript "soft-factory"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Install the latest software-factory release.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			process := exec.CommandContext(cmd.Context(), "bash", "-c", installscript.Script)
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
