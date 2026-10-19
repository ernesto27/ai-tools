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
			if got := reportError(cmd, &out, tt.err, 80, false); got != tt.status {
				t.Fatalf("status = %d, want %d", got, tt.status)
			}
			for _, want := range tt.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q in %q", want, out.String())
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(out.String(), absent) {
					t.Errorf("unexpected %q in %q", absent, out.String())
				}
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
			if !strings.Contains(out.String(), "\x1b[") || !strings.Contains(out.String(), path) {
				t.Fatalf("missing color or intact path: %q", out.String())
			}
			for _, line := range strings.Split(ansi.Strip(out.String()), "\n") {
				if !strings.Contains(line, path) && ansi.StringWidth(line) > width {
					t.Errorf("line exceeds width %d: %q", width, line)
				}
			}
		})
	}
	var captured bytes.Buffer
	t.Setenv("NO_COLOR", "")
	if _, color := errorTerminal(&captured); color {
		t.Fatal("captured output must not receive ANSI color")
	}
}

func TestConfigurationDiagnostics(t *testing.T) {
	for _, content := range []string{
		`{"run":{"apiKey":"test-secret-never-display","push":"invalid"}}`,
		`{"run":{"apiKey":"test-secret-never-display","typo":true}}`,
		`{"run":{"apiKey":"test-secret-never-display"},`,
	} {
		t.Run(content, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile(localConfigFile, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			root, _ := newRootCmd()
			root.SetArgs([]string{"run", "-a", "codex", "-q", "inspect"})
			cmd, err := root.ExecuteC()
			if err == nil {
				t.Fatal("invalid configuration accepted")
			}
			var out bytes.Buffer
			if status := reportError(cmd, &out, err, 80, false); status != 2 {
				t.Fatalf("status = %d, want 2", status)
			}
			if !strings.Contains(out.String(), "Category: Configuration") ||
				!strings.Contains(out.String(), localConfigFile) ||
				strings.Contains(out.String(), "test-secret-never-display") {
				t.Fatalf("unsafe or incomplete diagnostic: %q", out.String())
			}
		})
	}
}
