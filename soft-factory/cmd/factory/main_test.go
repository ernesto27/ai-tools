package main

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestMissingSkillStopsBeforeImplementation(t *testing.T) {
	for _, property := range []string{"codeReview", "securityReview", "riskClassification", "reviewChanges"} {
		t.Run(property, func(t *testing.T) {
			t.Chdir(t.TempDir())
			data := `{"customSkills":{"` + property + `":"missing"}}`
			if err := os.WriteFile(factoryConfigFile, []byte(data), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile("config.json", []byte(`{`), 0644); err != nil {
				t.Fatal(err)
			}

			err := runWorkflow(workflowOptions{Implement: true})
			want := "customSkills." + property + `: project skill "missing" not found`
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("expected missing-skill error %q first, got %v", want, err)
			}
		})
	}
}

func TestObsoletePropertyStopsBeforeSkillsAndDocuments(t *testing.T) {
	for _, tc := range []struct{ data, want string }{
		{`{"custom-skills":{"code-review":"missing"}}`, "custom-skills is unsupported; use customSkills"},
		{`{"customSkills":{"code-review":"missing"}}`, "customSkills.code-review is unsupported; use customSkills.codeReview"},
		{`{"customSkills":{"codeReview":"missing"},"google_drive":{"folders":["test1"]}}`, "google_drive is unsupported; use googleDrive"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile(factoryConfigFile, []byte(tc.data), 0644); err != nil {
				t.Fatal(err)
			}
			// Skill resolution or Drive setup would report a different error.
			err := runWorkflow(workflowOptions{Implement: true})
			if err == nil || err.Error() != "validate factory configuration: "+tc.want {
				t.Fatalf("expected configuration error %q, got %v", tc.want, err)
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
