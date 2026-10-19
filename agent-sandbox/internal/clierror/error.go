// Package clierror carries diagnostics without owning terminal presentation.
package clierror

import "fmt"

type Category string

const (
	General       Category = "General"
	Usage         Category = "Usage"
	Configuration Category = "Configuration"
	Dependency    Category = "Dependency"
	Worktree      Category = "Worktree"
)

type Detail struct {
	Label string
	Value string
}

type Error struct {
	Category Category
	Summary  string
	Details  []Detail
	Hints    []string
	Cause    error
}

// Error preserves the original diagnostic for callers that do not render fields.
func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return e.Summary
}

func (e *Error) Unwrap() error { return e.Cause }

func MissingDependency(name, purpose string, cause error) error {
	return &Error{
		Category: Dependency,
		Summary:  fmt.Sprintf("%s is required to %s.", name, purpose),
		Details:  []Detail{{Label: "Executable", Value: name}},
		Hints:    []string{fmt.Sprintf("Install %s and make sure it is available in PATH.", name)},
		Cause:    cause,
	}
}
