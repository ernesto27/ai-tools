package main

import (
	"encoding/json"
	"testing"

	"soft-factory/internal/config"
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
		{"missing resume branch", workflowOptions{Implement: true, Continue: true}, `{"run":{"branch":"new-task"}}`, "", false},
		{"missing run section", workflowOptions{Implement: true}, `{}`, "", false},
		{"null branch", workflowOptions{Implement: true}, `{"run":{"branch":null}}`, "", false},
		{"empty branch", workflowOptions{Implement: true}, `{"run":{"branch":""}}`, "", false},
		{"decode failure", workflowOptions{Implement: true}, `{"run":{"branch":123}}`, "", true},
		{"blank resume branch", workflowOptions{Implement: true, Continue: true}, `{"resume":{"branch":" "}}`, " ", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			var cfg config.Config
			err := json.Unmarshal([]byte(`{"agentSandbox":`+tt.settings+`}`), &cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("decode error = %v", err)
			}
			if err != nil {
				return
			}
			got, _ := workflowBranch(cfg, tt.opts)
			if got != tt.want {
				t.Fatalf("branch = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWorkflowBranchPresence(t *testing.T) {
	for _, tc := range []struct {
		settings string
		wantSet  bool
	}{
		{`null`, false}, {`{}`, false}, {`{"branch":""}`, true}, {`{"run":{"branch":""}}`, true},
	} {
		var cfg config.Config
		if err := json.Unmarshal([]byte(`{"agentSandbox":`+tc.settings+`}`), &cfg); err != nil {
			t.Fatal(err)
		}
		branch, set := workflowBranch(cfg, workflowOptions{Implement: true})
		if branch != "" || set != tc.wantSet {
			t.Fatalf("branch presence for %s = %q, %t", tc.settings, branch, set)
		}
		branch, set = workflowBranch(cfg, workflowOptions{Implement: true, Branch: "explicit"})
		if branch != "explicit" || !set {
			t.Fatal("explicit branch override lost")
		}
	}
}
