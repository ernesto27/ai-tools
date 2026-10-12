package executionlog

import (
	"strings"
	"testing"
)

func TestStageChangesPromptNilLog(t *testing.T) {
	var run *Run
	got, filename, err := run.StageChangesPrompt("task\n")
	if err != nil || got != "task\n" || filename != "" {
		t.Fatalf("got %q, %q, %v", got, filename, err)
	}
}

func TestStageChangesPromptReportPath(t *testing.T) {
	got, filename, err := (&Run{}).StageChangesPrompt("task")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(filename, ".soft-factory-stage-changes-") || !strings.HasSuffix(filename, ".txt") {
		t.Fatalf("unexpected report filename %q", filename)
	}
	if !strings.HasPrefix(got, "task\n\n## Stage change summary\n\n") || !strings.Contains(got, `"/workspace/`+filename+`"`) {
		t.Fatalf("unexpected prompt %q", got)
	}
}
