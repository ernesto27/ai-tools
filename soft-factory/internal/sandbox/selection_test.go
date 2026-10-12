package sandbox

import (
	"bytes"
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

const codexStartup = "OpenAI Codex v0.100.0\r\n--------\r\nworkdir: /workspace\r\nmodel: runtime-model\r\nprovider: openai\r\n--------\r\nuser\r\n"
const claudeStartup = "{\n  \"type\": \"system\",\n  \"subtype\": \"init\",\n  \"model\": \"claude-runtime\"\n}\n"

// Startup lookalikes an agent or its commands could print after launch.
const spoofedCodex = "OpenAI Codex v0.200.0\r\n--------\r\nmodel: other-model\r\n--------\r\n"
const spoofedClaude = "{\n  \"type\": \"system\",\n  \"subtype\": \"init\",\n  \"model\": \"spoofed\"\n}\n"

func newSelectionLog(t *testing.T) *executionlog.Run {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	// Use a controlled Git fixture; the log's ignore rule never touches the
	// real repository, and these tests need no live agents or worktrees.
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\nprintf '%s\\n' 'test-exclude'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	run, err := executionlog.NewRun("feature", executionlog.CommandRun, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { run.Finish(nil) })
	return run
}

func assertSelectionSummary(t *testing.T, run *executionlog.Run, agent, model, status string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(run.Path(), "summary.log"))
	if err != nil {
		t.Fatal(err)
	}
	want := "1. IMPLEMENTATION\n   Status:    " + status + "\n   Agent:     " + agent + "\n   Model:     " + model + "\n   Duration:  1 sec\n   Changes:   Change summary unavailable.\n"
	if !strings.Contains(string(data), want) || strings.Count(string(data), "Agent:") != 1 || strings.Count(string(data), "Model:") != 1 {
		t.Fatalf("unexpected summary:\n%s\nwant entry:\n%s", data, want)
	}
	for _, forbidden := range []string{"configured:", "provider:", "attempt", "retry", "fallback"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("summary contains %q: %s", forbidden, data)
		}
	}
}

func TestRuntimeSelectionRecords(t *testing.T) {
	for _, tt := range []struct {
		name, output, agent, model string
	}{
		{"normal", codexStartup, "codex", "runtime-model"},
		{"claude", claudeStartup, "claude", "claude-runtime"},
		{"missing", "Agent: configured-agent\nModel: configured-model\n", "unavailable", "unavailable"},
		{"not a version", "OpenAI Codex version guessed\nmodel: guessed\n", "unavailable", "unavailable"},
		{"multiline value", `{"type":"system","subtype":"init","model":"name\n   Model: injected"}`, "claude", "unavailable"},
		{"missing model", `{"type":"system","subtype":"init"}`, "claude", "unavailable"},
		{"malformed model", `{"type":"system","subtype":"init","model":42}`, "claude", "unavailable"},
		{"partial header", "OpenAI Codex v0.101.0", "codex", "unavailable"},
		{"header without model", "OpenAI Codex v0.101.0\r\n--------\r\nworkdir: /workspace\r\n--------\r\nmodel: authored-model\r\n", "codex", "unavailable"},
		{"partial init", "{\n  \"type\": \"system\",\n  \"subtype\": \"init\",\n", "claude", "unavailable"},
		{"truncated init", "{\n  \"type\": \"system\",\n  \"subtype\": \"init\",\n" + spoofedClaude, "claude", "unavailable"},
		{"authored text", `{"type":"assistant","message":{"content":"Model: authored-model","model":"authored-model"}}`, "unavailable", "unavailable"},
		{"header ended", codexStartup + "model: authored-model\n", "codex", "runtime-model"},
		{"oversize", strings.Repeat("x", selectionBufferLimit+1) + "\n" + claudeStartup, "claude", "claude-runtime"},
		{"terminal escapes", "\x1b[32m" + codexStartup + "\x1b[0m", "codex", "runtime-model"},
		// One sandbox run launches one agent; startup lookalikes printed
		// afterwards by the agent or its commands never change the selection.
		{"repeated codex header", codexStartup + codexStartup, "codex", "runtime-model"},
		{"codex then lookalikes", codexStartup + spoofedCodex + spoofedClaude + spoofedCodex, "codex", "runtime-model"},
		{"claude then lookalikes", claudeStartup + spoofedClaude + spoofedCodex + spoofedClaude, "claude", "claude-runtime"},
		{"init inside codex header", "OpenAI Codex v0.100.0\r\n--------\r\n" + spoofedClaude + "model: runtime-model\r\n" + spoofedCodex, "codex", "runtime-model"},
		{"version inside codex header", "OpenAI Codex v0.100.0\r\n--------\r\nOpenAI Codex v0.200.0\r\nmodel: runtime-model\r\n" + spoofedCodex, "codex", "runtime-model"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// An undetermined launched agent accepts either record kind.
			assertObservedSelection(t, "", tt.output, tt.agent, tt.model)
		})
	}
}

