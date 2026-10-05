package main

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"agent-sandbox/internal/sandbox"
)

// commandState carries the agent's exit status out of Cobra's deferred RunE
// callbacks. Both run and resume update the same named state,
// which main reads only after ExecuteContextC returns.
type commandState struct {
	status int
}

const (
	flagAgent         = "agent"
	flagModel         = "model"
	flagBaseImage     = "base-image"
	flagPush          = "push"
	flagPR            = "pr"
	flagHostNetwork   = "hn"
	flagBranch        = "branch"
	flagQuery         = "query"
	flagFilePrompt    = "file-prompt"
	flagCommitMessage = "commit-message"
	flagImage         = "image"
)

func newRootCmd() (*cobra.Command, *commandState) {
	state := &commandState{}

	cmd := &cobra.Command{
		Use:   "agent-sandbox <command>",
		Short: "Run a coding agent in a container, on a git worktree of its own",
		Long: "Run a coding agent inside the agent-sandbox container, on a git worktree of\n" +
			"its own, so it never touches the current working copy. Use run and resume\n" +
			"for the terminal view, or run-old and resume-old for plain output. Run\n" +
			"creates a worktree; resume continues one already recorded.\n\n" +
			"Authentication comes from the agent's configuration directory on the host,\n" +
			"or from api-key in agent-sandbox.json for Codex or Claude.",
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

	cmd.AddCommand(newRunCmd(state), newResumeCmd(state), newRunCmdOld(state), newResumeCmdOld(state), newWorktreeListCmd(), newWorktreeDeleteCmd(), newWorktreeDeleteAllCmd(), newWorktreeEditorOpenCmd())
	return cmd, state
}

// newRunCmdOld creates a fresh sandbox worktree. The root only dispatches verbs,
// so a session cannot start accidentally through the old implicit syntax.
func newRunCmdOld(state *commandState) *cobra.Command {
	var flags runFlags

	cmd := &cobra.Command{
		Use:   "run-old [-b <branch-name>] -a <agent> [flags] (-q <query> | -f <prompt-file>)",
		Short: "Run a coding agent with the previous plain terminal output",
		Long: "Run a coding agent in a new sandbox worktree with plain output. Supply the\n" +
			"branch with -b or --branch; otherwise one is generated. Supply the\n" +
			"instruction with -q or --query, or -f or --file-prompt. Defaults\n" +
			"may be set in ./agent-sandbox.json.",
		Example: "  agent-sandbox run-old -a codex -q \"fix the login redirect loop\"\n" +
			"  agent-sandbox run-old -b fix-go-tests -a codex -i golang:1.26-alpine -q \"run go test ./...\"\n" +
			"  agent-sandbox run-old -a codex -f prompt.md",
		Args: configRunArgs("run", &flags),
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := sandbox.NewOptions(flags.Options)
			if err != nil {
				return err
			}

			state.status, err = sandbox.Run(cmd.Context(), opts, sandbox.Runtime{Output: cmd.OutOrStdout()})
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
}

// bind gives both commands the same agent execution flags and completions, so
// a new run and a resumed run cannot silently grow different container or
// publish behavior.
func (f *runFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&f.AgentName, flagAgent, "a", "", "agent to run ("+strings.Join(sandbox.AgentNames(), "|")+")")
	cmd.Flags().StringVarP(&f.Model, flagModel, "m", "", "model to use (default: the agent's own)")
	cmd.Flags().StringVarP(&f.BaseImage, flagBaseImage, "i", "", "Alpine base image for the agent sandbox (for example golang:1.26-alpine)")
	cmd.Flags().StringVarP(&f.Prompt, flagQuery, "q", "", "instruction for the agent")
	cmd.Flags().BoolVarP(&f.Push, flagPush, "p", false, "commit the agent's work and push the branch")
	cmd.Flags().BoolVar(&f.PR, flagPR, false, "commit, push to origin, and create or reuse a GitHub pull request")
	cmd.Flags().StringVarP(&f.CommitMessage, flagCommitMessage, "c", "", "commit message (default: resolved prompt)")
	cmd.Flags().StringVarP(&f.FilePrompt, flagFilePrompt, "f", "", "path to a file containing the agent prompt")
	cmd.Flags().StringArrayVar(&f.Images, flagImage, nil, "image to attach to the initial Codex prompt (repeatable)")
	cmd.Flags().BoolVar(&f.HostNetwork, flagHostNetwork, false, "access to host services from container")

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
	filePrompt, err := cmd.Flags().GetString(flagFilePrompt)
	if err != nil {
		return sandbox.NewUsageError(err)
	}
	query, err := cmd.Flags().GetString(flagQuery)
	if err != nil {
		return sandbox.NewUsageError(err)
	}

	if query == "" && filePrompt == "" {
		return sandbox.UsageError{}
	}
	if query != "" && filePrompt != "" {
		return sandbox.NewUsageError(errors.New("--query and --file-prompt cannot be used together"))
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
