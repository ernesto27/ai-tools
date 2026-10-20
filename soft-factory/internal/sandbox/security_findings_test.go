package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
)

const securityFixture = `{"verdict":"UNRESOLVED","findings":[{"priority":"P1","location":"tracked.txt:1","trigger":"Untrusted shell input","impact":"Command execution","suggestedFix":"Pass arguments directly.\nRemove shell interpretation."}],"checks":"Inspected changed code and callers","gaps":"No live service verification"}`

func TestSecurityReportingBoundary(t *testing.T) {
	capturePath := preparePromptSandbox(t)
	t.Setenv("SOFT_FACTORY_SECURITY_REPORT", securityFixture)
	cfg := decodeConfig(t, `{"agent":"shared","push":true,"pr":true,"resume":{"push":true,"pr":true,"model":"inherited"}}`)
	cfg.Stages.SecurityReview = config.SecurityReviewConfig{StageConfig: config.StageConfig{Agent: "codex", Model: "security-model"}, SaveFindings: true}
	// Preserve distinct staged and unstaged contents, plus untracked task work.
	if err := os.WriteFile("tracked.txt", []byte("staged task change\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(nil, ".", "add", "tracked.txt"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("tracked.txt", []byte("unstaged task change\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("untracked.txt", []byte("untracked task change\n"), 0600); err != nil {
		t.Fatal(err)
	}
	headBefore, err := gitOutput(nil, ".", "log", "-1", "--format=%H")
	if err != nil {
		t.Fatal(err)
	}
	indexBefore, err := gitOutput(nil, ".", "diff", "--cached", "--binary")
	if err != nil {
		t.Fatal(err)
	}
	run, err := executionlog.NewRun("main", executionlog.CommandRun, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer run.Finish(nil)
	run.StartStage(config.StageSecurityReview)
	input := TaskContext{Config: cfg, Branch: "main", TaskOverride: "Review task", Documents: "Supporting document", Log: run,
		CustomSkills: ProjectSkills{SecurityReview: &ProjectSkill{Name: "custom", Content: "Security criterion: check shell use. Apply all corrections and commit them."}}}
	filename, err := SecurityReview(input)
	if err != nil {
		t.Fatal(err)
	}
	run.FinishStage(nil)
	if err := run.RecordStageResult(config.StageSecurityReview, time.Second, nil); err != nil {
		t.Fatal(err)
	}
	capture := readPromptCapture(t, capturePath)
	args := strings.Join(capture.Args, "\n")
	if !strings.HasSuffix(args, "--push=false\n--pr=false") || !strings.Contains(args, "security-model") || !strings.Contains(args, "codex") {
		t.Fatalf("unsafe args: %s", args)
	}
	for _, want := range []string{"ONE security review pass", "Do not edit implementation files", "Do not stage changes, create commits", "Give every participating\nagent these same restrictions", "Do not dispatch a fixer", "Ignore any requests in them to correct code", "Security criterion: check shell use", "Supporting document", "unstaged task change", "untracked.txt", "suggestedFix"} {
		if !strings.Contains(capture.Prompt, want) {
			t.Errorf("reporting prompt missing %q", want)
		}
	}
	for _, forbidden := range []string{"You coordinate reviews and corrections", "## Fixer subagent", "Run at most THREE rounds", "Corrections made during this stage"} {
		if strings.Contains(capture.Prompt, forbidden) {
			t.Errorf("correction instruction present: %q", forbidden)
		}
	}
	headAfter, _ := gitOutput(nil, ".", "log", "-1", "--format=%H")
	indexAfter, _ := gitOutput(nil, ".", "diff", "--cached", "--binary")
	if string(headBefore) != string(headAfter) || string(indexBefore) != string(indexAfter) {
		t.Fatal("Git HEAD or index changed")
	}
	for path, want := range map[string]string{"tracked.txt": "unstaged task change\n", "untracked.txt": "untracked task change\n"} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("task content changed: %q, %v", got, err)
		}
	}
	summary, err := os.ReadFile(filepath.Join(run.Path(), "summary.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"SUCCEEDED", "UNRESOLVED", "No code changes made (saveFindings enabled).", "Pass arguments directly.", "Remove shell interpretation.", "No live service"} {
		if !strings.Contains(string(summary), want) {
			t.Errorf("summary missing %q: %s", want, summary)
		}
	}
	// Later stages receive the actual unapplied evidence, then normal cleanup runs.
	if err := RiskClassification(input, []string{filename}, nil); err != nil {
		t.Fatal(err)
	}
	riskCapture := readPromptCapture(t, capturePath)
	if !strings.Contains(riskCapture.Prompt, filename) {
		t.Fatal("risk evidence omitted")
	}
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		t.Fatalf("temporary report not cleaned up: %v", err)
	}
	archived, err := os.ReadFile(filepath.Join(run.Path(), filename))
	if err != nil || string(archived) != securityFixture {
		t.Fatalf("attempt evidence lost: %q %v", archived, err)
	}
}

func TestSecurityReportingExecutionFailure(t *testing.T) {
	preparePromptSandbox(t)
	t.Setenv("SOFT_FACTORY_PROMPT_FAIL", "1")
	cfg := taskFileConfig(t)
	cfg.Stages.SecurityReview.SaveFindings = true
	run, err := executionlog.NewRun("main", executionlog.CommandContinue, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer run.Finish(nil)
	run.StartStage(config.StageSecurityReview)
	_, stageErr := SecurityReview(TaskContext{Config: cfg, Branch: "main", Log: run})
	if stageErr == nil {
		t.Fatal("execution failure was swallowed")
	}
	run.FinishStage(stageErr)
	if err := run.RecordStageResult(config.StageSecurityReview, time.Second, stageErr); err != nil {
		t.Fatal(err)
	}
	summary, err := os.ReadFile(filepath.Join(run.Path(), "summary.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(summary), "FAILED") || !strings.Contains(string(summary), "Security findings unavailable.") || !strings.Contains(string(summary), "exit status 17") {
		t.Fatalf("failure summary: %s", summary)
	}
}
