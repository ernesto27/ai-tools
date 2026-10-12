package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
	"soft-factory/internal/sandbox"
)

func TestStageSummariesUseRuntimeSelection(t *testing.T) {
	for _, command := range []string{executionlog.CommandRun, executionlog.CommandContinue} {
		t.Run(command, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			bin := filepath.Join(dir, "bin")
			if err := os.Mkdir(bin, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\nprintf '%s\\n' 'test-exclude'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin)
			run, err := executionlog.NewRun("feature", command, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer run.Finish(nil)
			input := sandbox.TaskContext{Context: context.Background(), Log: run}
			for i, stage := range []struct {
				id   config.Stage
				name string
			}{
				{config.StageImplementation, "implementation"},
				{config.StageCodeReview, "code-review"},
				{config.StageSecurityReview, "security-review"},
				{config.StageRiskClassification, "risk-classification"},
			} {
				err := runStage(input, config.Config{}, stage.id, stage.name, "Starting stage", func() error {
					run.RuntimeSelection("earlier-agent", "earlier-model")
					run.RuntimeSelection("runtime-agent", fmt.Sprintf("model-%d", i))
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			cfg := config.Config{DisabledStages: []config.Stage{config.StageCodeReview}}
			if err := runStage(input, cfg, config.StageCodeReview, "code-review", "", func() error {
				t.Fatal("disabled stage executed")
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(run.Path(), "summary.log"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(data), "   Agent:     runtime-agent\n") != 4 || strings.Count(string(data), "   Model:     model-") != 4 || strings.Contains(string(data), "earlier-") || !strings.HasSuffix(string(data), "Skipping disabled stage: codeReview\n\n") {
				t.Fatalf("unexpected workflow summary: %s", data)
			}
		})
	}
}

func TestMissingSkillStopsBeforeImplementation(t *testing.T) {
	for _, stage := range []string{"codeReview", "securityReview", "riskClassification", "reviewChanges"} {
		t.Run(stage, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("PATH", t.TempDir())
			data := `{"customSkills":{"` + stage + `":"missing"}}`
			if err := os.WriteFile(factoryConfigFile, []byte(data), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile("config.json", []byte(`{`), 0644); err != nil {
				t.Fatal(err)
			}

			err := runWorkflow(workflowOptions{Implement: true})
			want := "customSkills." + stage + `: project skill "missing" not found`
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("expected missing-skill error %q first, got %v", want, err)
			}
		})
	}
}

func TestWorkflowDoesNotFallBackToLegacyConfig(t *testing.T) {
	for _, implement := range []bool{false, true} {
		for _, invalid := range []bool{false, true} {
			name := "review"
			if implement {
				name = "implementation"
			}
			if invalid {
				name += "/invalid"
			} else {
				name += "/missing"
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
				err := runWorkflow(workflowOptions{Implement: implement})
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
}
