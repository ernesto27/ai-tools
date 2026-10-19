package executionlog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"soft-factory/internal/config"
)

const unresolvedSecurityReport = `{"verdict":"UNRESOLVED","findings":[{"priority":"P1","location":"handler.go:42","trigger":"Untrusted request reaches shell","impact":"Command execution","suggestedFix":"Pass arguments directly.\nAvoid the shell."}],"checks":"Inspected handler and callers.","gaps":"No live service verification."}`

func TestSecuritySummaryEvidence(t *testing.T) {
	for _, tc := range []struct{ name, report, want string }{
		{"unresolved", unresolvedSecurityReport, "Avoid the shell."},
		{"pass", `{"verdict":"PASS","findings":[],"checks":"Inspected callers","gaps":"None."}`, "No actionable findings."},
		{"blocked", `{"verdict":"BLOCKED","findings":[],"checks":"Inspected handler","gaps":"Missing callers"}`, "review evidence is insufficient"},
		{"empty", "", "Security findings unavailable."},
		{"invalid", "prompt echo: PASS", "Security findings unavailable."},
		{"incomplete", `{"verdict":"PASS"}`, "Security findings unavailable."},
		{"contradictory", strings.Replace(unresolvedSecurityReport, "UNRESOLVED", "PASS", 1), "Security findings unavailable."},
		{"missing", "missing", "Security findings unavailable."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "report.txt")
			if tc.report != "missing" {
				if err := os.WriteFile(path, []byte(tc.report), 0600); err != nil {
					t.Fatal(err)
				}
			}
			run := &Run{securityFindingsPath: path}
			got := run.securitySummaryLocked()
			if !strings.Contains(got, tc.want) {
				t.Fatalf("summary = %s", got)
			}
			if tc.name == "unresolved" {
				for _, want := range []string{"UNRESOLVED", "P1", "handler.go:42", "Untrusted request", "Command execution", "Suggested fix", "Inspected handler", "No live service"} {
					if !strings.Contains(got, want) {
						t.Errorf("summary missing %q: %s", want, got)
					}
				}
			}
		})
	}
}

func TestSecurityFindingsAttemptsAndRecovery(t *testing.T) {
	prepareLogTestDirectory(t)
	run, err := NewRun("main", CommandRun, []config.Stage{config.StageSecurityReview})
	if err != nil {
		t.Fatal(err)
	}
	path := run.Path()
	run.StartStage(config.StageSecurityReview)
	source := "first-review.txt"
	if err := os.WriteFile(source, []byte(unresolvedSecurityReport), 0600); err != nil {
		t.Fatal(err)
	}
	run.SecurityFindings(source, source, nil)
	run.FinishStage(errors.New("execution failed"))
	if err := run.RecordStageResult(config.StageSecurityReview, time.Second, errors.New("execution failed")); err != nil {
		t.Fatal(err)
	}
	run.Finish(nil)
	before, err := os.ReadFile(filepath.Join(path, summaryFile))
	if err != nil {
		t.Fatal(err)
	}
	run, err = OpenRun(path, "main", []config.Stage{config.StageSecurityReview})
	if err != nil {
		t.Fatal(err)
	}
	defer run.Finish(nil)
	run.StartStage(config.StageSecurityReview)
	run.BeginSecurityFindings() // An attempt with no report must not reuse the old one.
	run.FinishStage(nil)
	if err := run.RecordStageResult(config.StageSecurityReview, time.Second, nil); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(path, summaryFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(after), string(before)) || strings.Count(string(after), "UNRESOLVED") != 1 || !strings.Contains(string(after), "2. SECURITY REVIEW") || !strings.Contains(string(after), "Security findings unavailable.") || !strings.Contains(string(before), "execution failed") {
		t.Fatalf("attempt summaries = %s", after)
	}
	if _, err := os.Stat(filepath.Join(path, source)); err != nil {
		t.Fatalf("attempt archive missing: %v", err)
	}
}

func TestSecuritySummaryFailureRetainsEvidence(t *testing.T) {
	prepareLogTestDirectory(t)
	run, err := NewRun("main", CommandRun, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer run.Finish(nil)
	run.StartStage(config.StageSecurityReview)
	source := "review.txt"
	if err := os.WriteFile(source, []byte(unresolvedSecurityReport), 0600); err != nil {
		t.Fatal(err)
	}
	run.SecurityFindings(source, source, nil)
	if err := run.summary.Close(); err != nil {
		t.Fatal(err)
	}
	if err := run.RecordStageResult(config.StageSecurityReview, time.Second, nil); err == nil {
		t.Fatal("expected summary failure")
	}
	if data, err := os.ReadFile(filepath.Join(run.Path(), source)); err != nil || string(data) != unresolvedSecurityReport {
		t.Fatalf("report not retained: %q, %v", data, err)
	}
	// If archiving also fails, worktree evidence must survive later cleanup.
	run.path = filepath.Join(t.TempDir(), "missing")
	run.SecurityFindings(source, source, nil)
	if !run.RetainReviewReport(source) {
		t.Fatal("unpersisted report was not protected")
	}
}
