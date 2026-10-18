package main

import (
	"errors"
	"os"
	"strings"
	"testing"

	"soft-factory/internal/config"
)

func TestSkillLoadFailureStopsBeforeImplementation(t *testing.T) {
	for _, stage := range []string{"codeReview", "securityReview", "riskClassification", "reviewChanges"} {
		for _, failure := range []string{"missing", "unreadable"} {
			t.Run(stage+"/"+failure, func(t *testing.T) {
				t.Chdir(t.TempDir())
				t.Setenv("PATH", t.TempDir())
				data := `{"agentSandbox":{},"stages":{"` + stage + `":{"customSkill":"project"}}}`
				if err := os.WriteFile(factoryConfigFile, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
				want := `project skill "project" not found`
				if failure == "unreadable" {
					// A directory fails ReadFile even when tests run as root.
					if err := os.MkdirAll(".agents/skills/project/SKILL.md", 0700); err != nil {
						t.Fatal(err)
					}
					want = "read project skill"
				}
				err := runWorkflow(workflowOptions{Implement: true})
				if err == nil || !strings.Contains(err.Error(), "stages."+stage+".customSkill: "+want) {
					t.Fatalf("expected skill loading error before execution, got %v", err)
				}
			})
		}
	}
}

func TestWorkflowSkipsUnselectedSkillFiles(t *testing.T) {
	for _, tc := range []struct {
		name, settings string
		start          config.Stage
	}{
		{"disabled code review", `"disabledStages":["codeReview"],"stages":{"codeReview":{"customSkill":"missing"}}`, ""},
		{"disabled security review", `"disabledStages":["securityReview"],"stages":{"securityReview":{"customSkill":"missing"}}`, ""},
		{"disabled risk classification", `"disabledStages":["riskClassification"],"stages":{"riskClassification":{"customSkill":"missing"}}`, ""},
		{"disabled review changes", `"disabledStages":["reviewChanges"],"stages":{"reviewChanges":{"customSkill":"missing"}}`, ""},
		{"resume after code review", `"stages":{"codeReview":{"customSkill":"missing"}}`, config.StageSecurityReview},
		{"resume after security review", `"stages":{"securityReview":{"customSkill":"missing"}}`, config.StageRiskClassification},
		{"resume after risk classification", `"stages":{"riskClassification":{"customSkill":"missing"}}`, config.StageReviewChanges},
		{"old mapping", `"customSkills":{"codeReview":"missing","securityReview":"missing","riskClassification":"missing","reviewChanges":"missing"}`, ""},
		{"implementation skill", `"stages":{"implementation":{"customSkill":"missing"}}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			t.Setenv("PATH", root)
			if err := os.WriteFile("agent-sandbox", []byte("#!/bin/sh\nexit 99\n"), 0700); err != nil {
				t.Fatal(err)
			}
			data := `{"agentSandbox":{},"documents":["missing-document"],` + tc.settings + `}`
			if err := os.WriteFile(factoryConfigFile, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			// Reaching document loading proves skill loading skipped these entries.
			err := runWorkflow(workflowOptions{StartStage: tc.start})
			if err == nil || !strings.Contains(err.Error(), `read supporting document: "missing-document"`) {
				t.Fatalf("expected document error after skipping skill files, got %v", err)
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
				if err := os.WriteFile("config.json", []byte(`{"stages":{"codeReview":{"customSkill":"missing"}}}`), 0644); err != nil {
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
