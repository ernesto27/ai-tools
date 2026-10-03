// Command agent-sandbox runs a coding agent inside the agent-sandbox container,
// on a git worktree of its own, so it never touches the current working copy.
//
// Usage:
//
//	agent-sandbox run [-b <branch-name>] -a <codex|claude|opencode|pi> [-m <model>] [-i <image>] [-p] [-c <commit-message>] (-q <query> | -f <prompt-file>)
//	agent-sandbox resume -b <branch-name> -a <codex|claude|opencode|pi> (-q <query> | -f <prompt-file>)
//	agent-sandbox worktree-list
//	agent-sandbox worktree-delete -b <branch-name> [--force]
//	agent-sandbox worktree-delete-all [--yes]
//

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"agent-sandbox/internal/sandbox"
)

func main() {
	root, state := newRootCmd()

	// An interrupt cancels the context, which stops the container and waits for
	// it: the agent holds the worktree open, so it must not outlive this
	// process. A second interrupt kills this process outright. The worktree
	// verbs never look at the context — they only ever talk to git — so
	// installing the handler before knowing which command will run costs them
	// nothing.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd, err := root.ExecuteContextC(ctx)
	if err != nil {
		fail(cmd, err)
	}

	os.Exit(state.status)
}

func fail(cmd *cobra.Command, err error) {
	var usageErr sandbox.UsageError
	if errors.As(err, &usageErr) {
		if message := usageErr.Error(); message != "" {
			fmt.Fprintf(os.Stderr, "error: %s\n", message)
		}
		fmt.Fprint(os.Stderr, cmd.UsageString())
		os.Exit(sandbox.ExitUsage)
	}

	if errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "interrupted; the container was stopped")
		os.Exit(sandbox.ExitInterrupted)
	}

	fmt.Fprintf(os.Stderr, "error: %s\n", err)

	status := sandbox.ExitFailure
	var statusErr sandbox.StatusError
	if errors.As(err, &statusErr) {
		status = statusErr.Status
	}
	os.Exit(status)
}
