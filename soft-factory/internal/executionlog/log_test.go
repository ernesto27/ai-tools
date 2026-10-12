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
	run, err := NewRun("feature", CommandRun, stageNames)
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
		{"implementation", 60855 * time.Millisecond, nil, "1. IMPLEMENTATION\n   Status:    SUCCEEDED\n   Agent:     unavailable\n   Model:     unavailable\n   Duration:  1 min 1 sec\n   Changes:   Change summary unavailable.\n\n"},
		{"code-review", 135183 * time.Millisecond, nil, "2. CODE REVIEW\n   Status:    SUCCEEDED\n   Agent:     unavailable\n   Model:     unavailable\n   Duration:  2 min 15 sec\n   Changes:   Change summary unavailable.\n\n"},
		{"code-review", 18 * time.Second, errors.New("execute agent-sandbox: exit status 1\nverification failed"), "3. CODE REVIEW\n   Status:    FAILED\n   Agent:     unavailable\n   Model:     unavailable\n   Duration:  18 sec\n   Changes:   Change summary unavailable.\n   Error:     execute agent-sandbox: exit status 1\n              verification failed\n\n"},
		{"security-review", time.Second, fmt.Errorf("execute agent-sandbox: %w", context.Canceled), "4. SECURITY REVIEW\n   Status:    INTERRUPTED\n   Agent:     unavailable\n   Model:     unavailable\n   Duration:  1 sec\n   Changes:   Change summary unavailable.\n   Error:     execute agent-sandbox: context canceled\n\n"},
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

func TestSummaryRuntimeSelectionLifecycle(t *testing.T) {
	prepareLogTestDirectory(t)
	run, err := NewRun("feature", CommandContinue, stageNames)
	if err != nil {
		t.Fatal(err)
	}
	defer run.Finish(nil)
	for i, tt := range []struct {
		stage, agent, model string
		err                 error
	}{
		{"implementation", "codex", "final-model", nil},
		{"code-review", "", "", errors.New("failed before launch")},
		{"security-review", "claude", "", context.Canceled},
		{"risk-classification", "", "runtime-model", nil},
	} {
		run.StartStage(tt.stage)
		// Configured details never populate the summary, even after a known stage.
		run.Details("configured-agent", "configured-model", "configured-provider")
		if tt.agent != "" || tt.model != "" {
			run.RuntimeSelection("earlier-agent", "earlier-model")
			run.RuntimeSelection(tt.agent, tt.model)
		}
		run.FinishStage(tt.err)
		if err := run.RecordStageResult(tt.stage, time.Second, tt.err); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(run.Path(), summaryFile))
		if err != nil {
			t.Fatal(err)
		}
		entry := fmt.Sprintf("%d. %s\n   Status:    %s\n   Agent:     %s\n   Model:     %s\n   Duration:  1 sec\n",
			i+1, strings.ToUpper(strings.ReplaceAll(tt.stage, "-", " ")), outcome(tt.err), selectionName(tt.agent), selectionName(tt.model))
		if !strings.Contains(string(data), entry) || strings.Contains(string(data), "earlier-") || strings.Contains(string(data), "configured-") {
			t.Fatalf("unexpected summary: %s", data)
		}
	}
	if err := run.RecordSkippedStage("Skipping disabled stage: codeReview"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(run.Path(), summaryFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "   Agent:") != 4 || strings.Count(string(data), "   Model:") != 4 || !strings.HasSuffix(string(data), "Skipping disabled stage: codeReview\n\n") {
		t.Fatalf("skipped stage added result fields: %s", data)
	}
}

