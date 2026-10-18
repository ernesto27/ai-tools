package config

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestLoadGoogleDrive(t *testing.T) {
	const documentURL = "https://docs.google.com/document/d/abc123/edit?tab=t.0"
	for _, tc := range []struct {
		name    string
		data    string
		want    *GoogleDriveConfig
		wantErr string
	}{
		{name: "omitted", data: `{}`},
		{name: "empty block", data: `{"googleDrive":{}}`, wantErr: "at least one folder or file"},
		{name: "empty arrays", data: `{"googleDrive":{"folders":[],"files":[]}}`, wantErr: "at least one folder or file"},
		{name: "folders only", data: `{"googleDrive":{"folders":["test1"]}}`, want: &GoogleDriveConfig{Folders: []string{"test1"}}},
		{name: "files only", data: `{"googleDrive":{"files":["` + documentURL + `"]}}`, want: &GoogleDriveConfig{Files: []string{documentURL}}},
		{name: "files with empty folders", data: `{"googleDrive":{"folders":[],"files":["` + documentURL + `"]}}`, want: &GoogleDriveConfig{Folders: []string{}, Files: []string{documentURL}}},
		{name: "both", data: `{"googleDrive":{"folders":["test1"],"files":["` + documentURL + `"]}}`, want: &GoogleDriveConfig{Folders: []string{"test1"}, Files: []string{documentURL}}},
		{name: "blank folder with valid file", data: `{"googleDrive":{"folders":[" "],"files":["` + documentURL + `"]}}`, wantErr: "googleDrive.folders[0]"},
		{name: "blank file", data: `{"googleDrive":{"files":[" "]}}`, wantErr: "googleDrive.files[0]"},
		{name: "malformed URL", data: `{"googleDrive":{"files":["https://docs.google.com/document/d/%zz"]}}`, wantErr: "googleDrive.files[0]"},
		{name: "wrong host", data: `{"googleDrive":{"files":["https://example.com/document/d/abc123/edit"]}}`, wantErr: "googleDrive.files[0]"},
		{name: "missing ID", data: `{"googleDrive":{"files":["https://docs.google.com/document/d//edit"]}}`, wantErr: "googleDrive.files[0]"},
		{name: "invalid file with folders", data: `{"googleDrive":{"folders":["test1"],"files":["not-a-url"]}}`, wantErr: "googleDrive.files[0]"},
		{name: "invalid second file", data: `{"googleDrive":{"files":["` + documentURL + `","not-a-url"]}}`, wantErr: "googleDrive.files[1]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile("software-factory.json", []byte(tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load("software-factory.json")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), "validate factory configuration:") || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected configuration validation error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(cfg.GoogleDrive, tc.want) {
				t.Fatalf("GoogleDrive = %+v; want %+v", cfg.GoogleDrive, tc.want)
			}
		})
	}
}

func TestLoadRejectsInvalidCustomSkillNames(t *testing.T) {
	for _, stage := range []string{"codeReview", "securityReview", "riskClassification", "reviewChanges"} {
		for _, name := range []string{" ", ".", "..", "nested/skill", `nested\skill`} {
			t.Run(stage+"/"+name, func(t *testing.T) {
				t.Chdir(t.TempDir())
				// Marshal names to preserve backslashes in JSON.
				data, err := json.Marshal(map[string]any{"customSkills": map[string]string{stage: name}})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile("software-factory.json", data, 0600); err != nil {
					t.Fatal(err)
				}
				_, err = Load("software-factory.json")
				if err == nil || !strings.Contains(err.Error(), "customSkills."+stage+" must be") {
					t.Fatalf("expected validation error for %s = %q, got %v", stage, name, err)
				}
			})
		}
	}
}

