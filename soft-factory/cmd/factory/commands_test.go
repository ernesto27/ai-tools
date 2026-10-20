package main

import (
	"bytes"
	"io"
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
		{name: "continue", args: []string{"continue"}, want: workflowOptions{Continue: true}, wantRun: true},
		{name: "continue with jira", args: []string{"continue", "--jira", issue}, want: workflowOptions{Continue: true, IssueURL: issue}, wantRun: true},
		{name: "continue removed branch flag", args: []string{"continue", "--branch", " "}, wantErr: true},
		{name: "long config", args: []string{"--config", "x.json"}, wantErr: true},
		{name: "short config", args: []string{"-c", "x.json"}, wantErr: true},
		{name: "empty jira", args: []string{"--jira", ""}, wantErr: true},
		{name: "unknown command", args: []string{"extra"}, wantErr: true},
		{name: "removed review command", args: []string{"review"}, wantErr: true},
		{name: "single-dash long flag", args: []string{"-config", "x.json"}, wantErr: true},
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

func TestHelpOmitsConfigFlags(t *testing.T) {
	for _, args := range [][]string{{"--help"}} {
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
			require.NotContains(t, output.String(), "--config")
			require.NotContains(t, output.String(), "-c,")
		})
	}
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
