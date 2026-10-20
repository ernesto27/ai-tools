package executionlog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"soft-factory/internal/config"
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
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSummaryLifecycle(t *testing.T) {
	prepareLogTestDirectory(t)
	run, err := NewRun("feature", CommandRun, config.Stages)
	require.NoError(t, err)
	t.Cleanup(func() { run.Finish(nil) })

	summaryPath := filepath.Join(run.Path(), "summary.log")
	checkContents := func(want string) {
		t.Helper()
		data, err := os.ReadFile(summaryPath)
		require.NoError(t, err)
		require.Equal(t, want, string(data))
	}

	want := "SOFTWARE FACTORY — RUN SUMMARY\n==================================================\n\n"
	checkContents(want)

	entries := []struct {
		stage   config.Stage
		elapsed time.Duration
		err     error
		line    string
	}{
		{"implementation", 60855 * time.Millisecond, nil, "1. IMPLEMENTATION\n   Status:    SUCCEEDED\n   Agent:     \n   Model:     \n   Duration:  1 min 1 sec\n   Tokens usage:  unavailable\n   Changes:   Change summary unavailable.\n\n"},
		{"codeReview", 135183 * time.Millisecond, nil, "2. CODE REVIEW\n   Status:    SUCCEEDED\n   Agent:     \n   Model:     \n   Duration:  2 min 15 sec\n   Tokens usage:  unavailable\n   Changes:   Change summary unavailable.\n\n"},
		{"codeReview", 18 * time.Second, errors.New("execute agent-sandbox: exit status 1\nverification failed"), "3. CODE REVIEW\n   Status:    FAILED\n   Agent:     \n   Model:     \n   Duration:  18 sec\n   Tokens usage:  unavailable\n   Changes:   Change summary unavailable.\n   Error:     execute agent-sandbox: exit status 1\n              verification failed\n\n"},
		{"securityReview", time.Second, fmt.Errorf("execute agent-sandbox: %w", context.Canceled), "4. SECURITY REVIEW\n   Status:    INTERRUPTED\n   Agent:     \n   Model:     \n   Duration:  1 sec\n   Tokens usage:  unavailable\n   Changes:   Change summary unavailable.\n   Error:     execute agent-sandbox: context canceled\n\n"},
	}
	for _, entry := range entries {
		require.NoError(t, run.RecordStageResult(entry.stage, entry.elapsed, entry.err))
		want += entry.line
		checkContents(want)
	}

	file := run.summary
	run.Finish(nil)
	run.Finish(nil) // Finishing twice must be safe.
	want += "==================================================\n"

	_, err = file.WriteString("unexpected\n")
	require.ErrorIs(t, err, os.ErrClosed)

	require.Error(t, run.RecordStageResult("securityReview", time.Second, nil))
	checkContents(want)
}

func TestFormatTokenCount(t *testing.T) {
	for _, tc := range []struct {
		total int64
		want  string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1K"},
		{43834, "44K"},
		{474763, "475K"},
		{999000, "999K"},
		{1_000_000, "1M"},
		{1_400_000, "1M"},
		{1_600_000, "2M"},
		{475_000_000, "475M"},
	} {
		got := formatTokenCount(tc.total)
		assert.Equal(t, tc.want, got, "total %d", tc.total)
	}
}

func TestSummaryStageTokenUsage(t *testing.T) {
	prepareLogTestDirectory(t)
	run, err := NewRun("feature", CommandRun, config.Stages)
	require.NoError(t, err)
	defer run.Finish(nil)
	for _, tc := range []struct {
		agent, output string
	}{
		{"codex", `{"type":"usage","agent":"codex","usage":{"total_tokens":43834}}` + "\nFooter\n"},
		{"codex", `{"type":"usage","agent":"codex","usage":{"total_tokens":12}}` + "\nFooter\n"},
		{"claude", `{"type":"usage","agent":"claude","usage":{"total_tokens":73447}}` + "\nFooter\n"},
		{"claude", "Interrupted without usage\n"},
	} {
		run.StartStage(config.StageImplementation)
		run.Selection(tc.agent, "test-model")

		_, err := io.WriteString(run.Stream("stdout"), tc.output)
		require.NoError(t, err)

		run.FinishStage(nil)
		require.NoError(t, run.RecordStageResult(config.StageImplementation, time.Second, nil))
	}
	data, err := os.ReadFile(filepath.Join(run.Path(), summaryFile))
	require.NoError(t, err)
	for _, want := range []string{"44K", "12", "73K", "unavailable"} {
		require.Contains(t, string(data), "   Tokens usage:  "+want+"\n   Changes:")
	}
}

