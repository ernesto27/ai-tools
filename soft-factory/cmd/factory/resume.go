package main

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
)

func newResumeCmd(run func(workflowOptions) error) *cobra.Command {
	return &cobra.Command{
		Use:   "resume <path>",
		Short: "Resume an interrupted workflow from its run log folder",
		Long:  "Resume an interrupted workflow using current software-factory.json agentSandbox.resume settings.\nThe local task comes from agentSandbox.run.filePrompt. Settings travel through CLI flags;\nno sandbox configuration JSON is generated. Section apiKey and reviewers are ignored.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := executionlog.RunPath(args[0])
			if err != nil {
				return err
			}
			opts, err := optionsFrom(cmd, true)
			if err != nil {
				return err
			}
			saved, err := executionlog.NewStageStatus(path, false)
			if err != nil {
				return err
			}
			defer saved.Close()
			lastStage, status, err := saved.Last()
			if err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			for index, stage := range config.Stages {
				if stage != lastStage {
					continue
				}
				opts.StartStage = stage
				if status == executionlog.StatusSucceeded {
					if index+1 == len(config.Stages) {
						return nil
					}
					opts.StartStage = config.Stages[index+1]
				}
				break
			}
			opts.Continue, opts.ResumePath = true, path
			return run(opts)
		},
	}
}
