package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"soft-factory/internal/config"
	"soft-factory/internal/sandbox"
)

func TestWorkflowTaskFileAndGeneratedBranch(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("HOME", t.TempDir())
	output, err := exec.Command("git", "init", "-q").CombinedOutput()
	require.NoError(t, err, "%s", output)
	output, err = exec.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.com",
		"-c", "commit.gpgsign=false", "commit", "--allow-empty", "-qm", "initial").CombinedOutput()
	require.NoError(t, err, "%s", output)

	bin := t.TempDir()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	const fakeSandbox = `#!/bin/sh
set -e
while [ "$#" -gt 0 ]; do
  case "$1" in
    -f) cat "$2" > "$SOFT_FACTORY_TEST_CAPTURE/prompt"; shift ;;
    --branch) printf '%s\n' "$2" > "$SOFT_FACTORY_TEST_CAPTURE/branch"; shift ;;
  esac
  shift
done
exit 23
`
	require.NoError(t, os.WriteFile(filepath.Join(bin, "agent-sandbox"), []byte(fakeSandbox), 0700))
	const taskFile = "cron task.txt"
	const overrideText = "Scheduled task prompt"
	const defaultText = "Default task prompt"
	require.NoError(t, os.WriteFile(taskFile, []byte(overrideText), 0600))
	require.NoError(t, os.WriteFile("default.txt", []byte(defaultText), 0600))

	branches := make(map[string]string)
	for _, tt := range []struct {
		name, config, prefix string
	}{
		{"configured prompt", `{"agentSandbox":{"run":{"branch":"test-cron","filePrompt":["default.txt"]}}}`, "test-cron"},
		{"missing run section", `{"agentSandbox":{"branch":"test-cron"}}`, "test-cron"},
		{"no configured branch", `{"agentSandbox":{}}`, "scheduled-task"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			capture := t.TempDir()
			t.Setenv("SOFT_FACTORY_TEST_CAPTURE", capture)
			require.NoError(t, os.WriteFile(factoryConfigFile, []byte(tt.config), 0600))
			err := runWorkflow(workflowOptions{TaskFile: taskFile})
			var exitErr *exec.ExitError
			require.ErrorAs(t, err, &exitErr)
			require.Equal(t, 23, exitErr.ExitCode())

			prompt, err := os.ReadFile(filepath.Join(capture, "prompt"))
			require.NoError(t, err)
			require.Contains(t, string(prompt), overrideText)
			require.NotContains(t, string(prompt), defaultText)

			branch, err := os.ReadFile(filepath.Join(capture, "branch"))
			require.NoError(t, err)
			name := strings.TrimSpace(string(branch))
			require.Regexp(t, "^"+regexp.QuoteMeta(tt.prefix)+`-[0-9]{8}-[0-9]{6}-[a-z2-7]{26,}$`, name)
			branches[tt.name] = name
		})
	}
	require.NotEqual(t, branches["configured prompt"], branches["missing run section"])
}

func TestMissingSkillStopsBeforeImplementation(t *testing.T) {
	for _, stage := range []string{"codeReview", "securityReview", "riskClassification", "reviewChanges"} {
		t.Run(stage, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("PATH", t.TempDir())
			data := `{"agentSandbox":{},"stages":{"` + stage + `":{"customSkill":"missing"}}}`
			require.NoError(t, os.WriteFile(factoryConfigFile, []byte(data), 0644))
			require.NoError(t, os.WriteFile("config.json", []byte(`{`), 0644))

			err := runWorkflow(workflowOptions{})
			want := "stages." + stage + `.customSkill: project skill "missing" not found`
			require.ErrorContains(t, err, want)
		})
	}
}

func TestWorkflowDoesNotFallBackToLegacyConfig(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		name := "missing"
		if invalid {
			name = "invalid"
		}
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("PATH", t.TempDir())
			require.NoError(t, os.WriteFile("config.json", []byte(`{"customSkills":{"codeReview":"missing"}}`), 0644))
			if invalid {
				require.NoError(t, os.WriteFile(factoryConfigFile, []byte(`{`), 0644))
			}
			err := runWorkflow(workflowOptions{})
			if invalid {
				require.ErrorContains(t, err, "parse factory configuration")
			} else {
				require.ErrorIs(t, err, os.ErrNotExist)
				require.ErrorContains(t, err, factoryConfigFile)
			}
		})
	}
}

func TestWorkflowSkipsIneligibleCustomSkills(t *testing.T) {
	for _, tc := range []struct {
		name, disabled string
		start          config.Stage
	}{
		{"disabled", `,"disabled":true`, ""},
		{"before resume start", "", config.StageSecurityReview},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("PATH", t.TempDir())
			data := `{"agentSandbox":{},"stages":{"codeReview":{"customSkill":"skip-missing"` + tc.disabled + `},"securityReview":{"customSkill":"eligible-missing"}}}`
			require.NoError(t, os.WriteFile(factoryConfigFile, []byte(data), 0600))
			err := runWorkflow(workflowOptions{StartStage: tc.start})
			require.ErrorContains(t, err, `stages.securityReview.customSkill: project skill "eligible-missing" not found`)
			require.NotContains(t, err.Error(), "skip-missing")
		})
	}
}

func TestWorkflowIgnoresTopLevelCustomSkill(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("PATH", t.TempDir())
	data := `{"agentSandbox":{},"customSkills":{"codeReview":"old-missing"},"stages":{"securityReview":{"customSkill":"nested-missing"}}}`
	require.NoError(t, os.WriteFile(factoryConfigFile, []byte(data), 0600))
	err := runWorkflow(workflowOptions{})
	require.ErrorContains(t, err, `stages.securityReview.customSkill: project skill "nested-missing" not found`)
	require.NotContains(t, err.Error(), "old-missing")
}

func TestLoadedDisabledStageControlsWorkflow(t *testing.T) {
	for _, stage := range config.Stages {
		for _, disabled := range []bool{false, true} {
			name := string(stage) + "/enabled"
			if disabled {
				name = string(stage) + "/disabled"
			}
			t.Run(name, func(t *testing.T) {
				data, err := json.Marshal(map[string]any{
					"agentSandbox": map[string]any{},
					"stages": map[string]any{
						string(stage): map[string]any{"disabled": disabled},
					},
				})
				require.NoError(t, err)
				path := filepath.Join(t.TempDir(), "software-factory.json")
				require.NoError(t, os.WriteFile(path, data, 0600))
				cfg, err := config.Load(path)
				require.NoError(t, err)
				planned := plannedStages(cfg)
				if disabled {
					require.NotContains(t, planned, stage)
				} else {
					require.Equal(t, config.Stages, planned)
				}
				called := false
				input := sandbox.TaskContext{Context: context.Background(), Config: cfg}
				require.NoError(t, runStage(input, cfg, stage, "test stage", func() error {
					called = true
					return nil
				}))
				require.Equal(t, !disabled, called)
			})
		}
	}
}