func TestSummarySelectionLifecycle(t *testing.T) {
	prepareLogTestDirectory(t)
	run, err := NewRun("feature", CommandContinue, config.Stages)
	require.NoError(t, err)
	defer run.Finish(nil)
	for i, tt := range []struct {
		stage        config.Stage
		agent, model string
		err          error
	}{
		{"implementation", "codex", "final-model", nil},
		{"codeReview", "", "", errors.New("failed before launch")},
		{"securityReview", "claude", "", context.Canceled},
		{"riskClassification", "", "configured-model", nil},
	} {
		run.StartStage(tt.stage)
		// Detailed provenance is separate from the plain summary names.
		run.Details("details-agent", "details-model", "details-provider")
		if tt.agent != "" || tt.model != "" {
			run.Selection("earlier-agent", "earlier-model")
			run.Selection(tt.agent, tt.model)
		}
		run.FinishStage(tt.err)
		require.NoError(t, run.RecordStageResult(tt.stage, time.Second, tt.err))
		data, err := os.ReadFile(filepath.Join(run.Path(), summaryFile))
		require.NoError(t, err)
		entry := fmt.Sprintf("%d. %s\n   Status:    %s\n   Agent:     %s\n   Model:     %s\n   Duration:  1 sec\n",
			i+1, stageHeading(tt.stage), outcome(tt.err), tt.agent, tt.model)
		require.Contains(t, string(data), entry)
		require.NotContains(t, string(data), "earlier-")
		require.NotContains(t, string(data), "details-")
	}
	require.NoError(t, run.RecordSkippedStage("Skipping disabled stage: codeReview"))
	data, err := os.ReadFile(filepath.Join(run.Path(), summaryFile))
	require.NoError(t, err)
	require.Equal(t, 4, strings.Count(string(data), "   Agent:"))
	require.Equal(t, 4, strings.Count(string(data), "   Model:"))
	require.True(t, strings.HasSuffix(string(data), "Skipping disabled stage: codeReview\n\n"))
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
			run, err := NewRun("feature", CommandRun, config.Stages)
			require.NoError(t, err)
			t.Cleanup(func() { run.Finish(nil) })
			report := filepath.Join(run.Path(), "changes.txt")
			if !tc.missing {
				require.NoError(t, os.WriteFile(report, []byte(tc.content), 0600))
			}
			run.SetChangeSummaryPath(report)
			require.NoError(t, run.RecordStageResult("implementation", time.Second, nil))
			data, err := os.ReadFile(filepath.Join(run.Path(), summaryFile))
			require.NoError(t, err)
			require.Contains(t, string(data), "   Changes:   "+tc.want+"\n")

			_, err = os.Stat(report)
			require.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}

func TestStageChangeReportRetainedOnSummaryWriteFailure(t *testing.T) {
	prepareLogTestDirectory(t)
	run, err := NewRun("feature", CommandRun, config.Stages)
	require.NoError(t, err)
	t.Cleanup(func() { run.Finish(nil) })
	report := filepath.Join(run.Path(), "changes.txt")
	require.NoError(t, os.WriteFile(report, []byte("Added tracking."), 0600))
	run.SetChangeSummaryPath(report)
	require.NoError(t, run.summary.Close())
	require.Error(t, run.RecordStageResult("implementation", time.Second, nil))

	_, err = os.Stat(report)
	require.NoError(t, err)
}

