package sandbox

import (
	"errors"
	"fmt"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/docker"
)

// Exit statuses the command ends with. Anything else is the agent's own status.
const (
	// ExitFailure is something that went wrong before the agent ran.
	ExitFailure = 1
	// ExitUsage is a malformed or unusable command line.
	ExitUsage = 2
	// ExitTimeout is an agent container stopped at its configured deadline.
	ExitTimeout = 124
	// ExitInterrupted is a run cut short by Ctrl-C, reported the way a shell
	// reports a process killed by SIGINT.
	ExitInterrupted = 130
)

// UsageError is a malformed command line. The caller reports it with the usage
// message and exits with status 2. A zero UsageError carries no explanation:
// the usage message says everything there is to say.
type UsageError struct{ err error }

func (e UsageError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e UsageError) Unwrap() error { return e.err }

func usageErrorf(format string, args ...any) error {
	return UsageError{fmt.Errorf(format, args...)}
}

func NewUsageError(err error) error {
	return UsageError{err}
}

type StatusError struct {
	Status int
	err    error
}

func (e StatusError) Error() string { return e.err.Error() }
func (e StatusError) Unwrap() error { return e.err }

// IsTimeout distinguishes the sandbox deadline from an agent's own exit code.
func IsTimeout(err error) bool {
	return errors.Is(err, docker.ErrRunTimeout)
}
