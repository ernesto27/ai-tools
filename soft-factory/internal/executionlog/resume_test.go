package executionlog

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"soft-factory/internal/config"
)

func TestResumeAppendsInSameFolder(t *testing.T) {
	prepareLogTestDirectory(t)
	run, err := NewRun("test1", CommandRun, []config.Stage{config.StageCodeReview})
	require.NoError(t, err)
	t.Cleanup(func() { run.Finish(nil) })
	path := run.Path()
	run.StartStage(config.StageCodeReview)
	run.Message("Original output")
	failure := errors.New("interrupted")
	run.FinishStage(failure)
	require.NoError(t, run.RecordStageResult(config.StageCodeReview, time.Second, failure))
	run.Finish(failure)

	resumed, err := OpenRun(path, "test1", []config.Stage{config.StageCodeReview})
	require.NoError(t, err)
	t.Cleanup(func() { resumed.Finish(nil) })
	wantPath, err := filepath.Abs(path)
	require.NoError(t, err)
	require.Equal(t, wantPath, resumed.Path())
	resumed.StartStage(config.StageCodeReview)
	resumed.Message("Resumed output")
	resumed.FinishStage(nil)
	require.NoError(t, resumed.RecordStageResult(config.StageCodeReview, time.Second, nil))
	resumed.Finish(nil)

	for file, fragments := range map[string][]string{
		"02-codeReview.log": {"Original output", "Resumed output"},
		"summary.log":       {"1. CODE REVIEW", "2. CODE REVIEW"},
		StageStatusFilename: {"codeReview,FAILED\ncodeReview,SUCCEEDED\n"},
	} {
		data, err := os.ReadFile(filepath.Join(path, file))
		require.NoError(t, err)
		for _, fragment := range fragments {
			assert.Contains(t, string(data), fragment)
		}
	}
	folders, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	require.Len(t, folders, 1)
}

func TestReviewArchiveUsesStageIdentifier(t *testing.T) {
	prepareLogTestDirectory(t)
	run, err := NewRun("test1", CommandRun, config.Stages)
	require.NoError(t, err)
	t.Cleanup(func() { run.Finish(nil) })
	source := filepath.Join(t.TempDir(), "review.txt")
	const evidence = "PASS\nReview findings\n"
	require.NoError(t, os.WriteFile(source, []byte(evidence), 0600))
	for _, stage := range []config.Stage{config.StageCodeReview, config.StageSecurityReview} {
		require.NoError(t, run.ArchiveReview(stage, source))
		archive := filepath.Join(run.Path(), string(stage)+".txt")
		data, err := os.ReadFile(archive)
		require.NoError(t, err)
		require.Equal(t, evidence, string(data))
		destination := filepath.Join(t.TempDir(), "restored.txt")
		require.NoError(t, run.RestoreReview(run.Path(), stage, destination))
		data, err = os.ReadFile(destination)
		require.NoError(t, err)
		require.Equal(t, evidence, string(data))
	}
}
