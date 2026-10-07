package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
			if err := os.WriteFile("agent-sandbox.json", []byte("invalid json"), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, name := range tt.installed {
				// A marker would reveal accidental execution instead of PATH lookup.
				if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nprintf invoked > invoked\n"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if tt.nonexec != "" {
				if err := os.WriteFile(filepath.Join(dir, tt.nonexec), nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			cmd, state := newRootCmd()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetArgs([]string{"doctor"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if state.status != 0 {
				t.Fatalf("status = %d, want 0", state.status)
			}
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
			if output.String() != want.String() {
				t.Fatalf("output = %q, want %q", output.String(), want.String())
			}
			if _, err := os.Stat("invoked"); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("tool invocation marker: %v", err)
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
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "doctor") {
		t.Fatalf("doctor missing from help: %s", output.String())
	}
}

type doctorFailingWriter struct{}

func (doctorFailingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestDoctorOutputFailure(t *testing.T) {
	cmd, _ := newRootCmd()
	cmd.SetOut(doctorFailingWriter{})
	cmd.SetArgs([]string{"doctor"})
	if err := cmd.Execute(); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("error = %v, want output failure", err)
	}
}
