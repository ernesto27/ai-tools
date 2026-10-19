package clierror

import (
	"errors"
	"fmt"
	"testing"
)

func TestDiagnosticPreservesCause(t *testing.T) {
	cause := errors.New("original failure")
	for _, tt := range []struct {
		name string
		err  error
		want string
	}{
		{"diagnostic", &Error{Summary: "Readable summary", Cause: cause}, "original failure"},
		{"dependency", MissingDependency("git", "read the repository", cause), "original failure"},
		{"no cause", &Error{Summary: "Readable summary"}, "Readable summary"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := fmt.Errorf("operation: %w", tt.err)
			var diagnostic *Error
			if !errors.As(wrapped, &diagnostic) || tt.err.Error() != tt.want {
				t.Fatalf("diagnostic not preserved: %v", wrapped)
			}
			if diagnostic.Cause != nil && !errors.Is(wrapped, cause) {
				t.Fatal("original cause lost through wrapping")
			}
		})
	}
}
