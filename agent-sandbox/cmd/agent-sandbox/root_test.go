package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/sandbox"
)

func TestRunArgsAcceptsFilePromptWithoutPositionalPrompt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prompt.md")
	want := "# Fix login.\n"
	require.NoError(t, os.WriteFile(path, []byte(want), 0o644))

	cmd := newRunCmd(&commandState{})
	require.NoError(t, cmd.Flags().Set("filePrompt", path))

	assert.NoError(t, runArgs(cmd, nil))
}

func TestImageFlagIsRepeatableWithQuery(t *testing.T) {
	cmd := newRunCmd(&commandState{})
	require.NoError(t, cmd.Flags().Parse([]string{"--image", "first.png", "--image", "second.png", "-q", "describe them"}))

	images, err := cmd.Flags().GetStringArray("image")
	require.NoError(t, err)
	require.Equal(t, []string{"first.png", "second.png"}, images)
	require.Empty(t, cmd.Flags().Args())
	{
		query, err := cmd.Flags().GetString("query")
		require.NoError(t, err)
		require.Equal(t, "describe them", query)
	}
}

func TestRunArgsRejectsBothFlagPromptSources(t *testing.T) {
	cmd := newRunCmd(&commandState{})
	require.NoError(t, cmd.Flags().Set("filePrompt", "prompt.md"))
	require.NoError(t, cmd.Flags().Set("query", "different task"))

	err := runArgs(cmd, nil)
	assertUsageError(t, err)
}

func TestRunArgsRejectsMissingPromptSource(t *testing.T) {
	cmd := newRunCmd(&commandState{})
	assertUsageError(t, runArgs(cmd, nil))
}

func TestResumeRequiresBranch(t *testing.T) {
	cmd, _ := newRootCmd()
	cmd.SetArgs([]string{"resume", "-a", "codex", "-q", "continue"})

	assertUsageError(t, cmd.Execute())
}

func assertUsageError(t *testing.T, err error) {
	t.Helper()
	var usageErr sandbox.UsageError
	require.ErrorAs(t, err, &usageErr)
}
