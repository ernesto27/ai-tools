package main

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestMissingSkillStopsBeforeImplementation(t *testing.T) {
	for _, stage := range []string{"codeReview", "securityReview", "riskClassification", "reviewChanges"} {
		t.Run(stage, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("PATH", t.TempDir())
			data := `{"agentSandbox":{},"customSkills":{"` + stage + `":"missing"}}`
			if err := os.WriteFile(factoryConfigFile, []byte(data), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile("config.json", []byte(`{`), 0644); err != nil {
				t.Fatal(err)
			}

			err := runWorkflow(workflowOptions{})
			want := "customSkills." + stage + `: project skill "missing" not found`
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("expected missing-skill error %q first, got %v", want, err)
			}
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
			if err := os.WriteFile("config.json", []byte(`{"customSkills":{"codeReview":"missing"}}`), 0644); err != nil {
				t.Fatal(err)
			}
			if invalid {
				if err := os.WriteFile(factoryConfigFile, []byte(`{`), 0644); err != nil {
					t.Fatal(err)
				}
			}
			err := runWorkflow(workflowOptions{})
			if invalid {
				if err == nil || !strings.Contains(err.Error(), "parse factory configuration") {
					t.Fatalf("expected invalid factory configuration error, got %v", err)
				}
			} else if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), factoryConfigFile) {
				t.Fatalf("expected missing %s error, got %v", factoryConfigFile, err)
			}
		})
	}
}
