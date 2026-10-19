package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecuritySaveFindingsConfiguration(t *testing.T) {
	for _, tc := range []struct {
		value         string
		want, invalid bool
	}{
		{"", false, false}, {"false", false, false}, {"true", true, false},
		{"null", false, true}, {`"true"`, false, true}, {"1", false, true}, {"[]", false, true}, {"{}", false, true},
	} {
		t.Run("value="+tc.value, func(t *testing.T) {
			property := ""
			if tc.value != "" {
				property = `,"saveFindings":` + tc.value
			}
			data := fmt.Sprintf(`{"agentSandbox":{"agent":"shared"},"disabledStages":["securityReview"],"stages":{"securityReview":{"agent":"codex","model":"security-model"%s},"codeReview":{"model":"code-model"}}}`, property)
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if tc.invalid {
				if err == nil || !strings.Contains(err.Error(), "stages.securityReview.saveFindings") {
					t.Fatalf("expected named validation error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Stages.SecurityReview.SaveFindings != tc.want || cfg.Stages.SecurityReview.Agent != "codex" || cfg.Stages.SecurityReview.Model != "security-model" || cfg.Stages.CodeReview.Model != "code-model" || !cfg.StageDisabled(StageSecurityReview) {
				t.Fatalf("configuration not preserved: %+v", cfg)
			}
		})
	}
}
