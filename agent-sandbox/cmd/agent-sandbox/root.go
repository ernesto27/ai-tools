package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/sandbox"
)

// commandState carries the agent's exit status out of Cobra's deferred RunE
// callbacks. Both run and resume update the same named state,
// which main reads only after ExecuteContextC returns.
type commandState struct {
	status int
}

const (
	flagAgent         = "agent"
	flagAPIKey        = "apiKey"
	flagReviewers     = "reviewers"
	flagModel         = "model"
	flagEffort        = "effort"
	flagBaseImage     = "baseImage"
	flagPush          = "push"
	flagPR            = "pr"
	flagHostNetwork   = "hn"
	flagBranch        = "branch"
	flagQuery         = "query"
	flagFilePrompt    = "filePrompt"
	flagCommitMessage = "commitMessage"
	flagImage         = "image"
	flagFile          = "file"
	flagFolder        = "folder"
	flagCPUs          = "cpus"
	flagMemory        = "memory"
	flagTimeout       = "timeout"
)

func newRootCmd() (*cobra.Command, *commandState) {
	state := &commandState{}

	cmd := &cobra.Command{
		Use:   "agent-sandbox <command>",
		Short: "Run a coding agent in a container, on a git worktree of its own",
		Long: "Run a coding agent inside the agent-sandbox container, on a git worktree of\n" +
			"its own, so it never touches the current working copy. Use run-tui and resume-tui\n" +
			"for the terminal view, or run and resume for plain output. Run\n" +
			"creates a worktree; resume continues one already recorded.\n\n" +
			"Authentication comes from the agent's configuration directory on the host,\n" +
			"or from --apiKey or apiKey in agent-sandbox.json for Codex or Claude.",
		Example: "  agent-sandbox run -a codex -q \"fix the login redirect loop\"\n" +
			"  agent-sandbox resume -b fix-login -a codex -q \"add a regression test\"",

		// An explicit validator makes a former root invocation a UsageError,
		// preserving the exit-status protocol instead of Cobra's unknown-command
		// error when positional text follows the binary name.
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},

		// fail is the only thing that prints an error or a synopsis, so that the
		// exit status and the message it goes with are decided in one place.
		SilenceUsage:  true,
		SilenceErrors: true,

		Version: buildVersion(),
	}

	// pflag reports a malformed flag through the error func of the command it
	// was parsing, or of the nearest parent that has one, so this covers the
	// subcommands as well as the run.
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return sandbox.NewUsageError(err)
	})

	cmd.AddCommand(
		newRunCmd(state),
		newResumeCmd(state),
		newRunTUICmd(state),
		newResumeTUICmd(state),
		newWorktreeListCmd(),
		newWorktreeDeleteCmd(),
		newWorktreeDeleteAllCmd(),
		newWorktreeEditorOpenCmd(),
		newModelCmd(),
		newVersionCheckCmd(),
		newCustomImageCmd(),
	)
	cmd.AddCommand(newDoctorCmd(), newUpdateCmd())
	return cmd, state
}

// newRunCmd creates a fresh sandbox worktree. The root only dispatches verbs,
// so a session cannot start accidentally through the old implicit syntax.
func newRunCmd(state *commandState) *cobra.Command {
	var flags runFlags

	cmd := &cobra.Command{
		Use:   "run [-b <branch-name>] -a <agent> [flags] (-q <query> | -f <prompt-file>...)",
		Short: "Run a coding agent with plain terminal output",
		Long: "Run a coding agent in a new sandbox worktree with plain output. Supply the\n" +
			"branch with -b or --branch; otherwise one is generated. Supply the\n" +
			"instruction with -q or --query, or -f or --filePrompt. Defaults\n" +
			"may be set in ./agent-sandbox.json.",
		Example: "  agent-sandbox run -a codex -q \"fix the login redirect loop\"\n" +
			"  agent-sandbox run -b fix-go-tests -a codex -i golang:1.26-alpine -q \"run go test ./...\"\n" +
			"  agent-sandbox run -a codex -f prompt.md",
		Args: configRunArgs("run", &flags),
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := sandbox.NewOptions(flags.Options)
			if err != nil {
				return err
			}

			state.status, err = sandbox.Run(cmd.Context(), opts, sandbox.Runtime{
				Output: cmd.OutOrStdout(), FormatStdout: agentOutputFormatter(opts.Agent.Name()),
			})
			return err
		},
	}

	cmd.Flags().StringVarP(&flags.Branch, flagBranch, "b", "", "worktree branch name (default: generated)")
	flags.bind(cmd)
	return cmd
}

// runFlags is the execution configuration shared by a fresh run and a resume.
// Their only different input is the meaning of the branch: run
// creates it when necessary, while resume finds it in the sandbox state file.
type runFlags struct {
	sandbox.Options
	cpus, memory, timeout string
}

