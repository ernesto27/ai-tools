package sandbox

import (
	"context"
	"errors"
	"io"
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
		{"retry", codexStartup + codexStartup, "codex", "runtime-model"},
		{"fallback", codexStartup + claudeStartup, "claude", "claude-runtime"},
		{"fallback to codex", claudeStartup + codexStartup, "codex", "runtime-model"},
		{"missing", "Agent: configured-agent\nModel: configured-model\n", "unavailable", "unavailable"},
		{"not a version", "OpenAI Codex version guessed\nmodel: guessed\n", "unavailable", "unavailable"},
		{"multiline value", `{"type":"system","subtype":"init","model":"name\n   Model: injected"}`, "claude", "unavailable"},
		{"missing model", codexStartup + `{"type":"system","subtype":"init"}`, "claude", "unavailable"},
		{"malformed model", `{"type":"system","subtype":"init","model":42}`, "claude", "unavailable"},
		{"partial header", codexStartup + "OpenAI Codex v0.101.0", "codex", "unavailable"},
		{"partial init", codexStartup + "{\n  \"type\": \"system\",\n  \"subtype\": \"init\",\n", "claude", "unavailable"},
		{"authored text", `{"type":"assistant","message":{"content":"Model: authored-model","model":"authored-model"}}`, "unavailable", "unavailable"},
		{"header ended", codexStartup + "model: authored-model\n", "codex", "runtime-model"},
		{"oversize", strings.Repeat("x", selectionBufferLimit+1) + "\n" + claudeStartup, "claude", "claude-runtime"},
		{"terminal escapes", "\x1b[32m" + codexStartup + "\x1b[0m", "codex", "runtime-model"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			run := newSelectionLog(t)
			run.StartStage("implementation")
			w := &selectionWriter{out: io.Discard, log: run}
			// Exercise arbitrary pipe boundaries, including every JSON token.
			for _, b := range []byte(tt.output) {
				if n, err := w.Write([]byte{b}); n != 1 || err != nil {
					t.Fatalf("write: %d, %v", n, err)
				}
			}
			w.flush()
			run.FinishStage(nil)
			if err := run.RecordStageResult("implementation", time.Second, nil); err != nil {
				t.Fatal(err)
			}
			assertSelectionSummary(t, run, tt.agent, tt.model, "SUCCEEDED")
		})
	}
}

func TestExecuteSelection(t *testing.T) {
	for _, tt := range []struct {
		name, script, agent, model, status string
		wantErr                            bool
	}{
		{"success", "printf '%s' '" + codexStartup + "'\n", "codex", "runtime-model", "SUCCEEDED", false},
		{"failed fallback", "printf '%s' '" + codexStartup + claudeStartup + "'\nexit 1\n", "claude", "claude-runtime", "FAILED", true},
		{"interrupted", "printf '%s' '" + claudeStartup + "'\nexit 130\n", "claude", "claude-runtime", "INTERRUPTED", true},
		{"missing", "printf '%s\\n' 'successful agent output'\n", "unavailable", "unavailable", "SUCCEEDED", false},
		{"before launch", "", "unavailable", "unavailable", "FAILED", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			run := newSelectionLog(t)
			if tt.script != "" {
				if err := os.WriteFile(filepath.Join(os.Getenv("PATH"), "agent-sandbox"), []byte("#!/bin/sh\n"+tt.script), 0700); err != nil {
					t.Fatal(err)
				}
			}
			// Configured values intentionally disagree with runtime records.
			if err := os.WriteFile("agent-sandbox.json", []byte(`{"agent":"configured-agent","model":"configured-model"}`), 0600); err != nil {
				t.Fatal(err)
			}
			run.StartStage("implementation")
			_, err := execute(ExecuteOptions{Args: []string{"run"}, Context: context.Background(), Log: run, Stage: config.StageImplementation})
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
