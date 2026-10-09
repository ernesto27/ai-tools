package main

import (
	"github.com/spf13/cobra"

	"soft-factory/internal/executionlog"
)

func newContinueCmd(run func(workflowOptions) error) *cobra.Command {
	return &cobra.Command{
		Use:   executionlog.CommandContinue,
		Short: "Continue work in the resume.branch configured in agent-sandbox.json",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := optionsFrom(cmd, true)
			if err != nil {
				return err
			}
			opts.Continue = true
			return run(opts)
		},
	}
}
