package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
			require.NoError(t, os.WriteFile(filepath.Join(path, executionlog.StageStatusFilename), []byte("codeReview,"+tc.status+"\n"), 0600))
			called := false
			cmd := newRootCmd(func(opts workflowOptions) error {
				called = true
				assert.Equal(t, tc.want, opts.StartStage)
				assert.Equal(t, path, opts.ResumePath)
				assert.True(t, opts.Continue)
				return nil
			})
			cmd.SetArgs([]string{"resume", path})
			require.NoError(t, cmd.Execute())
			require.True(t, called)
		})
	}
}
