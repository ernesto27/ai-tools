package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newRootCmd builds the CLI command tree. run executes the selected workflow;
// tests pass a fake to check routing without starting agent-sandbox.
func newRootCmd(run func(workflowOptions) error) *cobra.Command {
	root := &cobra.Command{
		Use:           "software-factory",
		Short:         "Implement a task, review it, classify risk, and explain the final changes.",
		Version:       version,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := optionsFrom(cmd, true)
			if err != nil {
				return err
			}
			return run(opts)
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().StringP("config", "c", "config.json", "Path to the factory configuration")
	root.PersistentFlags().String("jira", "", "Jira Cloud issue URL to use instead of run.file-prompt")

	root.AddCommand(&cobra.Command{
		Use:   "review",
		Short: "Run code review, security review, and risk classification on existing changes.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := optionsFrom(cmd, false)
			if err != nil {
				return err
			}
			return run(opts)
		},
	})
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newUpdateCmd())
	return root
}

func optionsFrom(cmd *cobra.Command, implement bool) (workflowOptions, error) {
	flags := cmd.Flags()
	configPath, err := flags.GetString("config")
	if err != nil {
		return workflowOptions{}, err
	}
	issueURL, err := flags.GetString("jira")
	if err != nil {
		return workflowOptions{}, err
	}
	if flags.Changed("jira") && strings.TrimSpace(issueURL) == "" {
		return workflowOptions{}, fmt.Errorf("--jira requires a non-empty issue URL")
	}
	return workflowOptions{ConfigPath: configPath, IssueURL: issueURL, Implement: implement}, nil
}
