package sandbox

import (
	"os"
	"strings"
	"testing"
)

func TestTaskPromptFragments(t *testing.T) {
	for _, documents := range []string{"", "documents {{literal}} <code> 100%"} {
		got, err := buildTaskPrompt(TaskContext{TaskOverride: "task", Documents: documents})
		if err != nil {
			t.Fatal(err)
		}
		want := "# Task\n\ntask"
		if documents != "" {
			want += "\n\n# Supporting context\n\n" + documents
		}
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

func TestCustomSkillFragments(t *testing.T) {
	if got, err := customSkillPrompt(nil, true); err != nil || got != "" {
		t.Fatalf("got %q, %v", got, err)
	}
	for _, reviewer := range []bool{false, true} {
		scope := ""
		if reviewer {
			scope = "Give this skill to the reviewer after the default review skill. It applies only to the reviewer role.\n\n"
		}
		got, err := customSkillPrompt(&ProjectSkill{Name: "name", Content: "body"}, reviewer)
		if err != nil {
			t.Fatal(err)
		}
		want := "\n## Additional project skill: name\n\n" + scope + "body\n"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

func TestRiskPromptUsesCurrentReportFiles(t *testing.T) {
	capturePath := preparePromptSandbox(t)
	// The classifier cleans up the files listed for this invocation.
	writePromptTestFile(t, "review.txt", "PASS\n")
	if err := RiskClassification(TaskContext{TaskOverride: "task", Branch: "main"}, []string{"", "review.txt"}, nil); err != nil {
		t.Fatal(err)
	}
	capture := readPromptCapture(t, capturePath)
	if !strings.HasSuffix(capture.Prompt, "## Review reports\n\n- \"review.txt\"\n\n") {
		t.Fatalf("wrong review evidence: %s", capture.Prompt)
	}
	if strings.Contains(capture.Prompt, "No review reports are available") || strings.Contains(capture.Prompt, "These review stages were disabled") {
		t.Fatal("unexpected missing/disabled evidence")
	}
	if _, err := os.Stat("review.txt"); !os.IsNotExist(err) {
		t.Fatalf("review report remains: %v", err)
	}
}

func writePromptTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}
