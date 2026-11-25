package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCmdRouting(t *testing.T) {
	const issue = "https://a.atlassian.net/browse/X-1"
	tests := []struct {
		name    string
		args    []string
		want    workflowOptions
		wantRun bool
		wantErr bool
	}{
		{name: "full workflow", args: nil, want: workflowOptions{}, wantRun: true},
		{name: "full workflow with task file", args: []string{"--file", "cron task.txt"}, want: workflowOptions{TaskFile: "cron task.txt"}, wantRun: true},
		{name: "full workflow with two task sources", args: []string{"--file", "task.txt", "--jira", issue}, wantErr: true},
		{name: "continue", args: []string{"continue"}, want: workflowOptions{Continue: true}, wantRun: true},
		{name: "continue with jira", args: []string{"continue", "--jira", issue}, want: workflowOptions{Continue: true, IssueURL: issue}, wantRun: true},
		{name: "continue removed branch flag", args: []string{"continue", "--branch", " "}, wantErr: true},
		{name: "long config", args: []string{"--config", "x.json"}, want: workflowOptions{ConfigPath: "x.json"}, wantRun: true},
		{name: "short config", args: []string{"-c", "x.json"}, want: workflowOptions{ConfigPath: "x.json"}, wantRun: true},
		{name: "continue with config", args: []string{"continue", "-c", "x.json"}, want: workflowOptions{Continue: true, ConfigPath: "x.json"}, wantRun: true},
		{name: "empty config", args: []string{"--config", " "}, wantErr: true},
		{name: "empty jira", args: []string{"--jira", ""}, wantErr: true},
		{name: "unknown command", args: []string{"extra"}, wantErr: true},
		{name: "removed review command", args: []string{"review"}, wantErr: true},
		{name: "help", args: []string{"--help"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got workflowOptions
			ran := false
			cmd := newRootCmd(func(opts workflowOptions) error {
				ran = true
				got = opts
				return nil
			})
			cmd.SetArgs(tt.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)

			err := cmd.Execute()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantRun, ran)
			if ran {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestImageFlagRouting(t *testing.T) {
	const issue = "https://a.atlassian.net/browse/X-1"
	for _, mode := range []string{"default", "continue"} {
		t.Run(mode, func(t *testing.T) {
			for _, tt := range []struct {
				name    string
				images  []string
				wantErr bool
			}{
				{"jira and repeated images", []string{"--image", "a,b.png", "--image", " c d.png "}, false},
				{"empty image", []string{"--image="}, true},
				{"blank image after valid image", []string{"--image", "a.png", "--image", " \t"}, true},
			} {
				t.Run(tt.name, func(t *testing.T) {
					args := []string{"--jira", issue}
					if mode == "continue" {
						args = append(args, mode)
					}
					called := false
					cmd := newRootCmd(func(opts workflowOptions) error {
						called = true
						require.Equal(t, issue, opts.IssueURL)
						require.Equal(t, mode == "continue", opts.Continue)
						return nil
					})
					var output bytes.Buffer
					cmd.SetOut(&output)
					cmd.SetErr(&output)
					cmd.SetArgs(append(args, tt.images...))
					if tt.wantErr {
						require.EqualError(t, cmd.Execute(), "--image requires a non-empty file path")
					} else {
						require.NoError(t, cmd.Execute())
					}
					require.Equal(t, !tt.wantErr, called)
					require.Empty(t, output.String())
				})
			}
		})
	}
	cmd := newRootCmd(func(workflowOptions) error {
		t.Fatal("recovery must not accept --image")
		return nil
	})
	cmd.SetArgs([]string{"resume", "saved-run", "--image", "a.png"})
	require.EqualError(t, cmd.Execute(), "unknown flag: --image")
}

func TestHelpShowsConfigFlags(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"continue", "--help"}, {"resume", "--help"}, {"audit", "--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var output bytes.Buffer
			cmd := newRootCmd(func(workflowOptions) error {
				require.FailNow(t, "help invoked the workflow")
				return nil
			})
			cmd.SetArgs(args)
			cmd.SetOut(&output)
			cmd.SetErr(io.Discard)
			require.NoError(t, cmd.Execute())
			require.Contains(t, output.String(), "-c, --config")
			require.Contains(t, output.String(), factoryConfigFile)
		})
	}
}

func TestAuditLoadsCustomConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	cmd := newRootCmd(func(workflowOptions) error {
		require.FailNow(t, "audit invoked the workflow")
		return nil
	})
	cmd.SetArgs([]string{"audit", "--log", "run.log", "--config", "absent.json"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.Execute()
	require.ErrorIs(t, err, os.ErrNotExist)
	require.ErrorContains(t, err, "absent.json")
}

func TestVersionFlags(t *testing.T) {
	const releaseTag = "software-factory-v9.8.7"
	for _, flag := range []string{"-v", "--version"} {
		t.Run(flag, func(t *testing.T) {
			var output bytes.Buffer
			ran := false
			cmd := newRootCmd(func(workflowOptions) error {
				ran = true
				return nil
			})
			cmd.Version = releaseTag
			cmd.SetArgs([]string{flag})
			cmd.SetOut(&output)
			cmd.SetErr(io.Discard)

			require.NoError(t, cmd.Execute())
			assert.False(t, ran)

			got, want := output.String(), releaseTag+"\n"
			assert.Equal(t, want, got)
		})
	}
}
