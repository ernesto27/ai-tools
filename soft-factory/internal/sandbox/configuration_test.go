package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
)

func decodeConfig(t *testing.T, sandbox string) config.Config {
	t.Helper()
	var cfg config.Config
	if err := json.Unmarshal([]byte(`{"agentSandbox":`+sandbox+`}`), &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func taskFileConfig(t *testing.T) config.Config {
	return decodeConfig(t, `{"run":{"filePrompt":"task.txt"}}`)
}

func TestSandboxArgumentTransport(t *testing.T) {
	for _, tt := range []struct {
		name, settings   string
		invocation, want []string
		branch           string
		branchSet        bool
	}{
		{name: "all run settings", settings: `{"agent":"codex","model":"shared","effort":"high","baseImage":"go image","branch":"task branch","push":true,"pr":false,"hn":true,"file":["a b","$(touch forbidden);x"],"run":{"query":"do $stuff; now","filePrompt":"task path","commitMessage":"commit !","image":["one,two.png","two.png"],"apiKey":"secret-run","reviewers":["secret-reviewer"]}}`, invocation: []string{"run"}, want: []string{"run", "--agent", "codex", "--model", "shared", "--effort", "high", "--baseImage", "go image", "--branch", "task branch", "--push=true", "--pr=false", "--hn=true", "--file", "a b", "--file", "$(touch forbidden);x", "--query", "do $stuff; now", "--filePrompt", "task path", "--commitMessage", "commit !", "--image", "one,two.png", "--image", "two.png"}},
		{name: "resume explicit zeros replace shared", settings: `{"agent":"codex","model":"shared","effort":"high","baseImage":"shared","branch":"shared","push":true,"pr":true,"hn":true,"file":["shared"],"resume":{"agent":"","model":"","effort":"","baseImage":"","branch":"","push":false,"pr":false,"hn":false,"file":[],"image":[],"query":"","filePrompt":"","commitMessage":"","apiKey":"secret-resume","reviewers":["secret-reviewer"]}}`, invocation: []string{"resume"}, want: []string{"resume", "--agent", "", "--model", "", "--effort", "", "--baseImage", "", "--branch", "", "--push=false", "--pr=false", "--hn=false", "--query", "", "--filePrompt", "", "--commitMessage", ""}},
		{name: "explicit shared zeros", settings: `{"agent":"","model":"","effort":"","baseImage":"","branch":"","push":false,"pr":false,"hn":false}`, invocation: []string{"run"}, want: []string{"run", "--agent", "", "--model", "", "--effort", "", "--baseImage", "", "--branch", "", "--push=false", "--pr=false", "--hn=false"}},
		{name: "workflow controls and independent agent override", settings: `{"agent":"shared","model":"shared-model","branch":"shared","push":true,"pr":true,"resume":{"agent":"section","model":"section-model","branch":"section","query":"ignored","filePrompt":"ignored","file":["replacement"]}}`, invocation: []string{"resume", "--push=false", "--pr=false", "-f", "generated path", "-a", "stage"}, branch: "workflow", want: []string{"resume", "--push=false", "--pr=false", "-f", "generated path", "-a", "stage", "--model", "section-model", "--branch", "workflow", "--file", "replacement"}},
		{name: "independent model override", settings: `{"agent":"shared","resume":{"agent":"section","model":"old"}}`, invocation: []string{"resume", "-m", "stage-model"}, want: []string{"resume", "-m", "stage-model", "--agent", "section"}},
		{name: "empty workflow branch overrides resume", settings: `{"resume":{"branch":"other"}}`, invocation: []string{"resume"}, branchSet: true, want: []string{"resume", "--branch", ""}},
		{name: "explicit invocation branch is emitted once", settings: `{"branch":"shared","resume":{"branch":"section"}}`, invocation: []string{"resume", "--branch", "explicit"}, branch: "workflow", branchSet: true, want: []string{"resume", "--branch", "explicit"}},
		{name: "null sandbox", settings: `null`, invocation: []string{"run"}, want: []string{"run"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			capturePath := preparePromptSandbox(t)
			t.Setenv("SOFT_FACTORY_ARGS_ONLY", "1")
			cfg := decodeConfig(t, tt.settings)
			// Use execute to assert the actual argv boundary and unchanged cwd.
			if _, err := execute(ExecuteOptions{Config: cfg, Args: tt.invocation, Branch: tt.branch, BranchSet: tt.branchSet}); err != nil {
				t.Fatal(err)
			}
			got := readPromptCapture(t, capturePath)
			if !reflect.DeepEqual(got.Args, tt.want) {
				t.Fatalf("args = %q, want %q", got.Args, tt.want)
			}
			cwd, _ := os.Getwd()
			if got.SecretInEnvironment {
				t.Fatal("embedded secret forwarded through environment")
			}
			if got.Directory != cwd {
				t.Fatalf("cwd = %q, want %q", got.Directory, cwd)
			}
			assertNoConfigArtifacts(t)
		})
	}
}

func assertNoConfigArtifacts(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "agent-sandbox.json") {
			t.Fatalf("unexpected configuration artifact: %s", entry.Name())
		}
	}
}

