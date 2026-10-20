package executionlog

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStageChangesPromptNilLog(t *testing.T) {
	var run *Run
	got, filename, err := run.StageChangesPrompt("task\n")
	require.NoError(t, err)
	require.Equal(t, "task\n", got)
	require.Empty(t, filename)
}

func TestStageChangesPromptReportPath(t *testing.T) {
	got, filename, err := (&Run{}).StageChangesPrompt("task")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(filename, ".soft-factory-stage-changes-"))
	require.True(t, strings.HasSuffix(filename, ".txt"))
	require.True(t, strings.HasPrefix(got, "task\n\n## Stage change summary\n\n"))
	require.Contains(t, got, `"/workspace/`+filename+`"`)
}
