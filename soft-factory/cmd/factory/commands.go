package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newRootCmd builds the CLI command tree. run executes the selected workflow;
// tests pass a fake to check routing without starting agent-sandbox.
func newRootCmd(run func(workflowOptions) error) *cobra.Command {
	var taskFile string
	root := &cobra.Command{
		Use:           "software-factory",
		Short:         "Implement a task, review it, classify risk, and explain the final changes.",
		Long:          "Implement a task, review it, classify risk, and explain the final changes.\n\nReads software-factory.json from the project root, or the file passed with --config,\nincluding agentSandbox settings. Relative paths in it stay relative to the project root.\nSettings use CLI flags; no sandbox configuration JSON is generated.\nSet agentSandbox.run.reviewers to request GitHub reviewers on new pull requests. Use host authentication.",
		Version:       version,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := optionsFrom(cmd)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("file") {
				if strings.TrimSpace(taskFile) == "" {
					return fmt.Errorf("--file requires a non-empty file path")
				}
				if opts.IssueURL != "" {
					return fmt.Errorf("--file and --jira are mutually exclusive")
				}
				opts.TaskFile = taskFile
			}
			return run(opts)
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.CompletionOptions.DisableDefaultCmd = true
	root.Flags().StringVar(&taskFile, "file", "", "Task prompt file to use instead of agentSandbox.run.filePrompt")
	root.PersistentFlags().String("jira", "", "Jira Cloud issue URL to use instead of agentSandbox.run.filePrompt")
	addConfigFlag(root)

	root.AddCommand(newDoctorCmd())
	root.AddCommand(newUpdateCmd())
	root.AddCommand(newVersionCheckCmd())
	root.AddCommand(newContinueCmd(run))
	root.AddCommand(newResumeCmd(run))
	root.AddCommand(newAuditCmd())
	root.AddCommand(newScheduledTaskCmd())
	return root
}

// addConfigFlag registers --config/-c on a command that loads the factory configuration.
func addConfigFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("config", "c", factoryConfigFile, "Factory configuration file to use instead of software-factory.json")
}

// configPathFrom returns the --config value, or an empty string when the flag was not set.
func configPathFrom(cmd *cobra.Command) (string, error) {
	flags := cmd.Flags()
	if !flags.Changed("config") {
		return "", nil
	}
	path, err := flags.GetString("config")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("--config requires a non-empty file path")
	}
	return path, nil
}

func optionsFrom(cmd *cobra.Command) (workflowOptions, error) {
	flags := cmd.Flags()
	issueURL, err := flags.GetString("jira")
	if err != nil {
		return workflowOptions{}, err
	}
	if flags.Changed("jira") && strings.TrimSpace(issueURL) == "" {
		return workflowOptions{}, fmt.Errorf("--jira requires a non-empty issue URL")
	}
	configPath, err := configPathFrom(cmd)
	if err != nil {
		return workflowOptions{}, err
	}
	return workflowOptions{IssueURL: issueURL, ConfigPath: configPath}, nil
}
