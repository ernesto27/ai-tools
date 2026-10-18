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
		Long:          "Configure this project with software-factory.json, including agentSandbox settings. Sandbox settings are passed as command-line flags.",
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
	root.PersistentFlags().String("jira", "", "Jira Cloud issue URL to use instead of agentSandbox.run.filePrompt")

	root.AddCommand(&cobra.Command{
		Use:   "review",
		Short: "Run code review, security review, and risk classification on existing changes.",
		Long:  "Review existing changes using agentSandbox.resume settings in software-factory.json. The original local task remains agentSandbox.run.filePrompt; sandbox settings are passed as command-line flags.",
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
	root.AddCommand(newContinueCmd(run))
	root.AddCommand(newResumeCmd(run))
	return root
}

func optionsFrom(cmd *cobra.Command, implement bool) (workflowOptions, error) {
	flags := cmd.Flags()
	issueURL, err := flags.GetString("jira")
	if err != nil {
		return workflowOptions{}, err
	}
	if flags.Changed("jira") && strings.TrimSpace(issueURL) == "" {
		return workflowOptions{}, fmt.Errorf("--jira requires a non-empty issue URL")
	}
	return workflowOptions{IssueURL: issueURL, Implement: implement}, nil
}
