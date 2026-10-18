package sandbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
)

func TestUnifiedConfigHandoffAcrossStages(t *testing.T) {
	for _, stage := range []string{"run", "continue", "codeReview", "securityReview", "riskClassification", "reviewChanges"} {
		for _, legacy := range []string{`{`, `{"agent":"legacy","model":"legacy-model","run":{"filePrompt":"missing-task.txt"}}`} {
			t.Run(stage+"/"+legacy, func(t *testing.T) {
				capturePath := preparePromptSandbox(t)
				if err := os.WriteFile("agent-sandbox.json", []byte(legacy), 0600); err != nil {
					t.Fatal(err)
				}
				const settings = `{"agent":"codex","model":"shared","branch":"main","push":true,"pr":true,"baseImage":"golang:1.26-alpine","file":["tracked.txt"],"run":{"branch":"main","filePrompt":"task.txt","apiKey":"transport-secret","model":"run-model"},"resume":{"branch":"main","filePrompt":"wrong-resume-task.txt","apiKey":"resume-secret","model":"","push":false,"pr":false,"hn":false},"upstreamUnknown":{"preserve":true}}`
				data := `{"stages":{"implementation":{"agent":"claude"},"codeReview":{"model":"stage-model"}},"agentSandbox":` + settings + `}`
				if err := os.WriteFile("software-factory.json", []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
				cfg := loadPromptConfig(t)
				// Later reads must not reload either configuration source.
				if err := os.WriteFile("software-factory.json", []byte(`{`), 0600); err != nil {
					t.Fatal(err)
				}
				input := TaskContext{Config: cfg, Branch: "main", Documents: "supporting context"}
				run, err := executionlog.NewRun("main", executionlog.CommandRun, nil)
				if err != nil {
					t.Fatal(err)
				}
				input.Log = run
				run.StartStage(config.StageImplementation)
				switch stage {
				case "run":
					err = Run(input)
				case "continue":
					input.Continue = true
					err = Run(input)
				case "codeReview":
					_, err = Review(input)
				case "securityReview":
					_, err = SecurityReview(input)
				case "riskClassification":
					err = RiskClassification(input, nil, nil)
				case "reviewChanges":
					err = ReviewChanges(input)
				}
				run.FinishStage(err)
				run.Finish(err)
				if err != nil {
					t.Fatal(err)
				}
				got := readPromptCapture(t, capturePath)
				if string(got.Config) != settings {
					t.Fatalf("embedded JSON changed: %s", got.Config)
				}
				wd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				if got.WorkingDir != wd {
					t.Fatalf("cwd = %q, want %q", got.WorkingDir, wd)
				}
				if !strings.Contains(got.Prompt, "file task {{literal}} 100%") || !strings.Contains(got.Prompt, "supporting context") {
					t.Fatal("original run task or supporting context missing")
				}
				wantMode := "resume"
				if stage == "run" {
					wantMode = "run"
				}
				if got.Args[0] != wantMode {
					t.Fatalf("mode = %q, want %q", got.Args[0], wantMode)
				}
				args := strings.Join(got.Args, " ")
				if (stage == "run" || stage == "continue") && (!strings.Contains(args, "-a claude") || slices.Contains(got.Args, "-m")) {
					t.Fatalf("independent agent override: %s", args)
				}
				if stage == "codeReview" && (!strings.Contains(args, "-m stage-model") || slices.Contains(got.Args, "-a")) {
					t.Fatalf("independent model override: %s", args)
				}
				if strings.Contains(args, "transport-secret") || strings.Contains(args, "resume-secret") {
					t.Fatal("configuration secret in arguments")
				}
				logs, err := filepath.Glob(filepath.Join(run.Path(), "*.log"))
				if err != nil {
					t.Fatal(err)
				}
				var logText strings.Builder
				for _, path := range logs {
					text, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					logText.Write(text)
				}
				if strings.Contains(logText.String(), "transport-secret") || strings.Contains(logText.String(), "resume-secret") || strings.Contains(logText.String(), "agent-sandbox.json:") {
					t.Fatal("secret or legacy provenance in logs")
				}
				if stage == "run" && (!strings.Contains(logText.String(), "software-factory.json: stages.implementation.agent") || !strings.Contains(logText.String(), "software-factory.json: agentSandbox.run.model")) {
					t.Fatal("selection provenance disagrees with invocation")
				}
				if got, err := os.ReadFile("agent-sandbox.json"); err != nil || string(got) != legacy {
					t.Fatal("legacy file changed")
				}
			})
		}
	}
}

func TestConfigTransportCleanupOnCancellation(t *testing.T) {
	capturePath := preparePromptSandbox(t)
	t.Setenv("SOFT_FACTORY_PROMPT_WAIT", "1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	cfg := loadPromptConfig(t)
	go func() { done <- Run(TaskContext{Config: cfg, TaskOverride: "task", Context: ctx}) }()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
ready:
	for {
		select {
		case <-tick.C:
			if _, err := os.Stat(capturePath); err == nil {
				break ready
			}
		case err := <-done:
			t.Fatalf("subprocess returned before cancellation: %v", err)
		case <-deadline.C:
			cancel()
			t.Fatal("subprocess did not capture configuration")
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled subprocess did not exit")
	}
	readPromptCapture(t, capturePath)
}

func TestSelectionUsesLoadedSandbox(t *testing.T) {
	preparePromptSandbox(t)
	data := `{"agentSandbox":{"agent":"codex","model":"shared-model","run":{"agent":"claude","model":"run-model"},"resume":{"model":""}}}`
	if err := os.WriteFile("software-factory.json", []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := loadPromptConfig(t)
	selected, agent, model, _ := selectionDetails(cfg.AgentSandbox, "run", config.StageImplementation, config.StageConfig{Model: "stage-model"})
	if selected.Agent != "claude" || selected.Model != "stage-model" || !strings.Contains(agent, "agentSandbox.run.agent") || !strings.Contains(model, "stages.implementation.model") {
		t.Fatal("incorrect stage selection provenance")
	}
	selected, _, model, _ = selectionDetails(cfg.AgentSandbox, "resume", config.StageCodeReview, config.StageConfig{})
	if selected.Model == "shared-model" || strings.Contains(model, "shared-model") {
		t.Fatal("explicit empty model failed to clear shared setting")
	}
}

func TestEmptySandboxHandoffIgnoresLegacy(t *testing.T) {
	for _, data := range []string{`{}`, `{"agentSandbox":null}`} {
		t.Run(data, func(t *testing.T) {
			capturePath := preparePromptSandbox(t)
			if err := os.WriteFile("software-factory.json", []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			const legacy = `{"agent":"legacy","resume":{"query":"legacy task"}}`
			if err := os.WriteFile("agent-sandbox.json", []byte(legacy), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := execute(ExecuteOptions{Config: loadPromptConfig(t), Args: []string{"resume", "-q", "explicit task"}})
			if err != nil {
				t.Fatal(err)
			}
			got := readPromptCapture(t, capturePath)
			if string(got.Config) != "{}" {
				t.Fatalf("configuration = %s, want empty object", got.Config)
			}
			if got, err := os.ReadFile("agent-sandbox.json"); err != nil || string(got) != legacy {
				t.Fatal("legacy file changed")
			}
		})
	}
}

func TestConfigTransportCleanupOnLaunchFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	t.Setenv("PATH", t.TempDir())
	_, err := execute(ExecuteOptions{Args: []string{"run"}})
	if err == nil || !strings.Contains(err.Error(), "software-factory.json: agentSandbox") {
		t.Fatalf("error = %v", err)
	}
	files, err := os.ReadDir(temp)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatal("configuration transport remains after launch failure")
	}
}
