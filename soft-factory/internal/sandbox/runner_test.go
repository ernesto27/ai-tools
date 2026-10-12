package sandbox

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
)

// The plain sandbox CLI has no selection metadata channel. Controlled children
// verify that neither configuration nor arbitrary output supplies one.
func TestExecuteWithoutRuntimeSelection(t *testing.T) {
	for _, tc := range []struct {
		name       string
		exit       string
		missing    bool
		canceled   bool
		wantStatus string
	}{
		{"success", "0", false, false, "SUCCEEDED"},
		{"failure", "7", false, false, "FAILED"},
		{"interrupted", "130", false, false, "INTERRUPTED"},
		{"missing executable", "", true, false, "FAILED"},
		{"canceled before launch", "0", false, true, "INTERRUPTED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := prepareRunnerLog(t)
			bin := t.TempDir()
			if !tc.missing {
				fixture := "#!/bin/sh\nprintf 'Agent: forged-agent\\nModel: forged-model\\n'\nprintf 'Agent: forged-stderr\\nModel: forged-stderr-model\\n' >&2\nexit " + tc.exit + "\n"
				if err := os.WriteFile(filepath.Join(bin, "agent-sandbox"), []byte(fixture), 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin)
			if err := os.WriteFile("agent-sandbox.json", []byte(`{"agent":"configured-agent","model":"configured-model"}`), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.canceled {
				cancel()
			}
			stages := []struct {
				name  string
				mode  string
				stage config.Stage
			}{
				{"implementation", "run", config.StageImplementation},
				{"implementation", "resume", config.StageImplementation},
				{"code-review", "resume", config.StageCodeReview},
				{"security-review", "resume", config.StageSecurityReview},
				{"risk-classification", "resume", config.StageRiskClassification},
			}
			for _, stage := range stages {
				run.StartStage(stage.name)
				_, err := execute(ExecuteOptions{
					Args: []string{stage.mode}, Context: ctx, Log: run, Stage: stage.stage,
					StageConfig: config.StageConfig{Agent: "override-agent", Model: "override-model"},
				})
				if (err == nil) != (tc.wantStatus == "SUCCEEDED") {
					t.Fatalf("execute error = %v, want status %s", err, tc.wantStatus)
				}
				if tc.wantStatus == "INTERRUPTED" && !errors.Is(err, context.Canceled) {
					t.Fatalf("expected interruption, got %v", err)
				}
				run.FinishStage(err)
				if err := run.RecordStageResult(stage.name, time.Second, err); err != nil {
					t.Fatal(err)
				}
			}
			run.Finish(nil)
			data, err := os.ReadFile(filepath.Join(run.Path(), "summary.log"))
			if err != nil {
				t.Fatal(err)
			}
			want := "   Status:    " + tc.wantStatus + "\n   Agent:     unavailable\n   Model:     unavailable\n   Duration:  1 sec\n"
			if strings.Count(string(data), want) != len(stages) {
				t.Fatalf("unexpected summary:\n%s", data)
			}
			for _, forbidden := range []string{"configured-agent", "configured-model", "override-agent", "override-model", "forged-", "Provider:", "(configured:", "Attempt:", "Retry:", "Fallback:"} {
				if strings.Contains(string(data), forbidden) {
					t.Fatalf("summary contains %q:\n%s", forbidden, data)
				}
			}
		})
	}
}

func TestExecuteInvalidatesEarlierSelectionOnlyAfterStart(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "started", true: "not started"}[missing], func(t *testing.T) {
			run := prepareRunnerLog(t)
			bin := t.TempDir()
			if !missing {
				if err := os.WriteFile(filepath.Join(bin, "agent-sandbox"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin)
			run.StartStage("implementation")
			run.RecordStageSelection("earlier-agent", "earlier-model")
			_, err := execute(ExecuteOptions{Args: []string{"run"}, Log: run})
			if (err != nil) != missing {
				t.Fatalf("execute error = %v, missing = %v", err, missing)
			}
			run.FinishStage(err)
			if err := run.RecordStageResult("implementation", time.Second, err); err != nil {
				t.Fatal(err)
			}
			run.Finish(nil)
			data, err := os.ReadFile(filepath.Join(run.Path(), "summary.log"))
			if err != nil {
				t.Fatal(err)
			}
			want := "   Agent:     unavailable\n   Model:     unavailable\n"
			if missing {
				want = "   Agent:     earlier-agent\n   Model:     earlier-model\n"
			}
			if !strings.Contains(string(data), want) {
				t.Fatalf("selection not tied to last started attempt:\n%s", data)
			}
		})
	}
}

func prepareRunnerLog(t *testing.T) *executionlog.Run {
	t.Helper()
	t.Chdir(t.TempDir())
	if output, err := exec.Command("git", "init", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("initialize fixture repository: %v\n%s", err, output)
	}
	run, err := executionlog.NewRun("feature", executionlog.CommandRun, []string{"implementation", "code-review", "security-review", "risk-classification"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { run.Finish(nil) })
	return run
}
