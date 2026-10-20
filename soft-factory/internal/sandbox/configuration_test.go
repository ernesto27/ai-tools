package sandbox

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
)

func decodeConfig(t *testing.T, sandbox string) config.Config {
	t.Helper()
	var cfg config.Config
	require.NoError(t, json.Unmarshal([]byte(`{"agentSandbox":`+sandbox+`}`), &cfg))
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
		{name: "all run settings", settings: `{"agent":"codex","model":"shared","effort":"high","baseImage":"go image","branch":"task branch","push":true,"pr":false,"hn":true,"file":["a b","$(touch forbidden);x"],"run":{"query":"do $stuff; now","filePrompt":"task path","commitMessage":"commit !","image":["one,two.png","two.png"],"apiKey":"secret-run","reviewers":["secret-reviewer"]}}`, invocation: []string{"run"}, want: []string{"run", "--agent", "codex", "--model", "shared", "--effort", "high", "--baseImage", "go image", "--branch", "task branch", "--push=true", "--hn=true", "--file", "a b", "--file", "$(touch forbidden);x", "--query", "do $stuff; now", "--filePrompt", "task path", "--commitMessage", "commit !", "--image", "one,two.png", "--image", "two.png"}},
		{name: "resume explicit zeros replace shared", settings: `{"agent":"codex","model":"shared","effort":"high","baseImage":"shared","branch":"shared","push":true,"pr":true,"hn":true,"file":["shared"],"resume":{"agent":"","model":"","effort":"","baseImage":"","branch":"","push":false,"pr":false,"hn":false,"file":[],"image":[],"query":"","filePrompt":"","commitMessage":"","apiKey":"secret-resume","reviewers":["secret-reviewer"]}}`, invocation: []string{"resume"}, want: []string{"resume", "--agent", "", "--model", "", "--effort", "", "--baseImage", "", "--branch", "", "--push=false", "--pr=false", "--hn=false", "--query", "", "--filePrompt", "", "--commitMessage", ""}},
		{name: "shared zeros use sandbox defaults", settings: `{"agent":"","model":"","effort":"","baseImage":"","branch":"","push":false,"pr":false,"hn":false}`, invocation: []string{"run"}, want: []string{"run"}},
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
			_, err := execute(ExecuteOptions{Config: cfg, Args: tt.invocation, Branch: tt.branch, BranchSet: tt.branchSet})
			require.NoError(t, err)

			got := readPromptCapture(t, capturePath)
			require.Equal(t, tt.want, got.Args)
			cwd, _ := os.Getwd()
			require.False(t, got.SecretInEnvironment)
			require.Equal(t, cwd, got.Directory)
			assertNoConfigArtifacts(t)
		})
	}
}

func assertNoConfigArtifacts(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	for _, entry := range entries {
		require.NotContains(t, entry.Name(), "agent-sandbox.json")
	}
}

func TestUnifiedTaskAndSelection(t *testing.T) {
	capture := preparePromptSandbox(t)
	cfg := decodeConfig(t, `{"agent":"shared","model":"shared-model","run":{"filePrompt":"task.txt","apiKey":"secret-run"},"resume":{"agent":"resume-agent","model":"resume-model","filePrompt":"missing.txt","apiKey":"secret-resume","reviewers":["secret-reviewer"]}}`)
	// Changing the config on disk cannot affect the already-loaded workflow.
	require.NoError(t, os.WriteFile("software-factory.json", []byte(`{"agentSandbox":{"agent":"wrong"}}`), 0600))
	log, err := executionlog.NewRun("main", executionlog.CommandContinue, nil)
	require.NoError(t, err)
	log.StartStage(config.StageImplementation)
	cfg.Stages.Implementation.Model = "stage-model"
	require.NoError(t, Run(TaskContext{Config: cfg, Continue: true, Branch: "main", Log: log}))
	log.FinishStage(nil)
	log.Finish(nil)
	got := readPromptCapture(t, capture)
	require.True(t, strings.HasPrefix(got.Prompt, "file task {{literal}} 100%"))
	selection, agent, model, _ := selectionDetails(cfg, "resume", config.StageImplementation, cfg.Stages.Implementation)
	require.Equal(t, "resume-agent", selection.Agent)
	require.Equal(t, "stage-model", selection.Model)
	require.Contains(t, agent, "software-factory.json: agentSandbox.resume.agent")
	require.Contains(t, model, "software-factory.json: stages.implementation.model")
	files, err := filepath.Glob(filepath.Join(log.Path(), "*.log"))
	require.NoError(t, err)
	for _, path := range files {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NotContains(t, string(data), "secret-")
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
			require.Error(t, err)
			if mode == "cancel" {
				require.ErrorIs(t, err, context.Canceled)
			}
			if mode == "child failure" {
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr)
				require.Equal(t, 17, exitErr.ExitCode())
				got := readPromptCapture(t, capture)
				for i, arg := range got.Args {
					if arg == "-f" {
						_, err := os.Stat(got.Args[i+1])
						require.ErrorIs(t, err, os.ErrNotExist)
					}
				}
			}
			prompts, globErr := filepath.Glob(filepath.Join(os.Getenv("TMPDIR"), "factory-prompt-*.txt"))
			require.NoError(t, globErr)
			require.Len(t, prompts, 0)

			assertNoConfigArtifacts(t)
		})
	}
}

func TestMissingUnifiedTaskPath(t *testing.T) {
	for _, settings := range []string{`null`, `{}`, `{"run":{"filePrompt":"missing task.txt"}}`} {
		t.Run(settings, func(t *testing.T) {
			t.Chdir(t.TempDir())
			require.NoError(t, os.WriteFile("agent-sandbox.json", []byte(`{"run":{"filePrompt":"legacy task.txt"}}`), 0600))
			_, err := buildTaskPrompt(TaskContext{Config: decodeConfig(t, settings)})
			require.ErrorIs(t, err, os.ErrNotExist)
			require.ErrorContains(t, err, "software-factory.json: agentSandbox.run.filePrompt")
			require.NotContains(t, err.Error(), "legacy task.txt")
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
			require.FailNow(t, "child stopped before cancellation", "%v", err)
		case <-deadline:
			cancel()
			require.FailNow(t, "child never reached capture fixture")
		case <-ticker.C:
			if _, err := os.Stat(capture); err == nil {
				break waiting
			}
		}
	}
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "child did not stop on interruption")
	}
	got := readPromptCapture(t, capture)
	for i, arg := range got.Args {
		if arg == "-f" {
			_, err := os.Stat(got.Args[i+1])
			require.ErrorIs(t, err, os.ErrNotExist)
		}
	}
	assertNoConfigArtifacts(t)
}