func TestLegacyEntriesPreventExecution(t *testing.T) {
	for _, kind := range []string{"valid", "malformed", "conflicting", "symlink", "directory", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			capture := preparePromptSandbox(t)
			content := []byte(`{"agent":"legacy","apiKey":"never disclose"}`)
			switch kind {
			case "symlink":
				if err := os.Symlink("missing-target", "agent-sandbox.json"); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir("agent-sandbox.json", 0700); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				createLegacyFIFO(t)
			default:
				if kind == "malformed" {
					content = []byte(`{malformed secret`)
				}
				if err := os.WriteFile("agent-sandbox.json", content, 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.Lstat("agent-sandbox.json")
			if err != nil {
				t.Fatal(err)
			}
			_, err = execute(ExecuteOptions{Config: decodeConfig(t, `{"agent":"embedded"}`), Args: []string{"run"}})
			if err == nil || !strings.Contains(err.Error(), "migrate") || !strings.Contains(err.Error(), "software-factory.json: agentSandbox") || strings.Contains(err.Error(), "secret") {
				t.Fatalf("error = %v", err)
			}
			if _, err := os.Stat(capture); !os.IsNotExist(err) {
				t.Fatalf("child launched: %v", err)
			}
			after, err := os.Lstat("agent-sandbox.json")
			if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
				t.Fatalf("legacy entry changed: %v", err)
			}
			if kind == "valid" || kind == "malformed" || kind == "conflicting" {
				data, err := os.ReadFile("agent-sandbox.json")
				if err != nil || string(data) != string(content) {
					t.Fatal("legacy content changed")
				}
			}
		})
	}
}

