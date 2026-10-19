package clierror

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			require.ErrorAs(t, wrapped, &diagnostic)
			assert.Equal(t, tt.want, tt.err.Error())
			if diagnostic.Cause != nil {
				assert.ErrorIs(t, wrapped, cause)
			}
		})
	}
}
