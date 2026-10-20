package prompts

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderMatchesOriginalPromptText(t *testing.T) {
	cases := map[string]map[string]any{
		"review-changes.txt":      {"ReportPath": "VALUE_ReportPath {{literal}} 100%", "Skill": "VALUE_Skill {{literal}} 100%", "AdditionalSkill": "VALUE_AdditionalSkill {{literal}} 100%", "Task": "VALUE_Task {{literal}} 100%", "HostEvidence": "VALUE_HostEvidence {{literal}} 100%"},
		"review.txt":              {"Skill": "VALUE_Skill {{literal}} 100%", "AdditionalSkill": "VALUE_AdditionalSkill {{literal}} 100%", "Task": "VALUE_Task {{literal}} 100%"},
		"review-report.txt":       {"ReportPath": "VALUE_ReportPath {{literal}} 100%"},
		"risk-classification.txt": {"ReportDirectory": "VALUE_ReportDirectory {{literal}} 100%", "Skill": "VALUE_Skill {{literal}} 100%", "AdditionalSkill": "VALUE_AdditionalSkill {{literal}} 100%", "Task": "VALUE_Task {{literal}} 100%", "ReviewEvidence": "VALUE_ReviewEvidence {{literal}} 100%"},
		"disabled-reviews.txt":    {"Stages": "VALUE_Stages {{literal}} 100%"},
		"stage-changes.txt":       {"ReportPath": "VALUE_ReportPath {{literal}} 100%"},
		"custom-skill.txt":        {"Name": "VALUE_Name {{literal}} 100%", "Scope": "VALUE_Scope {{literal}} 100%", "Content": "VALUE_Content {{literal}} 100%"},
		"reviewer-scope.txt":      {},
		"task.txt":                {"Task": "VALUE_Task {{literal}} 100%"},
		"supporting-context.txt":  {"Documents": "VALUE_Documents {{literal}} 100%"},
		"no-review-reports.txt":   {},
		"document.txt":            {"Path": "VALUE_Path {{literal}} 100%"},
		"google-document.txt":     {"Name": "VALUE_Name {{literal}} 100%", "FolderName": "VALUE_FolderName {{literal}} 100%", "ID": "VALUE_ID {{literal}} 100%"},
		"google-document-url.txt": {"ID": "VALUE_ID {{literal}} 100%"},
	}
	for name, fields := range cases {
		t.Run(name, func(t *testing.T) {
			want, err := os.ReadFile("testdata/" + name + ".golden")
			require.NoError(t, err)
			got, err := Render(name, fields)
			require.NoError(t, err)
			require.Equal(t, string(want), got)
		})
	}
}

func TestRenderErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		data any
	}{
		{"task.txt", map[string]any{}},
		{"task.txt", struct{ Other string }{}},
		{"absent.txt", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Render(tc.name, tc.data)
			require.ErrorContains(t, err, "render bundled prompt")
			require.Empty(t, got)
		})
	}
}

func TestRenderPreservesQuotedPathsAndLiteralInput(t *testing.T) {
	path := "/workspace/with spaces/quote\"\\line\n.txt"
	got, err := Render("review-report.txt", map[string]any{"ReportPath": path})
	require.NoError(t, err)
	require.Contains(t, got, strconv.Quote(path))
	got, err = Render("task.txt", map[string]any{"Task": "{{.Secret}} <code> 100%"})
	require.NoError(t, err)
	require.Equal(t, "# Task\n\n{{.Secret}} <code> 100%", got)
}

func TestRenderIgnoresLocalPromptFiles(t *testing.T) {
	want, err := Render("task.txt", map[string]any{"Task": "original"})
	require.NoError(t, err)
	t.Chdir(t.TempDir())
	require.NoError(t, os.MkdirAll("internal/prompts", 0700))
	require.NoError(t, os.WriteFile("internal/prompts/task.txt", []byte("override"), 0600))
	require.NoError(t, os.WriteFile("task.txt", []byte("override"), 0600))
	got, err := Render("task.txt", map[string]any{"Task": "original"})
	require.NoError(t, err)
	require.Equal(t, want, got)
}
