package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/clierror"
	"github.com/ernesto27/ai-tools/agent-sandbox/internal/docker"
	"github.com/ernesto27/ai-tools/agent-sandbox/internal/sandbox"
)

func TestResourceLimitsConfig(t *testing.T) {
	for _, tc := range []struct {
		name, config, section string
		args                  []string
		cpu, memory           int64
		timeout               time.Duration
		badField              string
	}{
		{name: "omitted"},
		{name: "root defaults", config: `{"cpus":1.5,"memory":"2GiB","timeout":"30m"}`,
			cpu: 1500000000, memory: 2 << 30, timeout: 30 * time.Minute},
		{name: "independent precedence", config: `{"cpus":2,"memory":"1GiB","timeout":"1m","run":{"cpus":1.5,"timeout":"10s"}}`,
			args: []string{"--cpus=0.5"}, cpu: 500000000, memory: 1 << 30, timeout: 10 * time.Second},
		{name: "section clears root", section: "resume",
			config: `{"cpus":2,"memory":"1GiB","timeout":"1m","resume":{"cpus":0,"memory":"0","timeout":"0"}}`},
		{name: "CLI clears section", config: `{"run":{"cpus":2,"memory":"1GiB","timeout":"1m"}}`,
			args: []string{"--cpus=0", "--memory=0", "--timeout=0"}},
		{name: "invalid shadowed CPU", config: `{"cpus":"2"}`, args: []string{"--cpus=1"}, badField: "cpus must be a JSON number"},
		{name: "invalid inactive section", config: `{"resume":{"memory":1024}}`, badField: "resume.memory must be a JSON string"},
		{name: "null", config: `{"run":{"timeout":null}}`, badField: "run.timeout must be a JSON string"},
		{name: "negative CPU", args: []string{"--cpus=-1"}, badField: "--cpus"},
		{name: "CPU precision loss", args: []string{"--cpus=0.0000000001"}, badField: "--cpus"},
		{name: "CPU minimum", args: []string{"--cpus=0.01"}, cpu: 10_000_000},
		{name: "CPU below Docker minimum", args: []string{"--cpus=0.0099"}, badField: "--cpus must be at least 0.01"},
		{name: "CPU overflow", args: []string{"--cpus=9223372036.854775808"}, badField: "--cpus"},
		{name: "exact CPU boundary", args: []string{"--cpus=9223372036.854775807"}, cpu: 1<<63 - 1},
		{name: "fractional memory", args: []string{"--memory=1.5GiB"}, badField: "--memory"},
		{name: "invalid memory unit", args: []string{"--memory=2GB"}, badField: "--memory"},
		{name: "memory overflow", args: []string{"--memory=8388608TiB"}, badField: "--memory"},
		{name: "memory minimum", args: []string{"--memory=6MiB"}, memory: 6 << 20},
		{name: "memory below Docker minimum", args: []string{"--memory=1KiB"}, badField: "--memory must be at least 6 MiB"},
		{name: "memory one byte below minimum", args: []string{"--memory=6291455B"}, badField: "--memory must be at least 6 MiB"},
		{name: "memory zero with unit", args: []string{"--memory=0MiB"}},
		{name: "invalid memory in JSON", config: `{"resume":{"memory":"1KiB"}}`, badField: "resume.memory must be at least 6 MiB"},
		{name: "negative duration", args: []string{"--timeout=-1s"}, badField: "--timeout"},
		{name: "bare duration", args: []string{"--timeout=30"}, badField: "--timeout"},
		{name: "duration overflow", args: []string{"--timeout=9223372036854775808ns"}, badField: "--timeout"},
		{name: "subnanosecond duration", args: []string{"--timeout=0.1ns"}, badField: "--timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if tc.config != "" {
				require.NoError(t, os.WriteFile(localConfigFile, []byte(tc.config), 0o600))
			}
			var flags runFlags
			cmd := &cobra.Command{}
			flags.bind(cmd)
			require.NoError(t, cmd.Flags().Parse(append([]string{"-a", "codex", "-q", "test"}, tc.args...)))
			section := tc.section
			if section == "" {
				section = "run"
			}
			err := configRunArgs(section, &flags)(cmd, cmd.Flags().Args())
			if tc.badField != "" {
				assertUsageError(t, err)
				require.ErrorContains(t, err, tc.badField)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.cpu, flags.NanoCPUs)
			require.Equal(t, tc.memory, flags.MemoryBytes)
			require.Equal(t, tc.timeout, flags.Timeout)
		})
	}
}

func TestResourceLimitsRegisteredOnExecutionCommands(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, name := range []string{"run", "run-tui", "resume", "resume-tui"} {
		t.Run(name, func(t *testing.T) {
			root, _ := newRootCmd()
			root.SetArgs([]string{name, "-b", "test", "-a", "codex", "-q", "task", "--cpus=1.5", "--memory=2GiB", "--timeout=invalid"})
			cmd, err := root.ExecuteC()
			require.Equal(t, name, cmd.Name())
			assertUsageError(t, err)
			require.ErrorContains(t, err, "--timeout")
		})
	}
}

func TestLiveTimeoutResult(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		err    error
		want   string
	}{
		{name: "timeout", err: docker.ErrRunTimeout, want: "Execution timed out (exit status 124).\n"},
		{name: "agent exit 124", status: 124, want: "Execution failed (exit status 124).\n"},
		{name: "interrupt", err: context.Canceled, want: "Execution cancelled (exit status 130).\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			printLiveResult(&stdout, &stderr, tc.status, tc.err)
			require.Empty(t, stdout.String())
			require.Equal(t, tc.want, stderr.String())
		})
	}
}

func TestReportTimeout(t *testing.T) {
	for _, command := range []string{"run", "resume-tui"} {
		t.Run(command, func(t *testing.T) {
			err := &clierror.Error{
				Summary: "Agent execution timed out after 30m.",
				Details: []clierror.Detail{{Label: "Worktree", Value: "/tmp/worktrees/task"}},
				Cause:   errors.Join(docker.ErrRunTimeout, context.Canceled),
			}
			var stderr bytes.Buffer
			require.Equal(t, sandbox.ExitTimeout, reportError(&cobra.Command{Use: command}, &stderr, err, 80, false))
			require.Contains(t, stderr.String(), err.Summary)
			require.Contains(t, stderr.String(), "/tmp/worktrees/task")
		})
	}
}
