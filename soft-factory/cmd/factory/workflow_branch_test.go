package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

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
		{"default workflow", workflowOptions{}, `{"run":{"branch":"new-task"},"resume":{"branch":"old-task"}}`, "new-task", false},
		{"continuation", workflowOptions{Continue: true}, `{"run":{"branch":"new-task"},"resume":{"branch":"old-task"}}`, "old-task", false},
		{"missing resume branch", workflowOptions{Continue: true}, `{"run":{"branch":"new-task"}}`, "", false},
		{"missing run section", workflowOptions{}, `{}`, "", false},
		{"null branch", workflowOptions{}, `{"run":{"branch":null}}`, "", false},
		{"empty branch", workflowOptions{}, `{"run":{"branch":""}}`, "", false},
		{"decode failure", workflowOptions{}, `{"run":{"branch":123}}`, "", true},
		{"blank resume branch", workflowOptions{Continue: true}, `{"resume":{"branch":" "}}`, " ", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			var cfg config.Config
			err := json.Unmarshal([]byte(`{"agentSandbox":`+tt.settings+`}`), &cfg)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if err != nil {
				return
			}
			got, _ := workflowBranch(cfg, tt.opts)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestWorkflowBranchPresence(t *testing.T) {
	for _, tc := range []struct {
		settings string
		wantSet  bool
	}{
		{`null`, false}, {`{}`, false}, {`{"branch":""}`, false}, {`{"run":{"branch":""}}`, true},
	} {
		var cfg config.Config
		require.NoError(t, json.Unmarshal([]byte(`{"agentSandbox":`+tc.settings+`}`), &cfg))
		branch, set := workflowBranch(cfg, workflowOptions{})
		require.Empty(t, branch)
		require.Equal(t, tc.wantSet, set)
		branch, set = workflowBranch(cfg, workflowOptions{Branch: "explicit"})
		require.Equal(t, "explicit", branch)
		require.True(t, set)
	}
}