func TestSelectionNameRejectsUnsafeValues(t *testing.T) {
	for _, tt := range []struct {
		name, value, want string
	}{
		{"plain", "gpt-5.2-codex", "gpt-5.2-codex"},
		{"trimmed", "  claude-opus  ", "claude-opus"},
		{"at limit", strings.Repeat("m", selectionNameLimit), strings.Repeat("m", selectionNameLimit)},
		{"empty", " ", "unavailable"},
		{"newline", "model\n   Model: injected", "unavailable"},
		{"tab", "model\tname", "unavailable"},
		{"escape", "model\x1b[2A\x1b[2Kforged", "unavailable"},
		{"nul", "model\x00", "unavailable"},
		{"delete", "model\x7f", "unavailable"},
		{"c1 control", "model\u009bforged", "unavailable"},
		{"line separator", "model forged", "unavailable"},
		{"paragraph separator", "model forged", "unavailable"},
		{"invalid utf8", "model\xff", "unavailable"},
		{"over limit", strings.Repeat("m", selectionNameLimit+1), "unavailable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectionName(tt.value); got != tt.want {
				t.Fatalf("selectionName(%q) = %q; want %q", tt.value, got, tt.want)
			}
		})
	}

	prepareLogTestDirectory(t)
	run, err := NewRun("feature", CommandRun, stageNames)
	if err != nil {
		t.Fatal(err)
	}
	defer run.Finish(nil)
	run.StartStage("implementation")
	run.RuntimeSelection("claude x", "model\x1b]0;title\x07")
	run.FinishStage(nil)
	if err := run.RecordStageResult("implementation", time.Second, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(run.Path(), summaryFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "   Agent:     unavailable\n   Model:     unavailable\n") || strings.ContainsAny(string(data), "\x1b ") {
		t.Fatalf("unsafe selection reached summary: %q", data)
	}
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
			run, err := NewRun("feature", CommandRun, stageNames)
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
	run, err := NewRun("feature", CommandRun, stageNames)
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
	run, err := NewRun("feature", CommandRun, stageNames)
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
			run, err := NewRun("feature", CommandRun, tc.planned)
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
	for _, firstCommand := range []string{CommandRun, CommandContinue} {
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
			otherCommand := CommandContinue
			if firstCommand == CommandContinue {
				otherCommand = CommandRun
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
	for i, command := range []string{CommandRun, CommandContinue, CommandContinue, CommandRun} {
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
	_, err := createRunDirectory("feature", CommandContinue, time.Now())
	if err == nil || !strings.Contains(err.Error(), "create branch log directory") {
		t.Fatalf("expected contextual branch directory error, got %v", err)
	}
}

func TestBranchDirectoryRejectsDotComponents(t *testing.T) {
	for _, branch := range []string{".", ".."} {
		t.Run(branch, func(t *testing.T) {
			workingDirectory := t.TempDir()
			t.Chdir(workingDirectory)
			path, err := createRunDirectory(branch, CommandContinue, time.Now())
			if err == nil || !strings.Contains(err.Error(), "invalid branch directory") || path != "" {
				t.Fatalf("expected contextual rejection, got path %q, error %v", path, err)
			}
			entries, err := os.ReadDir(workingDirectory)
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid branch created entries: %v, error %v", entries, err)
			}
		})
	}
}

func TestBranchDirectoryRejectsSymlinks(t *testing.T) {
	for _, target := range []string{"../outside", "other-branch", "../missing"} {
		t.Run(target, func(t *testing.T) {
			workingDirectory := t.TempDir()
			t.Chdir(workingDirectory)
			for _, directory := range []string{"logs", "outside", filepath.Join("logs", "other-branch")} {
				if err := os.MkdirAll(directory, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(target, filepath.Join("logs", "feature")); err != nil {
				t.Fatal(err)
			}
			path, err := createRunDirectory("feature", CommandContinue, time.Now())
			if err == nil || !strings.Contains(err.Error(), "create branch log directory") || path != "" {
				t.Fatalf("expected contextual symlink rejection, got path %q, error %v", path, err)
			}
			for _, directory := range []string{"outside", filepath.Join("logs", "other-branch")} {
				entries, err := os.ReadDir(directory)
				if err != nil || len(entries) != 0 {
					t.Fatalf("symlink target %s gained entries: %v, error %v", directory, entries, err)
				}
			}
			if _, err := os.Stat("missing"); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("dangling symlink target was created: %v", err)
			}
			if targetAfter, err := os.Readlink(filepath.Join("logs", "feature")); err != nil || targetAfter != target {
				t.Fatalf("branch symlink changed: target %q, error %v", targetAfter, err)
			}
		})
	}
}