func TestRuntimeSelectionLaunchedAgent(t *testing.T) {
	// A long header that never closes and never reaches its model line.
	openHeader := "OpenAI Codex v0.100.0\r\n--------\r\n" + strings.Repeat("note: agent output\r\n", codexHeaderLimit) + "model: authored-model\r\n"
	for _, tt := range []struct {
		name, launched, output, agent, model string
	}{
		{"opencode lookalikes", "opencode", spoofedCodex + spoofedClaude, "unavailable", "unavailable"},
		{"pi lookalikes", "pi", spoofedClaude + spoofedCodex, "unavailable", "unavailable"},
		{"codex ignores claude init", "codex", spoofedClaude + codexStartup, "codex", "runtime-model"},
		{"codex ignores only claude init", "codex", spoofedClaude, "unavailable", "unavailable"},
		{"claude ignores codex header", "claude", spoofedCodex + claudeStartup, "claude", "claude-runtime"},
		{"claude ignores only codex header", "claude", spoofedCodex, "unavailable", "unavailable"},
		{"undetermined codex", "", codexStartup + spoofedClaude, "codex", "runtime-model"},
		{"undetermined claude", "", claudeStartup + spoofedCodex, "claude", "claude-runtime"},
		{"header never closes", "codex", openHeader, "codex", "unavailable"},
		{"header model at limit", "codex", "OpenAI Codex v0.100.0\r\n--------\r\n" + strings.Repeat("note: x\r\n", codexHeaderLimit-2) + "model: runtime-model\r\n", "codex", "runtime-model"},
		{"osc title before version", "codex", "\x1b]0;codex\x07" + codexStartup, "codex", "runtime-model"},
		{"osc st before version", "", "\x1b]2;title\x1b\\" + codexStartup, "codex", "runtime-model"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assertObservedSelection(t, tt.launched, tt.output, tt.agent, tt.model)
		})
	}
}

func assertObservedSelection(t *testing.T, launched, output, agent, model string) {
	t.Helper()
	run := newSelectionLog(t)
	run.StartStage("implementation")
	var out bytes.Buffer
	w := newSelectionWriter(&out, run, launched)
	// Exercise arbitrary pipe boundaries, including every JSON token.
	for _, b := range []byte(output) {
		if n, err := w.Write([]byte{b}); n != 1 || err != nil {
			t.Fatalf("write: %d, %v", n, err)
		}
	}
	w.flush()
	if out.String() != output {
		t.Fatal("selection writer altered passed-through output")
	}
	run.FinishStage(nil)
	if err := run.RecordStageResult("implementation", time.Second, nil); err != nil {
		t.Fatal(err)
	}
	assertSelectionSummary(t, run, agent, model, "SUCCEEDED")
}

