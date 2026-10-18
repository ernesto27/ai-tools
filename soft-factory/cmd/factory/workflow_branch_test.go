package main

import (
	"os"
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
		{"shared branch", workflowOptions{Implement: true}, `{"branch":"shared"}`, "shared", false},
		{"empty overrides shared", workflowOptions{Implement: true}, `{"branch":"shared","run":{"branch":""},"resume":{"branch":"other"}}`, "", false},
		{"explicit Factory branch", workflowOptions{Implement: true, Branch: "explicit"}, `{"run":{"branch":"configured"}}`, "explicit", false},
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
			if err := os.WriteFile(factoryConfigFile, []byte(`{"agentSandbox":`+tt.settings+`}`), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(factoryConfigFile)
			var got string
			if err == nil {
				got, err = workflowBranch(tt.opts, cfg)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("branch = %q, want %q", got, tt.want)
			}
		})
	}
}