// bind gives both commands the same agent execution flags and completions, so
// a new run and a resumed run cannot silently grow different container or
// publish behavior.
func (f *runFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&f.AgentName, flagAgent, "a", "", "agent to run ("+strings.Join(sandbox.AgentNames(), "|")+")")
	cmd.Flags().StringVar(&f.APIKey, flagAPIKey, "", "provider API `key` for Codex or Claude (overrides JSON; empty uses host authentication)")
	cmd.Flags().StringArrayVar(&f.Reviewers, flagReviewers, nil, "GitHub `username` to request on a new PR (repeatable; replaces JSON; empty alone clears; no effect on resume)")
	cmd.Flags().StringVarP(&f.Model, flagModel, "m", "", "model to use (default: the agent's own)")
	cmd.Flags().StringVar(&f.Effort, flagEffort, "", "agent effort (default: "+sandbox.DefaultEffort+"; supported values depend on the agent and model)")
	cmd.Flags().StringVarP(&f.BaseImage, flagBaseImage, "i", "", "Alpine base image for the agent sandbox (for example golang:1.26-alpine)")
	cmd.Flags().StringVarP(&f.Prompt, flagQuery, "q", "", "instruction for the agent")
	cmd.Flags().BoolVarP(&f.Push, flagPush, "p", false, "commit the agent's work and push the branch")
	cmd.Flags().BoolVar(&f.PR, flagPR, false, "commit, push to origin, and create or reuse a GitHub pull request")
	cmd.Flags().StringVarP(&f.CommitMessage, flagCommitMessage, "c", "", "commit message (default: resolved prompt)")
	cmd.Flags().StringArrayVarP(&f.FilePrompt, flagFilePrompt, "f", nil, "prompt file path (repeatable; contents joined in order)")
	cmd.Flags().StringArrayVar(&f.Images, flagImage, nil, "image to attach to the initial Codex prompt (repeatable)")
	cmd.Flags().StringArrayVar(&f.Files, flagFile, nil, "file to mount read-only for the agent (repeatable)")
	cmd.Flags().StringArrayVar(&f.Folders, flagFolder, nil, "folders to mount read-only for the agent")
	cmd.Flags().BoolVar(&f.HostNetwork, flagHostNetwork, false, "access to host services from container")
	cmd.Flags().StringVar(&f.cpus, flagCPUs, "0", "CPU limit in `cores` (for example 1.5, 0 clears)")
	cmd.Flags().StringVar(&f.memory, flagMemory, "0", "memory limit `size` (minimum 6MiB, 0 clears)")
	cmd.Flags().StringVar(&f.timeout, flagTimeout, "0", "container execution `duration` (for example 30m, 0 clears)")

	completeFlag(cmd, flagAgent, func(string) ([]string, error) {
		return sandbox.AgentNames(), nil
	})
}

// runArgs requires exactly one prompt source and rejects positional text.
// A file prompt removes shell quoting from longer instructions, but accepting
// it with a query would silently discard one when Options resolves the file.
func runArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return sandbox.NewUsageError(errors.New("positional prompts are not supported; use -q or --query"))
	}
	filePrompt, err := cmd.Flags().GetStringArray(flagFilePrompt)
	if err != nil {
		return sandbox.NewUsageError(err)
	}
	// pflag serializes a single empty array entry like an empty list.
	// Reject it explicitly so an empty CLI path cannot clear JSON defaults.
	if cmd.Flags().Changed(flagFilePrompt) && len(filePrompt) == 0 {
		return sandbox.NewUsageError(errors.New("--filePrompt path must not be empty"))
	}
	for i, path := range filePrompt {
		if path == "" {
			return sandbox.NewUsageError(fmt.Errorf("--filePrompt[%d] path must not be empty", i))
		}
	}
	query, err := cmd.Flags().GetString(flagQuery)
	if err != nil {
		return sandbox.NewUsageError(err)
	}

	if query == "" && len(filePrompt) == 0 {
		return sandbox.UsageError{}
	}
	if query != "" && len(filePrompt) > 0 {
		return sandbox.NewUsageError(errors.New("--query and --filePrompt cannot be used together"))
	}
	return nil
}

// usageArgs reports a wrong number of arguments as a malformed command line.
// Cobra hands the errors an argument validator produces straight back from
// Execute, with no hook of their own to pass them through, so the wrapping that
// SetFlagErrorFunc does for flags happens here for positional arguments.
func usageArgs(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := validate(cmd, args); err != nil {
			return sandbox.NewUsageError(err)
		}
		return nil
	}
}

// completeFlag offers the values of a flag to the shell. The completions are
// filtered to what the user has typed so far and never fall back to file names,
// because none of these flags names a file. A registration only fails when the
// same flag is registered twice, which is a mistake in this file rather than
// anything a run can do, so there is nothing to report at run time.
func completeFlag(cmd *cobra.Command, name string, values func(prefix string) ([]string, error)) {
	_ = cmd.RegisterFlagCompletionFunc(name, func(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
		candidates, err := values(prefix)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		var matching []string
		for _, candidate := range candidates {
			if strings.HasPrefix(candidate, prefix) {
				matching = append(matching, candidate)
			}
		}
		return matching, cobra.ShellCompDirectiveNoFileComp
	})
}
