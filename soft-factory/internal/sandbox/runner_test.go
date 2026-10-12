package sandbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
)

func prepareRunnerTest(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	// NewRun's exclude lookup stays inside the fixture, without touching the
	// real repository or depending on worktrees or a live agent.
	t.Setenv("FACTORY_TEST_EXCLUDE", filepath.Join(root, "exclude"))
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\nprintf '%s\\n' \"$FACTORY_TEST_EXCLUDE\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if err := os.WriteFile("agent-sandbox.json", []byte(`{"agent":"configured-agent","model":"configured-model"}`), 0600); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestExecuteRuntimeSelection(t *testing.T) {
	const banner = "OpenAI Codex v0.100.0\n--------\nmodel: runtime-model\nprovider: openai\n--------\n"
	const init = `{"type":"system","subtype":"init","model":"fallback-model"}` + "\n"
	for _, tc := range []struct {
		name, output, agent, model string
		exit                       int
		stderr, report, noLaunch   bool
	}{
		{name: "success stdout", output: banner, agent: "codex", model: "runtime-model"},
		{name: "success stderr", output: banner, agent: "codex", model: "runtime-model", stderr: true},
		{name: "claude", output: init, agent: "claude", model: "fallback-model"},
		{name: "failed claude", output: init, agent: "claude", model: "fallback-model", exit: 1},
		{name: "interrupted claude", output: init, agent: "claude", model: "fallback-model", exit: 130},
		// Later output of the attempt (for example go test -v of the selection
		// fixtures) can repeat startup markers; they must not replace it.
		{name: "output repeats banner", output: banner + "exec\n" + strings.Replace(banner, "runtime-model", "printed-model", 1), agent: "codex", model: "runtime-model"},
		{name: "output repeats init", output: banner + init, agent: "codex", model: "runtime-model"},
		{name: "stderr output repeats init", output: banner + init, agent: "codex", model: "runtime-model", stderr: true},
		{name: "failed output repeats init", output: banner + init, agent: "codex", model: "runtime-model", exit: 1},
		{name: "claude output repeats banner", output: init + banner, agent: "claude", model: "fallback-model"},
		{name: "missing metadata", output: "Agent: configured-agent\nModel: configured-model\n", agent: "unavailable", model: "unavailable"},
		{name: "missing model", output: `{"type":"system","subtype":"init"}`, agent: "claude", model: "unavailable"},
		{name: "failed without metadata", agent: "unavailable", model: "unavailable", exit: 1},
		{name: "failure before launch", agent: "unavailable", model: "unavailable", noLaunch: true},
		{name: "combined report", output: banner + init, agent: "codex", model: "runtime-model", report: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := prepareRunnerTest(t)
			if !tc.noLaunch {
				script := "#!/bin/sh\ncat <<'FIXTURE'"
				if tc.stderr {
					script += " >&2"
				}
				// cat is deliberately resolved by absolute path; PATH contains
				// only our fixture executables.
				script = strings.Replace(script, "\ncat ", "\n/bin/cat ", 1)
				script += "\n" + tc.output + "\nFIXTURE\n"
				if tc.exit == 130 {
					script += "exit 130\n"
				} else if tc.exit == 1 {
					script += "exit 1\n"
				}
				if err := os.WriteFile(filepath.Join(bin, "agent-sandbox"), []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
			}
			run, err := executionlog.NewRun("fixture", executionlog.CommandRun, []string{"implementation"})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { run.Finish(nil) })
			run.StartStage("implementation")
			options := ExecuteOptions{
				Stage: config.StageImplementation, StageConfig: config.StageConfig{Agent: "override-agent", Model: "override-model"},
				Args: []string{"run"}, Log: run,
			}
			if tc.report {
				options.ReportName = "fixture-report"
			}
			report, runErr := execute(options)
			if (runErr != nil) != (tc.exit != 0 || tc.noLaunch) {
				t.Fatalf("execute error = %v", runErr)
			}
			if tc.exit == 130 && !errors.Is(runErr, context.Canceled) {
				t.Fatalf("expected interruption, got %v", runErr)
			}
			run.FinishStage(runErr)
			if err := run.RecordStageResult("implementation", time.Second, runErr); err != nil {
				t.Fatal(err)
			}
			// A subsequent stage without metadata must not inherit this stage.
			run.StartStage("code-review")
			run.FinishStage(nil)
			if err := run.RecordStageResult("code-review", time.Second, nil); err != nil {
				t.Fatal(err)
			}
			run.Finish(nil)
			data, err := os.ReadFile(filepath.Join(run.Path(), "summary.log"))
			if err != nil {
				t.Fatal(err)
			}
			status := "SUCCEEDED"
			if runErr != nil {
				status = "FAILED"
			}
			if tc.exit == 130 {
				status = "INTERRUPTED"
			}
			want := "1. IMPLEMENTATION\n   Status:    " + status + "\n   Agent:     " + tc.agent + "\n   Model:     " + tc.model + "\n   Duration:  1 sec\n   Changes:   Change summary unavailable.\n"
			if !strings.Contains(string(data), want) || strings.Count(string(data), "   Agent:") != 2 || strings.Count(string(data), "   Model:") != 2 {
				t.Fatalf("unexpected summary: %s", data)
			}
			if !strings.Contains(string(data), "2. CODE REVIEW\n   Status:    SUCCEEDED\n   Agent:     unavailable\n   Model:     unavailable\n") {
				t.Fatalf("stage metadata leaked: %s", data)
			}
			for _, forbidden := range []string{"configured", "override", "provider", "attempt", "retry", "fallback", "source"} {
				// The final model name can contain "fallback", so inspect only
				// the remainder after removing the expected selection fields.
				remaining := strings.ReplaceAll(string(data), "   Model:     "+tc.model, "")
				if strings.Contains(remaining, forbidden) {
					t.Fatalf("summary contains %q: %s", forbidden, data)
				}
			}
			if tc.output != "" {
				stageData, err := os.ReadFile(filepath.Join(run.Path(), "01-implementation.log"))
				if err != nil || !strings.Contains(string(stageData), tc.output) {
					t.Fatalf("runtime output changed: %q, %v", stageData, err)
				}
			}
			if tc.report {
				reportData, err := os.ReadFile(report)
				if err != nil || string(reportData) != tc.output+"\n" {
					t.Fatalf("combined report changed: %q, %v", reportData, err)
				}
			}
		})
	}
}

