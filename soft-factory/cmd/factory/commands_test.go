package main

import (
	"io"
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
		{name: "full workflow", args: nil, want: workflowOptions{ConfigPath: "config.json", Implement: true}, wantRun: true},
		{name: "long config", args: []string{"--config", "x.json"}, want: workflowOptions{ConfigPath: "x.json", Implement: true}, wantRun: true},
		{name: "short config", args: []string{"-c", "x.json"}, want: workflowOptions{ConfigPath: "x.json", Implement: true}, wantRun: true},
		{name: "review", args: []string{"review"}, want: workflowOptions{ConfigPath: "config.json"}, wantRun: true},
		{name: "jira before review", args: []string{"--jira", issue, "review"}, want: workflowOptions{ConfigPath: "config.json", IssueURL: issue}, wantRun: true},
		{name: "flags after review", args: []string{"review", "--jira", issue, "-c", "x.json"}, want: workflowOptions{ConfigPath: "x.json", IssueURL: issue}, wantRun: true},
		{name: "empty jira", args: []string{"--jira", ""}, wantErr: true},
		{name: "blank jira on review", args: []string{"review", "--jira", " "}, wantErr: true},
		{name: "unknown command", args: []string{"extra"}, wantErr: true},
		{name: "review extra arg", args: []string{"review", "extra"}, wantErr: true},
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