func TestStageChangeReportResetAndSymlinkRejected(t *testing.T) {
	prepareLogTestDirectory(t)
	run, err := NewRun("feature", CommandRun, config.Stages)
	require.NoError(t, err)
	t.Cleanup(func() { run.Finish(nil) })
	run.StartStage("implementation")
	report := filepath.Join(run.Path(), "changes.txt")
	require.NoError(t, os.WriteFile(report, []byte("Earlier stage changes."), 0600))
	run.SetChangeSummaryPath(report)
	run.FinishStage(nil)
	run.StartStage("codeReview")
	require.Empty(t, run.changeSummaryPath)
	link := filepath.Join(run.Path(), "report-link.txt")
	require.NoError(t, os.Symlink(report, link))
	run.SetChangeSummaryPath(link)
	require.NoError(t, run.RecordStageResult("codeReview", time.Second, nil))
	data, err := os.ReadFile(filepath.Join(run.Path(), summaryFile))
	require.NoError(t, err)
	require.NotContains(t, string(data), "Earlier stage changes")
}

func TestHeaderListsOnlyPlannedStages(t *testing.T) {
	for _, tc := range []struct {
		name    string
		planned []config.Stage
		want    string
	}{
		{"disabled stage omitted", []config.Stage{config.StageImplementation, config.StageCodeReview, config.StageRiskClassification}, "implementation, codeReview, riskClassification"},
		{"none planned", nil, "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepareLogTestDirectory(t)
			run, err := NewRun("feature", CommandRun, tc.planned)
			require.NoError(t, err)
			run.StartStage("implementation")
			run.FinishStage(nil)
			run.Finish(nil)
			data, err := os.ReadFile(filepath.Join(run.Path(), "01-implementation.log"))
			require.NoError(t, err)
			require.Contains(t, string(data), "\nPlanned stages: "+tc.want+"\n")
		})
	}
}

func prepareLogTestDirectory(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())

	output, err := exec.Command("git", "init", "-b", "main").CombinedOutput()
	require.NoError(t, err, "%s", output)
}

func TestInvocationDirectoryAllocation(t *testing.T) {
	for _, firstCommand := range []string{CommandRun, CommandContinue} {
		t.Run(firstCommand+" first", func(t *testing.T) {
			t.Chdir(t.TempDir())
			// Earlier flat logs must remain untouched.
			legacy := filepath.Join("logs", "feature-logs-2026-10-08_19-38-48.007046183")
			require.NoError(t, os.MkdirAll(legacy, 0700))
			legacyFile := filepath.Join(legacy, summaryFile)
			require.NoError(t, os.WriteFile(legacyFile, []byte("earlier summary"), 0600))
			// A non-UTC input also produces the existing UTC timestamp format.
			started := time.Date(2026, 10, 8, 21, 38, 48, 7046183, time.FixedZone("UTC+2", 2*60*60))
			parent := filepath.Join("logs", "feature-logs")
			name := firstCommand + "-2026-10-08_19-38-48.007046183"
			for i := 1; i <= 3; i++ {
				path, err := createRunDirectory("feature/logs", firstCommand, started)
				require.NoError(t, err)
				want := filepath.Join(parent, name)
				if i > 1 {
					want += fmt.Sprintf("-%d", i)
				}
				require.Equal(t, want, path)
				require.False(t, filepath.IsAbs(path))
				require.NoError(t, os.WriteFile(filepath.Join(path, summaryFile), []byte(path), 0600))
			}
			otherCommand := CommandContinue
			if firstCommand == CommandContinue {
				otherCommand = CommandRun
			}
			otherPath, err := createRunDirectory("feature/logs", otherCommand, started)
			require.NoError(t, err)
			require.Equal(t, filepath.Join(parent, otherCommand+"-2026-10-08_19-38-48.007046183"), otherPath)
			children, err := os.ReadDir(parent)
			require.NoError(t, err)
			require.Len(t, children, 4)
			for _, child := range children {
				path := filepath.Join(parent, child.Name())
				if path == otherPath {
					continue
				}
				data, err := os.ReadFile(filepath.Join(path, summaryFile))
				require.NoError(t, err)
				require.Equal(t, path, string(data))
			}
			data, err := os.ReadFile(legacyFile)
			require.NoError(t, err)
			require.Equal(t, "earlier summary", string(data))
			for _, path := range []string{"logs", parent, otherPath} {
				info, err := os.Stat(path)
				require.NoError(t, err)
				assert.Equal(t, os.FileMode(0700), info.Mode().Perm(), "path %s", path)
			}
		})
	}
}