func TestExecuteSelection(t *testing.T) {
	for _, tt := range []struct {
		name, configured, override, script, agent, model, status string
		wantErr                                                  bool
	}{
		{"success", "codex", "", "printf '%s' '" + codexStartup + "'\n", "codex", "runtime-model", "SUCCEEDED", false},
		{"failed", "codex", "", "printf '%s' '" + codexStartup + "'\nexit 1\n", "codex", "runtime-model", "FAILED", true},
		{"agent-printed lookalike", "codex", "", "printf '%s' '" + codexStartup + spoofedClaude + spoofedCodex + "'\n", "codex", "runtime-model", "SUCCEEDED", false},
		{"interrupted", "claude", "", "printf '%s' '" + claudeStartup + "'\nexit 130\n", "claude", "claude-runtime", "INTERRUPTED", true},
		{"missing", "codex", "", "printf '%s\\n' 'successful agent output'\n", "unavailable", "unavailable", "SUCCEEDED", false},
		{"before launch", "codex", "", "", "unavailable", "unavailable", "FAILED", true},
		{"other agent lookalike", "opencode", "", "printf '%s' '" + spoofedCodex + spoofedClaude + "'\n", "unavailable", "unavailable", "SUCCEEDED", false},
		{"stage override gates record", "codex", "claude", "printf '%s' '" + spoofedCodex + claudeStartup + "'\n", "claude", "claude-runtime", "SUCCEEDED", false},
		{"stage override to other agent", "codex", "pi", "printf '%s' '" + codexStartup + "'\n", "unavailable", "unavailable", "SUCCEEDED", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			run := newSelectionLog(t)
			if tt.script != "" {
				if err := os.WriteFile(filepath.Join(os.Getenv("PATH"), "agent-sandbox"), []byte("#!/bin/sh\n"+tt.script), 0700); err != nil {
					t.Fatal(err)
				}
			}
			// The configured agent only gates which record kind is accepted;
			// the configured model intentionally disagrees with runtime records.
			if err := os.WriteFile("agent-sandbox.json", []byte(`{"agent":"`+tt.configured+`","model":"configured-model"}`), 0600); err != nil {
				t.Fatal(err)
			}
			run.StartStage("implementation")
			_, err := execute(ExecuteOptions{Args: []string{"run"}, Context: context.Background(), Log: run, Stage: config.StageImplementation,
				StageConfig: config.StageConfig{Agent: tt.override}})
			if (err != nil) != tt.wantErr {
				t.Fatalf("execute error: %v", err)
			}
			if tt.status == "INTERRUPTED" && !errors.Is(err, context.Canceled) {
				t.Fatalf("expected interruption: %v", err)
			}
			run.FinishStage(err)
			if logErr := run.RecordStageResult("implementation", time.Second, err); logErr != nil {
				t.Fatal(logErr)
			}
			assertSelectionSummary(t, run, tt.agent, tt.model, tt.status)
		})
	}
}

func TestExecuteSelectionLaterAttempt(t *testing.T) {
	run := newSelectionLog(t)
	run.StartStage("implementation")
	// Each execute call is a separate sandbox run; the later attempt's
	// startup record replaces the earlier one, even when the agent changes.
	for _, script := range []string{
		"printf '%s' '" + codexStartup + "'\nexit 1\n",
		"printf '%s' '" + claudeStartup + "'\n",
	} {
		if err := os.WriteFile(filepath.Join(os.Getenv("PATH"), "agent-sandbox"), []byte("#!/bin/sh\n"+script), 0700); err != nil {
			t.Fatal(err)
		}
		_, _ = execute(ExecuteOptions{Args: []string{"run"}, Context: context.Background(), Log: run, Stage: config.StageImplementation})
	}
	run.FinishStage(nil)
	if err := run.RecordStageResult("implementation", time.Second, nil); err != nil {
		t.Fatal(err)
	}
	assertSelectionSummary(t, run, "claude", "claude-runtime", "SUCCEEDED")
}
