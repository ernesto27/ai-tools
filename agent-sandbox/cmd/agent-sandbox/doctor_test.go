package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDoctorInstallationReport(t *testing.T) {
	for _, tt := range []struct {
		name       string
		installed  []string
		nonexec    string
		wantStatus []bool
	}{
		{"all installed", []string{"git", "docker", "gh", "code"}, "", []bool{true, true, true, true}},
		{"all missing", nil, "", []bool{false, false, false, false}},
		{"mixed", []string{"git", "code"}, "", []bool{true, false, false, true}},
		{"non-executable", []string{"docker", "gh", "code"}, "git", []bool{false, true, true, true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			t.Setenv("PATH", dir)
			t.Setenv("NO_COLOR", "1")
			require.NoError(t, os.WriteFile("agent-sandbox.json", []byte("invalid json"), 0o600))
			for _, name := range tt.installed {
				// A marker would reveal accidental execution instead of PATH lookup.
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nprintf invoked > invoked\n"), 0o755))
			}
			if tt.nonexec != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, tt.nonexec), nil, 0o644))
			}
			cmd, state := newRootCmd()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetArgs([]string{"doctor"})
			require.NoError(t, cmd.Execute())
			require.Equal(t, 0, state.status)
			var want strings.Builder
			want.WriteString("Dependency checks:\n\nDependency  Status         Usage\n----------  -------------  -----\n")
			for i, name := range []string{"git", "docker", "gh", "code"} {
				status := "\x1b[31mnot installed\x1b[0m"
				padding := ""
				if tt.wantStatus[i] {
					status = "\x1b[32minstalled\x1b[0m"
					padding = "    "
				}
				usage := []string{"Required", "Required", "Optional: PRs", "Optional: worktree-editor"}[i]
				fmt.Fprintf(&want, "%-10s  %s%s  %s\n", name, status, padding, usage)
			}
			require.Equal(t, want.String(), output.String())
			{
				_, err := os.Stat("invoked")
				require.ErrorIs(t, err, os.ErrNotExist)
			}
		})
	}
}

func TestDoctorUsage(t *testing.T) {
	for _, args := range [][]string{{"doctor", "extra"}, {"doctor", "--agent", "codex"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd, _ := newRootCmd()
			cmd.SetOut(io.Discard)
			cmd.SetArgs(args)
			assertUsageError(t, cmd.Execute())
		})
	}
}

func TestDoctorAppearsInHelp(t *testing.T) {
	cmd, _ := newRootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--help"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, output.String(), "doctor")
}

type doctorFailingWriter struct{}

func (doctorFailingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestDoctorOutputFailure(t *testing.T) {
	cmd, _ := newRootCmd()
	cmd.SetOut(doctorFailingWriter{})
	cmd.SetArgs([]string{"doctor"})
	require.ErrorIs(t, cmd.Execute(), io.ErrClosedPipe)
}
