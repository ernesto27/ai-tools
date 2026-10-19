package prompts

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestRenderMatchesOriginalPromptText(t *testing.T) {
	cases := map[string]map[string]any{
		"security-findings.txt":   {"ReportPath": "VALUE_ReportPath {{literal}} 100%", "Skill": "VALUE_Skill {{literal}} 100%", "AdditionalSkill": "VALUE_AdditionalSkill {{literal}} 100%", "Task": "VALUE_Task {{literal}} 100%", "HostEvidence": "VALUE_HostEvidence {{literal}} 100%"},
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
			if err != nil {
				t.Fatal(err)
			}
			got, err := Render(name, fields)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Fatalf("prompt differs from baseline:\ngot: %q\nwant: %q", got, want)
			}
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
			if err == nil || !strings.Contains(err.Error(), "render bundled prompt") || got != "" {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}

func TestRenderPreservesQuotedPathsAndLiteralInput(t *testing.T) {
	path := "/workspace/with spaces/quote\"\\line\n.txt"
	got, err := Render("review-report.txt", map[string]any{"ReportPath": path})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, strconv.Quote(path)) {
		t.Fatalf("quoted path missing from %q", got)
	}
	got, err = Render("task.txt", map[string]any{"Task": "{{.Secret}} <code> 100%"})
	if err != nil || got != "# Task\n\n{{.Secret}} <code> 100%" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestRenderIgnoresLocalPromptFiles(t *testing.T) {
	want, err := Render("task.txt", map[string]any{"Task": "original"})
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("internal/prompts", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("internal/prompts/task.txt", []byte("override"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("task.txt", []byte("override"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Render("task.txt", map[string]any{"Task": "original"})
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}
