package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSecuritySaveFindingsConfiguration(t *testing.T) {
	for _, tc := range []struct {
		value   string
		want    bool
		invalid bool
	}{
		{"", false, false}, {"false", false, false}, {"true", true, false},
		{"null", false, false}, {`"true"`, false, true}, {"1", false, true}, {"[]", false, true}, {"{}", false, true},
	} {
		t.Run("value="+tc.value, func(t *testing.T) {
			property := ""
			if tc.value != "" {
				property = `,"saveFindings":` + tc.value
			}
			data := fmt.Sprintf(`{"agentSandbox":{"agent":"shared"},"stages":{"securityReview":{"disabled":true,"agent":"codex","model":"security-model","customSkill":"project"%s},"codeReview":{"model":"code-model"}}}`, property)
			path := filepath.Join(t.TempDir(), "config.json")
			require.NoError(t, os.WriteFile(path, []byte(data), 0600))
			cfg, err := Load(path)
			if tc.invalid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, cfg.Stages.SecurityReview.SaveFindings)
			require.Equal(t, "codex", cfg.Stages.SecurityReview.Agent)
			require.Equal(t, "security-model", cfg.Stages.SecurityReview.Model)
			require.Equal(t, "project", cfg.Stages.SecurityReview.CustomSkill)
			require.Equal(t, "code-model", cfg.Stages.CodeReview.Model)
			require.True(t, cfg.StageDisabled(StageSecurityReview))
		})
	}
}