func TestUnifiedTaskAndSelection(t *testing.T) {
	capture := preparePromptSandbox(t)
	cfg := decodeConfig(t, `{"agent":"shared","model":"shared-model","run":{"filePrompt":"task.txt","apiKey":"secret-run"},"resume":{"agent":"resume-agent","model":"resume-model","filePrompt":"missing.txt","apiKey":"secret-resume","reviewers":["secret-reviewer"]}}`)
	// Changing the config on disk cannot affect the already-loaded workflow.
	if err := os.WriteFile("software-factory.json", []byte(`{"agentSandbox":{"agent":"wrong"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	log, err := executionlog.NewRun("main", executionlog.CommandContinue, nil)
	if err != nil {
		t.Fatal(err)
	}
	log.StartStage(config.StageImplementation)
	cfg.Stages.Implementation.Model = "stage-model"
	if err := Run(TaskContext{Config: cfg, Continue: true, Branch: "main", Log: log}); err != nil {
		t.Fatal(err)
	}
	log.FinishStage(nil)
	log.Finish(nil)
	got := readPromptCapture(t, capture)
	if !strings.HasPrefix(got.Prompt, "file task {{literal}} 100%") {
		t.Fatalf("original run task missing: %q", got.Prompt)
	}
	selection, agent, model, _ := selectionDetails(cfg, "resume", config.StageImplementation, cfg.Stages.Implementation)
	if selection.Agent != "resume-agent" || selection.Model != "stage-model" || !strings.Contains(agent, "software-factory.json: agentSandbox.resume.agent") || !strings.Contains(model, "software-factory.json: stages.implementation.model") {
		t.Fatalf("selection = %+v, %s, %s", selection, agent, model)
	}
	files, err := filepath.Glob(filepath.Join(log.Path(), "*.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "secret-") {
			t.Fatal("secret disclosed in logs")
		}
	}
	assertNoConfigArtifacts(t)
}

func TestExecutionFailureAndCancellation(t *testing.T) {
	for _, mode := range []string{"child failure", "cancel", "setup failure"} {
		t.Run(mode, func(t *testing.T) {
			capture := preparePromptSandbox(t)
			input := TaskContext{Config: taskFileConfig(t), Continue: true, Branch: "main"}
			switch mode {
			case "child failure":
				t.Setenv("SOFT_FACTORY_PROMPT_FAIL", "1")
			case "cancel":
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				input.Context = ctx
			case "setup failure":
				t.Setenv("PATH", t.TempDir())
			}
			err := Run(input)
			if err == nil {
				t.Fatal("expected error")
			}
			if mode == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v", err)
			}
			if mode == "child failure" {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != 17 {
					t.Fatalf("downstream failure lost: %v", err)
				}
				got := readPromptCapture(t, capture)
				for i, arg := range got.Args {
					if arg == "-f" {
						if _, err := os.Stat(got.Args[i+1]); !os.IsNotExist(err) {
							t.Fatal("prompt remains")
						}
					}
				}
			}
			prompts, globErr := filepath.Glob(filepath.Join(os.Getenv("TMPDIR"), "factory-prompt-*.txt"))
			if globErr != nil || len(prompts) != 0 {
				t.Fatalf("temporary prompts remain: %q, %v", prompts, globErr)
			}

			assertNoConfigArtifacts(t)
		})
	}
}

func TestLegacyInspectionError(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.Chmod(root, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(root, 0700)
	err := requireNoLegacyConfig()
	if err == nil {
		t.Skip("process can inspect directories without search permission")
	}
	if !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), "inspect legacy agent-sandbox.json") {
		t.Fatalf("inspection error = %v", err)
	}
}

func TestMissingUnifiedTaskPath(t *testing.T) {
	for _, settings := range []string{`null`, `{}`, `{"run":{"filePrompt":"missing task.txt"}}`} {
		t.Run(settings, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile("agent-sandbox.json", []byte(`{"run":{"filePrompt":"legacy task.txt"}}`), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := buildTaskPrompt(TaskContext{Config: decodeConfig(t, settings)})
			if err == nil || !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "software-factory.json: agentSandbox.run.filePrompt") || strings.Contains(err.Error(), "legacy task.txt") {
				t.Fatalf("task error = %v", err)
			}
		})
	}
}

func TestRunningChildCancellationCleanup(t *testing.T) {
	capture := preparePromptSandbox(t)
	t.Setenv("SOFT_FACTORY_PROMPT_WAIT", "1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- Run(TaskContext{Config: taskFileConfig(t), Continue: true, Context: ctx}) }()
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
waiting:
	for {
		select {
		case err := <-result:
			t.Fatalf("child stopped before cancellation: %v", err)
		case <-deadline:
			cancel()
			t.Fatal("child never reached capture fixture")
		case <-ticker.C:
			if _, err := os.Stat(capture); err == nil {
				break waiting
			}
		}
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child did not stop on interruption")
	}
	got := readPromptCapture(t, capture)
	for i, arg := range got.Args {
		if arg == "-f" {
			if _, err := os.Stat(got.Args[i+1]); !os.IsNotExist(err) {
				t.Fatal("prompt remains after cancellation")
			}
		}
	}
	assertNoConfigArtifacts(t)
}
