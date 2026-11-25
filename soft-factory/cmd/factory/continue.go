package main

import (
	"github.com/spf13/cobra"

	"soft-factory/internal/executionlog"
)

func newContinueCmd(run func(workflowOptions) error) *cobra.Command {
	var images []string
	cmd := &cobra.Command{
		Use:   executionlog.CommandContinue,
		Short: "Continue work in the agentSandbox.resume.branch in software-factory.json",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := optionsFrom(cmd)
			if err != nil {
				return err
			}
			if err := validateImagePaths(images); err != nil {
				return err
			}
			opts.Continue = true
			opts.Images = images
			return run(opts)
		},
	}
	addConfigFlag(cmd)
	addImageFlag(cmd, &images)
	return cmd
}
