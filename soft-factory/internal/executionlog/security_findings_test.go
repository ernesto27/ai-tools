package executionlog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
				require.NoError(t, os.WriteFile(path, []byte(tc.report), 0600))
			}
			run := &Run{securityFindingsPath: path}
			got := run.securitySummaryLocked()
			require.Contains(t, got, tc.want)
			if tc.name == "unresolved" {
				for _, want := range []string{"UNRESOLVED", "P1", "handler.go:42", "Untrusted request", "Command execution", "Suggested fix", "Inspected handler", "No live service"} {
					assert.Contains(t, got, want)
				}
			}
		})
	}
}

func TestSecurityFindingsAttemptsAndRecovery(t *testing.T) {
	prepareLogTestDirectory(t)
	run, err := NewRun("main", CommandRun, []config.Stage{config.StageSecurityReview})
	require.NoError(t, err)
	path := run.Path()
	run.StartStage(config.StageSecurityReview)
	source := "first-review.txt"
	require.NoError(t, os.WriteFile(source, []byte(unresolvedSecurityReport), 0600))
	run.SecurityFindings(source, source, nil)
	run.FinishStage(errors.New("execution failed"))
	require.NoError(t, run.RecordStageResult(config.StageSecurityReview, time.Second, errors.New("execution failed")))
	run.Finish(nil)
	before, err := os.ReadFile(filepath.Join(path, summaryFile))
	require.NoError(t, err)
	run, err = OpenRun(path, "main", []config.Stage{config.StageSecurityReview})
	require.NoError(t, err)
	defer run.Finish(nil)
	run.StartStage(config.StageSecurityReview)
	run.BeginSecurityFindings() // An attempt with no report must not reuse the old one.
	run.FinishStage(nil)
	require.NoError(t, run.RecordStageResult(config.StageSecurityReview, time.Second, nil))
	after, err := os.ReadFile(filepath.Join(path, summaryFile))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(after), string(before)))
	require.Equal(t, 1, strings.Count(string(after), "UNRESOLVED"))
	require.Contains(t, string(after), "2. SECURITY REVIEW")
	require.Contains(t, string(after), "Security findings unavailable.")
	require.Contains(t, string(before), "execution failed")

	_, err = os.Stat(filepath.Join(path, source))
	require.NoError(t, err)
}

func TestSecuritySummaryFailureRetainsEvidence(t *testing.T) {
	prepareLogTestDirectory(t)
	run, err := NewRun("main", CommandRun, nil)
	require.NoError(t, err)
	defer run.Finish(nil)
	run.StartStage(config.StageSecurityReview)
	source := "review.txt"
	require.NoError(t, os.WriteFile(source, []byte(unresolvedSecurityReport), 0600))
	run.SecurityFindings(source, source, nil)
	require.NoError(t, run.summary.Close())
	require.Error(t, run.RecordStageResult(config.StageSecurityReview, time.Second, nil))

	data, err := os.ReadFile(filepath.Join(run.Path(), source))
	require.NoError(t, err)
	require.Equal(t, unresolvedSecurityReport, string(data))

	// If archiving also fails, worktree evidence must survive later cleanup.
	run.path = filepath.Join(t.TempDir(), "missing")
	run.SecurityFindings(source, source, nil)
	require.True(t, run.RetainReviewReport(source))
}