func TestLoadDisabledStages(t *testing.T) {
	const allowed = "allowed: implementation, codeReview, securityReview, riskClassification, reviewChanges"
	for _, tc := range []struct {
		name    string
		data    string
		want    []Stage
		wantErr string
	}{
		{name: "omitted", data: `{}`},
		{name: "empty", data: `{"disabledStages":[]}`, want: []Stage{}},
		{name: "implementation", data: `{"disabledStages":["implementation"]}`, want: []Stage{StageImplementation}},
		{name: "codeReview", data: `{"disabledStages":["codeReview"]}`, want: []Stage{StageCodeReview}},
		{name: "securityReview", data: `{"disabledStages":["securityReview"]}`, want: []Stage{StageSecurityReview}},
		{name: "riskClassification", data: `{"disabledStages":["riskClassification"]}`, want: []Stage{StageRiskClassification}},
		{name: "reviewChanges", data: `{"disabledStages":["reviewChanges"]}`, want: []Stage{StageReviewChanges}},
		{
			name: "all",
			data: `{"disabledStages":["reviewChanges","riskClassification","securityReview","codeReview","implementation"]}`,
			want: []Stage{StageReviewChanges, StageRiskClassification, StageSecurityReview, StageCodeReview, StageImplementation},
		},
		{name: "duplicates", data: `{"disabledStages":["codeReview","codeReview"]}`, want: []Stage{StageCodeReview, StageCodeReview}},
		{name: "malformed JSON", data: `{"disabledStages":["codeReview"`, wantErr: "parse factory configuration"},
		{name: "null", data: `{"disabledStages":null}`, wantErr: "disabledStages must be an array"},
		{name: "string", data: `{"disabledStages":"codeReview"}`, wantErr: "disabledStages must be an array"},
		{name: "number", data: `{"disabledStages":1}`, wantErr: "disabledStages must be an array"},
		{name: "boolean", data: `{"disabledStages":true}`, wantErr: "disabledStages must be an array"},
		{name: "object", data: `{"disabledStages":{"codeReview":true}}`, wantErr: "disabledStages must be an array"},
		{name: "null entry", data: `{"disabledStages":["codeReview",null]}`, wantErr: "disabledStages[1] must be a string"},
		{name: "number entry", data: `{"disabledStages":[1]}`, wantErr: "disabledStages[0] must be a string"},
		{name: "array entry", data: `{"disabledStages":[["codeReview"]]}`, wantErr: "disabledStages[0] must be a string"},
		{name: "object entry", data: `{"disabledStages":[{}]}`, wantErr: "disabledStages[0] must be a string"},
		{name: "blank", data: `{"disabledStages":[""]}`, wantErr: `disabledStages[0]: unknown stage ""; ` + allowed},
		{name: "whitespace", data: `{"disabledStages":[" "]}`, wantErr: `disabledStages[0]: unknown stage " "`},
		{name: "padded", data: `{"disabledStages":[" codeReview"]}`, wantErr: `disabledStages[0]: unknown stage " codeReview"`},
		{name: "case mismatch", data: `{"disabledStages":["CodeReview"]}`, wantErr: `disabledStages[0]: unknown stage "CodeReview"`},
		{name: "kebab-case", data: `{"disabledStages":["security-review"]}`, wantErr: `disabledStages[0]: unknown stage "security-review"`},
		{name: "unknown after valid", data: `{"disabledStages":["codeReview","securityReveiw"]}`, wantErr: `disabledStages[1]: unknown stage "securityReveiw"; ` + allowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile("software-factory.json", []byte(tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load("software-factory.json")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				if !strings.Contains(tc.wantErr, "parse") && !strings.Contains(err.Error(), "validate factory configuration: disabledStages") {
					t.Fatalf("error %q does not identify disabledStages validation", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(cfg.DisabledStages, tc.want) {
				t.Fatalf("DisabledStages = %#v; want %#v", cfg.DisabledStages, tc.want)
			}
			for _, stage := range Stages {
				if got, want := cfg.StageDisabled(stage), slices.Contains(tc.want, stage); got != want {
					t.Errorf("StageDisabled(%q) = %v; want %v", stage, got, want)
				}
			}
		})
	}
}

func TestLoadDisabledStagesKeepsOtherValidation(t *testing.T) {
	t.Chdir(t.TempDir())
	data := `{"customSkills":{"codeReview":"../escape"},"disabledStages":["codeReview"]}`
	if err := os.WriteFile("software-factory.json", []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("software-factory.json"); err == nil || !strings.Contains(err.Error(), "customSkills.codeReview must be") {
		t.Fatalf("expected custom skill validation error for disabled stage, got %v", err)
	}
}

func TestUnifiedSandboxSnapshot(t *testing.T) {
	t.Chdir(t.TempDir())
	const embedded = `{"agent":"not-an-agent","model":"not-a-model","baseImage":"image","branch":"shared","push":true,"run":{"branch":"","filePrompt":"task.txt","apiKey":"secret","image":[],"model":null},"resume":{"push":false,"agent":"","model":""},"future":{"value":null}}`
	data := `{"documents":["support.txt"],"stages":{"implementation":{"agent":"codex"},"codeReview":{"model":"review-model"}},"agentSandbox":` + embedded + `}`
	if err := os.WriteFile("software-factory.json", []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("software-factory.json")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AgentSandbox.Agent != "not-an-agent" || cfg.AgentSandbox.Model != "not-a-model" {
		t.Fatal("semantic settings were changed")
	}
	if cfg.AgentSandbox.Run.Branch == nil || *cfg.AgentSandbox.Run.Branch != "" || cfg.AgentSandbox.Run.Model != nil || cfg.AgentSandbox.Run.Image == nil || len(*cfg.AgentSandbox.Run.Image) != 0 {
		t.Fatal("run pointer semantics lost")
	}
	if cfg.AgentSandbox.Resume.Push == nil || *cfg.AgentSandbox.Resume.Push || cfg.AgentSandbox.Resume.Agent == nil || *cfg.AgentSandbox.Resume.Agent != "" {
		t.Fatal("resume pointer semantics lost")
	}
	if cfg.Stages.Implementation.Agent != "codex" || cfg.Stages.CodeReview.Model != "review-model" || cfg.Documents[0] != "support.txt" {
		t.Fatal("Factory settings lost")
	}
	if string(cfg.SandboxJSON()) != embedded {
		t.Fatal("transport changed original JSON")
	}
	copy := cfg.SandboxJSON()
	copy[0] = 'x'
	if string(cfg.SandboxJSON()) != embedded {
		t.Fatal("transport exposes mutable snapshot")
	}
}

func TestUnifiedSandboxNoLegacyFallback(t *testing.T) {
	for _, legacy := range []string{`{"agent":"claude","run":{"filePrompt":"legacy.txt"}}`, `{malformed`} {
		for _, data := range []string{`{}`, `{"agentSandbox":null}`} {
			t.Run(legacy+data, func(t *testing.T) {
				t.Chdir(t.TempDir())
				if err := os.WriteFile("agent-sandbox.json", []byte(legacy), 0640); err != nil {
					t.Fatal(err)
				}
				if _, err := Load("software-factory.json"); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("legacy satisfied Factory input: %v", err)
				}
				if err := os.WriteFile("software-factory.json", []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
				cfg, err := Load("software-factory.json")
				if err != nil {
					t.Fatal(err)
				}
				if cfg.AgentSandbox.Agent != "" || cfg.AgentSandbox.Run != nil || string(cfg.SandboxJSON()) != "{}" {
					t.Fatal("legacy fallback or defaults added")
				}
			})
		}
	}
}

func TestUnifiedSandboxDecodeErrors(t *testing.T) {
	for _, data := range []string{`{"agentSandbox":`, `{"agentSandbox":[]}`, `{"agentSandbox":{"run":{"push":"false"}}}`, `{"agentSandbox":{"agent":123}}`} {
		t.Run(data, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile("software-factory.json", []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load("software-factory.json"); err == nil || !strings.Contains(err.Error(), "parse factory configuration") {
				t.Fatalf("decode error = %v", err)
			}
		})
	}
}

func TestUnifiedSnapshotRepeatedObject(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, last := range []string{`null`, `{}`, `{"agent":"final"}`} {
		data := `{"agentSandbox":{"agent":"discarded","run":{"filePrompt":"discarded.txt"}},"agentSandbox":` + last + `}`
		if err := os.WriteFile("software-factory.json", []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load("software-factory.json")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AgentSandbox.Run != nil || cfg.AgentSandbox.Agent == "discarded" {
			t.Fatal("Factory and transport disagree on final object")
		}
		want := last
		if last == "null" {
			want = "{}"
		}
		if string(cfg.SandboxJSON()) != want {
			t.Fatalf("transport = %s", cfg.SandboxJSON())
		}
	}
}
