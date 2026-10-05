package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"agent-sandbox/internal/sandbox"
)

func newRunTUICmd(state *commandState) *cobra.Command {
	return newLiveCommand(state, "run")
}

func newResumeTUICmd(state *commandState) *cobra.Command {
	return newLiveCommand(state, "resume")
}

// Both live commands own the terminal in the same way. Only configuration,
// branch selection, and the worker's sandbox entry point depend on the mode.
func newLiveCommand(state *commandState, mode string) *cobra.Command {
	var flags runFlags
	use := "run-tui [-b <branch-name>] -a <agent> [flags] (-q <query> | -f <prompt-file>)"
	short := "Run a real agent with live output and details in a terminal view"
	lead := "Run a coding agent in a new sandbox worktree with a two-panel terminal view.\n"
	branchHelp := "worktree branch name (default: generated)"
	if mode == "resume" {
		use = "resume-tui -b <branch-name> -a <agent> [flags] (-q <query> | -f <prompt-file>)"
		short = "Resume a sandbox worktree with live output and details in a terminal view"
		lead = "Run a coding agent in an existing, recorded sandbox worktree with a terminal view.\n" +
			"The branch is shown by worktree-list; committed and uncommitted work stays in place.\n"
		branchHelp = "recorded worktree branch name to resume"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long: lead +
			fmt.Sprintf("Uses the same flags and agent-sandbox.json %s defaults as %s.\n", mode, mode) +
			"Requires terminal input and output. Tab changes focus; arrows and PgUp/PgDn\n" +
			"scroll; q or Ctrl+C cancels and waits for cleanup. The view closes when\n" +
			"execution and any requested publication finish.",
		Args: configRunArgs(mode, &flags),
		RunE: func(cmd *cobra.Command, _ []string) (runErr error) {
			if mode == "resume" && flags.Branch == "" {
				return sandbox.NewUsageError(errors.New("-b <branch-name> is required"))
			}
			log, err := newLiveLog(mode)
			if err != nil {
				return err
			}
			// Bubble Tea prints recovered panic traces directly to os.Stderr.
			// Capture those diagnostics while leaving terminal input/output alone.
			stderr := os.Stderr
			os.Stderr = log
			defer func() {
				if recovered := recover(); recovered != nil {
					fmt.Fprintf(log, "\nPANIC: %v\n%s\n", recovered, debug.Stack())
					runErr = fmt.Errorf("%s panic: %v", mode, recovered)
				}
				if runErr != nil {
					fmt.Fprintf(log, "\nERROR: %v\n", runErr)
					runErr = fmt.Errorf("%w (log: %s)", runErr, log.Name())
				} else {
					fmt.Fprintf(log, "\n%s finished; exit status: %d\n", mode, state.status)
				}
				os.Stderr = stderr
				log.Close()
			}()
			input, inOK := cmd.InOrStdin().(*os.File)
			output, outOK := cmd.OutOrStdout().(*os.File)
			if !inOK || !outOK || !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
				return sandbox.NewUsageError(fmt.Errorf("%s-tui requires an interactive terminal for stdin and stdout; use %s for plain output", mode, mode))
			}
			opts, err := sandbox.NewOptions(flags.Options)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			fmt.Fprintf(log, "Agent: %s\nModel: %s\nBranch: %s\n\n", opts.Agent.Name(), opts.Model, opts.Branch)
			bridge := &liveBridge{log: log, mode: mode}
			job := newLiveJob(ctx, opts, bridge, mode)
			defer func() {
				cancel()
				<-job.done
			}()
			model := newLiveModel(opts, bridge, job, cancel)
			// Parent signals cancel execution, but the screen stays alive while
			// the worker stops its container and completes cleanup.
			_, uiErr := tea.NewProgram(model,
				tea.WithContext(context.WithoutCancel(cmd.Context())),
				tea.WithoutSignalHandler(),
				tea.WithInput(input), tea.WithOutput(output),
			).Run()
			cancel()
			<-job.done
			state.status = job.status
			resultErr := uiErr
			if resultErr == nil {
				resultErr = job.err
			}
			// Print after Bubble Tea restores the terminal, so the result stays
			// visible in the shell instead of disappearing with the panels.
			printLiveResult(output, stderr, job.status, resultErr)
			if logErr := bridge.logError(); logErr != nil {
				fmt.Fprintf(stderr, "Warning: transcript log is incomplete (%s): %v\n", log.Name(), logErr)
			}
			if resultErr == nil && job.status != 0 {
				fmt.Fprintf(stderr, "Log: %s\n", log.Name())
			}
			return resultErr
		},
	}
	cmd.Flags().StringVarP(&flags.Branch, flagBranch, "b", "", branchHelp)
	flags.bind(cmd)
	if mode == "resume" {
		completeFlag(cmd, flagBranch, func(string) ([]string, error) {
			return sandbox.WorktreeBranches()
		})
	}
	return cmd
}

func printLiveResult(out, errOut io.Writer, status int, err error) {
	result := "completed"
	if err != nil {
		status = sandbox.ExitFailure
		var usageErr sandbox.UsageError
		var statusErr sandbox.StatusError
		switch {
		case errors.As(err, &usageErr):
			status = sandbox.ExitUsage
		case errors.Is(err, context.Canceled):
			status = sandbox.ExitInterrupted
		case errors.As(err, &statusErr):
			status = statusErr.Status
		}
	}
	if err != nil || status != 0 {
		result = "failed"
		out = errOut
		if errors.Is(err, context.Canceled) {
			result = "cancelled"
		}
	}
	fmt.Fprintf(out, "Execution %s (exit status %d).\n", result, status)
}
