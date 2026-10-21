package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"soft-factory/internal/config"
)

func TestMissingSkillStopsBeforeImplementation(t *testing.T) {
	for _, stage := range []string{"codeReview", "securityReview", "riskClassification", "reviewChanges"} {
		t.Run(stage, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("PATH", t.TempDir())
			data := `{"agentSandbox":{},"stages":{"` + stage + `":{"customSkills":"missing"}}}`
			require.NoError(t, os.WriteFile(factoryConfigFile, []byte(data), 0644))
			require.NoError(t, os.WriteFile("config.json", []byte(`{`), 0644))

			err := runWorkflow(workflowOptions{})
			want := "stages." + stage + `.customSkills: project skill "missing" not found`
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
		{"disabled", `,"disabledStages":["codeReview"]`, ""},
		{"before resume start", "", config.StageSecurityReview},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("PATH", t.TempDir())
			data := `{"agentSandbox":{},"stages":{"codeReview":{"customSkills":"skip-missing"},"securityReview":{"customSkills":"eligible-missing"}}` + tc.disabled + `}`
			require.NoError(t, os.WriteFile(factoryConfigFile, []byte(data), 0600))
			err := runWorkflow(workflowOptions{StartStage: tc.start})
			require.ErrorContains(t, err, `stages.securityReview.customSkills: project skill "eligible-missing" not found`)
			require.NotContains(t, err.Error(), "skip-missing")
		})
	}
}

func TestWorkflowIgnoresTopLevelCustomSkill(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("PATH", t.TempDir())
	data := `{"agentSandbox":{},"customSkills":{"codeReview":"old-missing"},"stages":{"securityReview":{"customSkills":"nested-missing"}}}`
	require.NoError(t, os.WriteFile(factoryConfigFile, []byte(data), 0600))
	err := runWorkflow(workflowOptions{})
	require.ErrorContains(t, err, `stages.securityReview.customSkills: project skill "nested-missing" not found`)
	require.NotContains(t, err.Error(), "old-missing")
}
