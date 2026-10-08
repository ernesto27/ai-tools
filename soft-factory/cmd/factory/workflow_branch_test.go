package main

import (
	"os"
	"testing"
)

func TestWorkflowBranch(t *testing.T) {
	for _, tt := range []struct {
		name     string
		opts     workflowOptions
		settings string
		want     string
		wantErr  bool
	}{
		{"implementation", workflowOptions{Implement: true}, `{"run":{"branch":"new-task"},"resume":{"branch":"old-task"}}`, "new-task", false},
		{"review only", workflowOptions{}, `{"run":{"branch":"new-task"},"resume":{"branch":"old-task"}}`, "old-task", false},
		{"continuation", workflowOptions{Implement: true, Continue: true}, `{"run":{"branch":"new-task"},"resume":{"branch":"old-task"}}`, "old-task", false},
		{"missing resume branch", workflowOptions{Implement: true, Continue: true}, `{"run":{"branch":"new-task"}}`, "", true},
		{"blank resume branch", workflowOptions{Implement: true, Continue: true}, `{"resume":{"branch":" "}}`, "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile("agent-sandbox.json", []byte(tt.settings), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := workflowBranch(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("branch = %q, want %q", got, tt.want)
			}
		})
	}
}
