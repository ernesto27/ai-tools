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
	run, err := NewRun("feature", "run", stageNames)
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
			run, err := NewRun("feature", "run", stageNames)
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
	run, err := NewRun("feature", "run", stageNames)
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
	run, err := NewRun("feature", "run", stageNames)
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

func TestHeaderListsOnlyPlannedStages(t *testing.T) {
	for _, tc := range []struct {
		name    string
		planned []string
		want    string
	}{
		{"disabled stage omitted", []string{"implementation", "code-review", "risk-classification"}, "implementation, code-review, risk-classification"},
		{"none planned", nil, "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepareLogTestDirectory(t)
			run, err := NewRun("feature", "run", tc.planned)
			if err != nil {
				t.Fatal(err)
			}
			run.StartStage("implementation")
			run.FinishStage(nil)
			run.Finish(nil)
			data, err := os.ReadFile(filepath.Join(run.Path(), "01-implementation.log"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "\nPlanned stages: "+tc.want+"\n") {
				t.Fatalf("header does not list planned stages %q:\n%s", tc.want, data)
			}
		})
	}
}

func prepareLogTestDirectory(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	if output, err := exec.Command("git", "init", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("initialize test repository: %v\n%s", err, output)
	}
}

func TestInvocationDirectoryAllocation(t *testing.T) {
	for _, firstCommand := range []string{"run", "continue"} {
		t.Run(firstCommand+" first", func(t *testing.T) {
			t.Chdir(t.TempDir())
			// Earlier flat logs must remain untouched.
			legacy := filepath.Join("logs", "feature-logs-2026-10-08_19-38-48.007046183")
			if err := os.MkdirAll(legacy, 0700); err != nil {
				t.Fatal(err)
			}
			legacyFile := filepath.Join(legacy, summaryFile)
			if err := os.WriteFile(legacyFile, []byte("earlier summary"), 0600); err != nil {
				t.Fatal(err)
			}
			// A non-UTC input also produces the existing UTC timestamp format.
			started := time.Date(2026, 10, 8, 21, 38, 48, 7046183, time.FixedZone("UTC+2", 2*60*60))
			parent := filepath.Join("logs", "feature-logs")
			name := firstCommand + "-2026-10-08_19-38-48.007046183"
			for i := 1; i <= 3; i++ {
				path, err := createRunDirectory("feature/logs", firstCommand, started)
				if err != nil {
					t.Fatal(err)
				}
				want := filepath.Join(parent, name)
				if i > 1 {
					want += fmt.Sprintf("-%d", i)
				}
				if path != want || filepath.IsAbs(path) {
					t.Fatalf("path = %q, want relative %q", path, want)
				}
				if err := os.WriteFile(filepath.Join(path, summaryFile), []byte(path), 0600); err != nil {
					t.Fatal(err)
				}
			}
			otherCommand := "continue"
			if firstCommand == "continue" {
				otherCommand = "run"
			}
			otherPath, err := createRunDirectory("feature/logs", otherCommand, started)
			if err != nil {
				t.Fatal(err)
			}
			if otherPath != filepath.Join(parent, otherCommand+"-2026-10-08_19-38-48.007046183") {
				t.Fatalf("other invocation path = %q", otherPath)
			}
			children, err := os.ReadDir(parent)
			if err != nil || len(children) != 4 {
				t.Fatalf("children = %v, error = %v; want four", children, err)
			}
			for _, child := range children {
				path := filepath.Join(parent, child.Name())
				if path == otherPath {
					continue
				}
				data, err := os.ReadFile(filepath.Join(path, summaryFile))
				if err != nil || string(data) != path {
					t.Fatalf("earlier summary at %s = %q, error = %v", path, data, err)
				}
			}
			data, err := os.ReadFile(legacyFile)
			if err != nil || string(data) != "earlier summary" {
				t.Fatalf("legacy summary = %q, error = %v", data, err)
			}
			for _, path := range []string{"logs", parent, otherPath} {
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != 0700 {
					t.Errorf("%s permissions = %04o, want 0700", path, info.Mode().Perm())
				}
			}
		})
	}
}

func TestInvocationArtifactsAreIsolated(t *testing.T) {
	prepareLogTestDirectory(t)
	// Compare summary permissions to the prior os.Create behavior, including umask.
	reference, err := os.Create(filepath.Join(t.TempDir(), summaryFile))
	if err != nil {
		t.Fatal(err)
	}
	referenceInfo, statErr := reference.Stat()
	if err := errors.Join(statErr, reference.Close()); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join("logs", "Feature-logs._-9-")
	paths := make(map[string]int)
	for i, command := range []string{"run", "continue", "continue", "run"} {
		run, err := NewRun("Feature/logs._-9é", command, []string{"implementation"})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { run.Finish(nil) })
		if filepath.Dir(run.Path()) != parent || !strings.HasPrefix(filepath.Base(run.Path()), command+"-") {
			t.Fatalf("unexpected invocation path: %s", run.Path())
		}
		if _, exists := paths[run.Path()]; exists {
			t.Fatalf("reused invocation path: %s", run.Path())
		}
		paths[run.Path()] = i
		run.StartStage("implementation")
		fmt.Fprintf(run.Stream("stdout"), "invocation %d\n", i)
		run.FinishStage(nil)
		if err := run.RecordStageResult("implementation", time.Second, nil); err != nil {
			t.Fatal(err)
		}
		run.Finish(nil)
	}
	children, err := os.ReadDir(parent)
	if err != nil || len(children) != 4 {
		t.Fatalf("children = %v, error = %v; want four", children, err)
	}
	for path, invocation := range paths {
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) != 2 {
			t.Fatalf("artifacts in %s = %v, error = %v", path, entries, err)
		}
		data, err := os.ReadFile(filepath.Join(path, "01-implementation.log"))
		if err != nil || strings.Count(string(data), "invocation ") != 1 || !strings.Contains(string(data), fmt.Sprintf("invocation %d\n", invocation)) {
			t.Fatalf("stage log at %s = %q, error = %v", path, data, err)
		}
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				t.Fatal(err)
			}
			want := os.FileMode(0600)
			if entry.Name() == summaryFile {
				// The summary retains os.Create's existing permissions.
				want = referenceInfo.Mode().Perm()
			}
			if info.Mode().Perm() != want {
				t.Errorf("%s permissions = %04o, want %04o", entry.Name(), info.Mode().Perm(), want)
			}
		}
	}
}

func TestBranchDirectoryCreationFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("logs", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("logs", "feature"), []byte("existing file"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := createRunDirectory("feature", "continue", time.Now())
	if err == nil || !strings.Contains(err.Error(), "create branch log directory") {
		t.Fatalf("expected contextual branch directory error, got %v", err)
	}
}