func TestInvocationArtifactsAreIsolated(t *testing.T) {
	prepareLogTestDirectory(t)
	// Compare summary permissions to the prior os.Create behavior, including umask.
	reference, err := os.Create(filepath.Join(t.TempDir(), summaryFile))
	require.NoError(t, err)
	referenceInfo, statErr := reference.Stat()
	require.NoError(t, errors.Join(statErr, reference.Close()))
	parent := filepath.Join("logs", "Feature-logs._-9-")
	paths := make(map[string]int)
	for i, command := range []string{CommandRun, CommandContinue, CommandContinue, CommandRun} {
		run, err := NewRun("Feature/logs._-9é", command, []config.Stage{config.StageImplementation})
		require.NoError(t, err)
		t.Cleanup(func() { run.Finish(nil) })
		require.Equal(t, parent, filepath.Dir(run.Path()))
		require.True(t, strings.HasPrefix(filepath.Base(run.Path()), command+"-"))

		_, exists := paths[run.Path()]
		require.False(t, exists)

		paths[run.Path()] = i
		run.StartStage("implementation")
		fmt.Fprintf(run.Stream("stdout"), "invocation %d\n", i)
		run.FinishStage(nil)
		require.NoError(t, run.RecordStageResult("implementation", time.Second, nil))
		run.Finish(nil)
	}
	children, err := os.ReadDir(parent)
	require.NoError(t, err)
	require.Len(t, children, 4)
	for path, invocation := range paths {
		entries, err := os.ReadDir(path)
		require.NoError(t, err)
		require.Len(t, entries, 3)
		data, err := os.ReadFile(filepath.Join(path, "01-implementation.log"))
		require.NoError(t, err)
		require.Equal(t, 1, strings.Count(string(data), "invocation "))
		require.Contains(t, string(data), fmt.Sprintf("invocation %d\n", invocation))
		for _, entry := range entries {
			info, err := entry.Info()
			require.NoError(t, err)
			want := os.FileMode(0600)
			if entry.Name() == summaryFile {
				// The summary retains os.Create's existing permissions.
				want = referenceInfo.Mode().Perm()
			}
			assert.Equal(t, want, info.Mode().Perm(), "entry %s", entry.Name())
		}
	}
}

func TestBranchDirectoryCreationFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.Mkdir("logs", 0700))
	require.NoError(t, os.WriteFile(filepath.Join("logs", "feature"), []byte("existing file"), 0600))
	_, err := createRunDirectory("feature", CommandContinue, time.Now())
	require.ErrorContains(t, err, "create branch log directory")
}

func TestBranchDirectoryRejectsDotComponents(t *testing.T) {
	for _, branch := range []string{".", ".."} {
		t.Run(branch, func(t *testing.T) {
			workingDirectory := t.TempDir()
			t.Chdir(workingDirectory)
			path, err := createRunDirectory(branch, CommandContinue, time.Now())
			require.ErrorContains(t, err, "invalid branch directory")
			require.Empty(t, path)
			entries, err := os.ReadDir(workingDirectory)
			require.NoError(t, err)
			require.Len(t, entries, 0)
		})
	}
}

func TestBranchDirectoryRejectsSymlinks(t *testing.T) {
	for _, target := range []string{"../outside", "other-branch", "../missing"} {
		t.Run(target, func(t *testing.T) {
			workingDirectory := t.TempDir()
			t.Chdir(workingDirectory)
			for _, directory := range []string{"logs", "outside", filepath.Join("logs", "other-branch")} {
				require.NoError(t, os.MkdirAll(directory, 0700))
			}
			require.NoError(t, os.Symlink(target, filepath.Join("logs", "feature")))
			path, err := createRunDirectory("feature", CommandContinue, time.Now())
			require.ErrorContains(t, err, "create branch log directory")
			require.Empty(t, path)
			for _, directory := range []string{"outside", filepath.Join("logs", "other-branch")} {
				entries, err := os.ReadDir(directory)
				require.NoError(t, err)
				require.Len(t, entries, 0)
			}

			_, err = os.Stat("missing")
			require.ErrorIs(t, err, os.ErrNotExist)

			targetAfter, err := os.Readlink(filepath.Join("logs", "feature"))
			require.NoError(t, err)
			require.Equal(t, target, targetAfter)
		})
	}
}
