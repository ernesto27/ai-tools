package sandbox

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTaskPromptFragments(t *testing.T) {
	for _, documents := range []string{"", "documents {{literal}} <code> 100%"} {
		got, err := buildTaskPrompt(TaskContext{TaskOverride: "task", Documents: documents})
		require.NoError(t, err)
		want := "# Task\n\ntask"
		if documents != "" {
			want += "\n\n# Supporting context\n\n" + documents
		}
		require.Equal(t, want, got)
	}
}

func TestCustomSkillFragments(t *testing.T) {
	got, err := customSkillPrompt(nil, true)
	require.NoError(t, err)
	require.Empty(t, got)

	for _, reviewer := range []bool{false, true} {
		scope := ""
		if reviewer {
			scope = "Give this skill to the reviewer after the default review skill. It applies only to the reviewer role.\n\n"
		}
		got, err := customSkillPrompt(&ProjectSkill{Name: "name", Content: "body"}, reviewer)
		require.NoError(t, err)
		want := "\n## Additional project skill: name\n\n" + scope + "body\n"
		require.Equal(t, want, got)
	}
}

func TestRiskPromptUsesCurrentReportFiles(t *testing.T) {
	capturePath := preparePromptSandbox(t)
	// The classifier cleans up the files listed for this invocation.
	writePromptTestFile(t, "review.txt", "PASS\n")
	require.NoError(t, RiskClassification(TaskContext{TaskOverride: "task", Branch: "main"}, []string{"", "review.txt"}, nil))
	capture := readPromptCapture(t, capturePath)
	require.True(t, strings.HasSuffix(capture.Prompt, "## Review reports\n\n- \"review.txt\"\n\n"))
	require.NotContains(t, capture.Prompt, "No review reports are available")
	require.NotContains(t, capture.Prompt, "These review stages were disabled")

	_, err := os.Stat("review.txt")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func writePromptTestFile(t *testing.T, path, contents string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(contents), 0600))
}
