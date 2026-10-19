package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/clierror"
	"github.com/ernesto27/ai-tools/agent-sandbox/internal/sandbox"
)

func TestReportError(t *testing.T) {
	for _, tt := range []struct {
		name    string
		err     error
		command string
		status  int
		want    []string
		absent  []string
	}{
		{"unknown", errors.New("server: original\nsecond line"), "run", 1,
			[]string{"\nError: The operation failed.\n", "Category: General", "server: original\nsecond line"}, []string{"Next steps:"}},
		{"wrapped classified", fmt.Errorf("preparing: %w", &clierror.Error{Category: clierror.Worktree, Summary: "Worktree unavailable.", Cause: errors.New("original"), Details: []clierror.Detail{{Label: "Branch", Value: "feature"}}, Hints: []string{"Run agent-sandbox worktree-list."}}), "run", 1,
			[]string{"Error: Worktree unavailable.", "Category: Worktree", "Branch:   feature", "preparing: original", "Next steps:"}, nil},
		{"duplicate", &clierror.Error{Category: clierror.Worktree, Summary: "Worktree unavailable."}, "run", 1,
			[]string{"Error: Worktree unavailable."}, []string{"Cause:"}},
		{"usage", sandbox.NewUsageError(errors.New("--agent is required")), "run", 2,
			[]string{"Category: Usage", "--agent is required", "Usage:"}, nil},
		{"empty usage", sandbox.UsageError{}, "run", 2,
			[]string{"Usage:"}, []string{"Error:", "Category:"}},
		{"cancelled", fmt.Errorf("waiting: %w", context.Canceled), "run", 130,
			[]string{"Error: Execution interrupted.", "waiting: context canceled"}, []string{"container was stopped"}},
		{"legacy tui", errors.New("original"), "run-tui", 1,
			[]string{"error: original\n"}, []string{"Category:"}},
		{"legacy resume tui", sandbox.NewUsageError(errors.New("missing branch")), "resume-tui", 2,
			[]string{"error: missing branch\n", "Usage:"}, []string{"Category:"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := &cobra.Command{Use: tt.command}
			status := reportError(cmd, &out, tt.err, 80, false)
			require.Equal(t, tt.status, status)
			for _, want := range tt.want {
				assert.Contains(t, out.String(), want)
			}
			for _, absent := range tt.absent {
				assert.NotContains(t, out.String(), absent)
			}
		})
	}
}

func TestErrorLayout(t *testing.T) {
	path := "/a/very/long/path/that/must/stay/intact"
	for _, width := range []int{20, 80} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			var out bytes.Buffer
			renderError(&out, &clierror.Error{
				Category: clierror.Worktree,
				Summary:  "The résumé worktree directory already exists.",
				Details:  []clierror.Detail{{Label: "Path", Value: path}},
				Hints:    []string{"Inspect the registered worktrees before deciding how to proceed."},
			}, clierror.General, width, true)
			require.Contains(t, out.String(), "\x1b[")
			require.Contains(t, out.String(), path)
			for _, line := range strings.Split(ansi.Strip(out.String()), "\n") {
				if !strings.Contains(line, path) {
					assert.LessOrEqual(t, ansi.StringWidth(line), width, "line: %q", line)
				}
			}
		})
	}
	var captured bytes.Buffer
	t.Setenv("NO_COLOR", "")
	_, color := errorTerminal(&captured)
	assert.False(t, color, "captured output must not receive ANSI color")
}

func TestConfigurationDiagnostics(t *testing.T) {
	for _, content := range []string{
		`{"run":{"apiKey":"test-secret-never-display","push":"invalid"}}`,
		`{"run":{"apiKey":"test-secret-never-display","typo":true}}`,
		`{"run":{"apiKey":"test-secret-never-display"},`,
	} {
		t.Run(content, func(t *testing.T) {
			t.Chdir(t.TempDir())
			require.NoError(t, os.WriteFile(localConfigFile, []byte(content), 0o600))
			root, _ := newRootCmd()
			root.SetArgs([]string{"run", "-a", "codex", "-q", "inspect"})
			cmd, err := root.ExecuteC()
			require.Error(t, err, "invalid configuration accepted")
			var out bytes.Buffer
			status := reportError(cmd, &out, err, 80, false)
			require.Equal(t, 2, status)
			assert.Contains(t, out.String(), "Category: Configuration")
			assert.Contains(t, out.String(), localConfigFile)
			assert.NotContains(t, out.String(), "test-secret-never-display")
		})
	}
}