// Each agent-sandbox invocation is one attempt with a fresh startup record, so
// a later invocation in the same stage (a retry or fallback) still replaces the
// earlier selection, including with unknown values.
func TestExecuteLaterAttemptReplacesSelection(t *testing.T) {
	const codex = "OpenAI Codex v0.100.0\n--------\nmodel: first-model\n--------\n"
	for _, tc := range []struct {
		name, final, agent, model string
	}{
		{name: "retry with unchanged selection", final: codex, agent: "codex", model: "first-model"},
		{name: "fallback with changed selection", final: `{"type":"system","subtype":"init","model":"claude-final"}` + "\n", agent: "claude", model: "claude-final"},
		{name: "fallback with unknown model", final: `{"type":"system","subtype":"init"}` + "\nOpenAI Codex v0.100.0\n--------\nmodel: printed-model\n--------\n", agent: "claude", model: "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := prepareRunnerTest(t)
			run, err := executionlog.NewRun("fixture", executionlog.CommandRun, []string{"implementation"})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { run.Finish(nil) })
			run.StartStage("implementation")
			for _, output := range []string{codex, tc.final} {
				script := "#!/bin/sh\n/bin/cat <<'FIXTURE'\n" + output + "FIXTURE\n"
				if err := os.WriteFile(filepath.Join(bin, "agent-sandbox"), []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
				if _, err := execute(ExecuteOptions{Stage: config.StageImplementation, Args: []string{"run"}, Log: run}); err != nil {
					t.Fatal(err)
				}
			}
			run.FinishStage(nil)
			if err := run.RecordStageResult("implementation", time.Second, nil); err != nil {
				t.Fatal(err)
			}
			run.Finish(nil)
			data, err := os.ReadFile(filepath.Join(run.Path(), "summary.log"))
			if err != nil {
				t.Fatal(err)
			}
			summary := string(data)
			want := "1. IMPLEMENTATION\n   Status:    SUCCEEDED\n   Agent:     " + tc.agent + "\n   Model:     " + tc.model + "\n   Duration:  1 sec\n"
			if !strings.Contains(summary, want) || strings.Count(summary, "   Agent:") != 1 || strings.Count(summary, "   Model:") != 1 {
				t.Fatalf("final attempt not reported once: %s", summary)
			}
			// No attempt history: the earlier attempt's values, printed markers,
			// and attempt annotations stay out of the summary.
			remaining := strings.Replace(summary, want, "", 1)
			for _, forbidden := range []string{"codex", "first-model", "printed-model", "claude", "attempt", "retry", "fallback"} {
				if strings.Contains(remaining, forbidden) {
					t.Fatalf("summary contains %q outside the final selection: %s", forbidden, summary)
				}
			}
		})
	}
}
