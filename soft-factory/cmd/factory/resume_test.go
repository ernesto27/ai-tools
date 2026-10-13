package main

import (
	"os"
	"path/filepath"
	"testing"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
)

func TestResumeStartingStage(t *testing.T) {
	for _, tc := range []struct {
		status string
		want   config.Stage
	}{
		{"FAILED", config.StageCodeReview},
		{"SUCCEEDED", config.StageSecurityReview},
	} {
		t.Run(tc.status, func(t *testing.T) {
			path := t.TempDir()
			if err := os.WriteFile(filepath.Join(path, executionlog.StageStatusFilename), []byte("codeReview,"+tc.status+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			called := false
			cmd := newRootCmd(func(opts workflowOptions) error {
				called = true
				if opts.StartStage != tc.want {
					t.Errorf("stage = %q; want %q", opts.StartStage, tc.want)
				}
				if opts.ResumePath != path || !opts.Continue || !opts.Implement {
					t.Errorf("unexpected options: %+v", opts)
				}
				return nil
			})
			cmd.SetArgs([]string{"resume", path})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("workflow was not called")
			}
		})
	}
}
