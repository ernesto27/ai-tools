package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"soft-factory/internal/config"
	"soft-factory/internal/sandbox"
)

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
