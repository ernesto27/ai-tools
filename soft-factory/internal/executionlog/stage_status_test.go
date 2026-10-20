package executionlog

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"soft-factory/internal/config"
)

func TestStageStatusAppendAndReopen(t *testing.T) {
	path := t.TempDir()
	saved, err := NewStageStatus(path, true)
	require.NoError(t, err)
	t.Cleanup(func() { saved.Close() })
	require.NoError(t, saved.append(config.StageImplementation, StatusSucceeded))
	require.NoError(t, saved.Close())
	saved, err = NewStageStatus(path, false)
	require.NoError(t, err)
	stage, status, err := saved.Last()
	require.NoError(t, err)
	require.Equal(t, config.StageImplementation, stage)
	require.Equal(t, StatusSucceeded, status)
	require.NoError(t, saved.append(config.StageCodeReview, statusFailed))
	stage, status, err = saved.Last()
	require.NoError(t, err)
	require.Equal(t, config.StageCodeReview, stage)
	require.Equal(t, statusFailed, status)
	data, err := os.ReadFile(filepath.Join(path, StageStatusFilename))
	require.NoError(t, err)
	require.Equal(t, "implementation,SUCCEEDED\ncodeReview,FAILED\n", string(data))
}

func TestStageStatusLast(t *testing.T) {
	for _, tc := range []struct {
		name, content  string
		stage          config.Stage
		status         string
		empty, invalid bool
	}{
		{name: "empty", empty: true},
		{name: "blank", content: "\n \n", empty: true},
		{name: "last entry", content: "implementation,SUCCEEDED\ncodeReview,FAILED\n", stage: config.StageCodeReview, status: statusFailed},
		{name: "no trailing newline", content: "securityReview,INTERRUPTED", stage: config.StageSecurityReview, status: statusInterrupted},
		{name: "trailing blank lines", content: "codeReview,SUCCEEDED\n\n", stage: config.StageCodeReview, status: StatusSucceeded},
		{name: "missing comma", content: "codeReview", invalid: true},
		{name: "missing stage", content: ",FAILED", invalid: true},
		{name: "missing status", content: "codeReview,", invalid: true},
		{name: "extra field", content: "codeReview,FAILED,extra", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(path, StageStatusFilename), []byte(tc.content), 0600))
			saved, err := NewStageStatus(path, false)
			require.NoError(t, err)
			t.Cleanup(func() { saved.Close() })
			stage, status, err := saved.Last()
			if tc.empty {
				require.ErrorIs(t, err, io.EOF)
				return
			}
			if tc.invalid {
				require.Error(t, err)
				require.NotErrorIs(t, err, io.EOF)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.stage, stage)
			require.Equal(t, tc.status, status)
		})
	}
}

func TestNewStageStatusDoesNotCreateWhenDisabled(t *testing.T) {
	path := t.TempDir()
	if saved, err := NewStageStatus(path, false); err == nil {
		saved.Close()
		require.FailNow(t, "opening a missing status file succeeded")
	} else {
		require.ErrorIs(t, err, os.ErrNotExist)
	}

	_, err := os.Stat(filepath.Join(path, StageStatusFilename))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestRunPath(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.Mkdir("run", 0700))
	want, err := filepath.Abs("run")
	require.NoError(t, err)
	for _, path := range []string{"run", want} {
		got, err := RunPath(path)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	require.NoError(t, os.WriteFile("file", nil, 0600))
	for _, path := range []string{"", " ", "missing", "file"} {
		_, err := RunPath(path)
		assert.Error(t, err)
	}
}
