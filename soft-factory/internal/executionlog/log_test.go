package executionlog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name    string
		elapsed time.Duration
		want    string
	}{
		{"zero", 0, "0 sec"},
		{"round down", 499 * time.Millisecond, "0 sec"},
		{"round up", 500 * time.Millisecond, "1 sec"},
		{"minute rollover", 59500 * time.Millisecond, "1 min"},
		{"implementation example", 60855 * time.Millisecond, "1 min 1 sec"},
		{"review example", 135183 * time.Millisecond, "2 min 15 sec"},
		{"exact hour", time.Hour, "1 hr"},
		{"omit zero minutes", time.Hour + 5*time.Second, "1 hr 5 sec"},
		{"all units", time.Hour + 12*time.Minute + 5*time.Second, "1 hr 12 min 5 sec"},
		{"over one day", 25 * time.Hour, "25 hr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.elapsed)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q; want %q",
					tt.elapsed, got, tt.want)
			}
		})
	}
}

func TestSummaryLifecycle(t *testing.T) {
	prepareLogTestDirectory(t)
	run, err := NewRun("feature")
	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}
	t.Cleanup(func() { run.Finish(nil) })

	summaryPath := filepath.Join(run.Path(), "summary.log")
	checkContents := func(want string) {
		t.Helper()
		data, err := os.ReadFile(summaryPath)
		if err != nil {
			t.Fatalf("read summary: %v", err)
		}
		if string(data) != want {
			t.Fatalf("summary = %q; want %q", data, want)
		}
	}

	want := "SOFTWARE FACTORY — RUN SUMMARY\n==================================================\n\n"
	checkContents(want)

	entries := []struct {
		stage   string
		elapsed time.Duration
		err     error
		line    string
	}{
		{"implementation", 60855 * time.Millisecond, nil, "1. IMPLEMENTATION\n   Status:    SUCCEEDED\n   Duration:  1 min 1 sec\n   Changes:   Change summary unavailable.\n\n"},
		{"code-review", 135183 * time.Millisecond, nil, "2. CODE REVIEW\n   Status:    SUCCEEDED\n   Duration:  2 min 15 sec\n   Changes:   Change summary unavailable.\n\n"},
		{"code-review", 18 * time.Second, errors.New("execute agent-sandbox: exit status 1\nverification failed"), "3. CODE REVIEW\n   Status:    FAILED\n   Duration:  18 sec\n   Changes:   Change summary unavailable.\n   Error:     execute agent-sandbox: exit status 1\n              verification failed\n\n"},
		{"security-review", time.Second, fmt.Errorf("execute agent-sandbox: %w", context.Canceled), "4. SECURITY REVIEW\n   Status:    INTERRUPTED\n   Duration:  1 sec\n   Changes:   Change summary unavailable.\n   Error:     execute agent-sandbox: context canceled\n\n"},
	}
	for _, entry := range entries {
		if err := run.RecordStageResult(entry.stage, entry.elapsed, entry.err); err != nil {
			t.Fatalf("record %s: %v", entry.stage, err)
		}
		want += entry.line
		checkContents(want)
	}

	file := run.summary
	run.Finish(nil)
	run.Finish(nil) // Finishing twice must be safe.
	want += "==================================================\n"
	if _, err := file.WriteString("unexpected\n"); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("write after Finish = %v; want closed file error", err)
	}
	if err := run.RecordStageResult("security-review", time.Second, nil); err == nil {
		t.Fatal("record after Finish: expected an error")
	}
	checkContents(want)
}

func TestStageChangeReports(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		missing bool
		want    string
	}{
		{"two lines", "Added duration tracking.\r\nAdded tests.\r\n", false, "Added duration tracking.\n              Added tests."},
		{"no changes", "No code changes made.\n", false, "No code changes made."},
		{"empty", " \n\r\n", false, "Change summary unavailable."},
		{"missing", "", true, "Change summary unavailable."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepareLogTestDirectory(t)
			run, err := NewRun("feature")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { run.Finish(nil) })
			report := filepath.Join(run.Path(), "changes.txt")
			if !tc.missing {
				if err := os.WriteFile(report, []byte(tc.content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			run.SetChangeSummaryPath(report)
			if err := run.RecordStageResult("implementation", time.Second, nil); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(run.Path(), summaryFile))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "   Changes:   "+tc.want+"\n") {
				t.Fatalf("unexpected summary: %s", data)
			}
			if _, err := os.Stat(report); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("report remains after recording: %v", err)
			}
		})
	}
}

func TestStageChangeReportRetainedOnSummaryWriteFailure(t *testing.T) {
	prepareLogTestDirectory(t)
	run, err := NewRun("feature")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { run.Finish(nil) })
	report := filepath.Join(run.Path(), "changes.txt")
	if err := os.WriteFile(report, []byte("Added tracking."), 0600); err != nil {
		t.Fatal(err)
	}
	run.SetChangeSummaryPath(report)
	if err := run.summary.Close(); err != nil {
		t.Fatal(err)
	}
	if err := run.RecordStageResult("implementation", time.Second, nil); err == nil {
		t.Fatal("expected summary write error")
	}
	if _, err := os.Stat(report); err != nil {
		t.Fatalf("report lost on write failure: %v", err)
	}
}

func TestStageChangeReportResetAndSymlinkRejected(t *testing.T) {
	prepareLogTestDirectory(t)
	run, err := NewRun("feature")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { run.Finish(nil) })
	run.StartStage("implementation")
	report := filepath.Join(run.Path(), "changes.txt")
	if err := os.WriteFile(report, []byte("Earlier stage changes."), 0600); err != nil {
		t.Fatal(err)
	}
	run.SetChangeSummaryPath(report)
	run.FinishStage(nil)
	run.StartStage("code-review")
	if run.changeSummaryPath != "" {
		t.Fatal("new stage retained earlier report path")
	}
	link := filepath.Join(run.Path(), "report-link.txt")
	if err := os.Symlink(report, link); err != nil {
		t.Fatal(err)
	}
	run.SetChangeSummaryPath(link)
	if err := run.RecordStageResult("code-review", time.Second, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(run.Path(), summaryFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Earlier stage changes") {
		t.Fatal("summary followed report symlink")
	}
}

func prepareLogTestDirectory(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	if output, err := exec.Command("git", "init", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("initialize test repository: %v\n%s", err, output)
	}
}
