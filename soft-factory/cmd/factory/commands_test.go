package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
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
			if (err != nil) != tt.wantErr {
				t.Fatalf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
			if ran != tt.wantRun {
				t.Fatalf("workflow ran = %v, want %v", ran, tt.wantRun)
			}
			if ran && got != tt.want {
				t.Errorf("options = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestHelpOmitsConfigFlags(t *testing.T) {
	for _, args := range [][]string{{"--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var output bytes.Buffer
			cmd := newRootCmd(func(workflowOptions) error {
				t.Fatal("help invoked the workflow")
				return nil
			})
			cmd.SetArgs(args)
			cmd.SetOut(&output)
			cmd.SetErr(io.Discard)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(output.String(), "--config") || strings.Contains(output.String(), "-c,") {
				t.Fatalf("help advertises removed config flags: %s", output.String())
			}
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

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if ran {
				t.Error("version flag ran the workflow")
			}
			if got, want := output.String(), releaseTag+"\n"; got != want {
				t.Errorf("version output = %q, want %q", got, want)
			}
		})
	}
}
