package sandbox

import (
	"agent-sandbox/internal/utils"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// resolveCommitMessage prefers an explicit message, then the agent's artifact.
// An unusable artifact falls back to the prompt so publication can continue.
func resolveCommitMessage(opts Options, worktreeDir string, out io.Writer) string {
	if opts.CommitMessage != "" {
		return opts.CommitMessage
	}

	path := filepath.Join(worktreeDir, utils.CommitMessageFile)
	data, err := os.ReadFile(path)
	if err == nil {
		message := strings.TrimSpace(string(data))
		if message != "" && !strings.ContainsAny(message, "\r\n\u0085\u2028\u2029") {
			return message
		}
		err = fmt.Errorf("%s must contain one nonempty line", utils.CommitMessageFile)
	}

	fmt.Fprintf(out, "Warning: cannot use generated commit message: %v. Using original prompt.\n", err)
	return opts.Prompt
}

// removeCommitMessage runs only after the commit step succeeds, preserving the
// artifact for a retry when committing fails. Cleanup failure should not stop
// publication of changes that have already been committed.
func removeCommitMessage(worktreeDir string, out io.Writer) {
	path := filepath.Join(worktreeDir, utils.CommitMessageFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(out, "Warning: cannot remove %s: %v.\n", utils.CommitMessageFile, err)
	}
}
