package config

import (
	"encoding/json"
	"os"
	"path/filepath"
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
		{name: "omitted", data: `{"agentSandbox":{}}`},
		{name: "empty block", data: `{"agentSandbox":{},"googleDrive":{}}`, wantErr: "at least one folder or file"},
		{name: "empty arrays", data: `{"agentSandbox":{},"googleDrive":{"folders":[],"files":[]}}`, wantErr: "at least one folder or file"},
		{name: "folders only", data: `{"agentSandbox":{},"googleDrive":{"folders":["test1"]}}`, want: &GoogleDriveConfig{Folders: []string{"test1"}}},
		{name: "files only", data: `{"agentSandbox":{},"googleDrive":{"files":["` + documentURL + `"]}}`, want: &GoogleDriveConfig{Files: []string{documentURL}}},
		{name: "files with empty folders", data: `{"agentSandbox":{},"googleDrive":{"folders":[],"files":["` + documentURL + `"]}}`, want: &GoogleDriveConfig{Folders: []string{}, Files: []string{documentURL}}},
		{name: "both", data: `{"agentSandbox":{},"googleDrive":{"folders":["test1"],"files":["` + documentURL + `"]}}`, want: &GoogleDriveConfig{Folders: []string{"test1"}, Files: []string{documentURL}}},
		{name: "blank folder with valid file", data: `{"agentSandbox":{},"googleDrive":{"folders":[" "],"files":["` + documentURL + `"]}}`, wantErr: "googleDrive.folders[0]"},
		{name: "blank file", data: `{"agentSandbox":{},"googleDrive":{"files":[" "]}}`, wantErr: "googleDrive.files[0]"},
		{name: "malformed URL", data: `{"agentSandbox":{},"googleDrive":{"files":["https://docs.google.com/document/d/%zz"]}}`, wantErr: "googleDrive.files[0]"},
		{name: "wrong host", data: `{"agentSandbox":{},"googleDrive":{"files":["https://example.com/document/d/abc123/edit"]}}`, wantErr: "googleDrive.files[0]"},
		{name: "missing ID", data: `{"agentSandbox":{},"googleDrive":{"files":["https://docs.google.com/document/d//edit"]}}`, wantErr: "googleDrive.files[0]"},
		{name: "invalid file with folders", data: `{"agentSandbox":{},"googleDrive":{"folders":["test1"],"files":["not-a-url"]}}`, wantErr: "googleDrive.files[0]"},
		{name: "invalid second file", data: `{"agentSandbox":{},"googleDrive":{"files":["` + documentURL + `","not-a-url"]}}`, wantErr: "googleDrive.files[1]"},
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
				data, err := json.Marshal(map[string]any{"agentSandbox": map[string]any{}, "stages": map[string]any{stage: map[string]string{"customSkill": name}}, "disabledStages": []string{stage}})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile("software-factory.json", data, 0600); err != nil {
					t.Fatal(err)
				}
				_, err = Load("software-factory.json")
				if err == nil || !strings.Contains(err.Error(), "stages."+stage+".customSkill must be") {
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
		{name: "omitted", data: `{"agentSandbox":{}}`},
		{name: "empty", data: `{"agentSandbox":{},"disabledStages":[]}`, want: []Stage{}},
		{name: "implementation", data: `{"agentSandbox":{},"disabledStages":["implementation"]}`, want: []Stage{StageImplementation}},
		{name: "codeReview", data: `{"agentSandbox":{},"disabledStages":["codeReview"]}`, want: []Stage{StageCodeReview}},
		{name: "securityReview", data: `{"agentSandbox":{},"disabledStages":["securityReview"]}`, want: []Stage{StageSecurityReview}},
		{name: "riskClassification", data: `{"agentSandbox":{},"disabledStages":["riskClassification"]}`, want: []Stage{StageRiskClassification}},
		{name: "reviewChanges", data: `{"agentSandbox":{},"disabledStages":["reviewChanges"]}`, want: []Stage{StageReviewChanges}},
		{
			name: "all",
			data: `{"agentSandbox":{},"disabledStages":["reviewChanges","riskClassification","securityReview","codeReview","implementation"]}`,
			want: []Stage{StageReviewChanges, StageRiskClassification, StageSecurityReview, StageCodeReview, StageImplementation},
		},
		{name: "duplicates", data: `{"agentSandbox":{},"disabledStages":["codeReview","codeReview"]}`, want: []Stage{StageCodeReview, StageCodeReview}},
		{name: "malformed JSON", data: `{"agentSandbox":{},"disabledStages":["codeReview"`, wantErr: "parse factory configuration"},
		{name: "null", data: `{"agentSandbox":{},"disabledStages":null}`, wantErr: "disabledStages must be an array"},
		{name: "string", data: `{"agentSandbox":{},"disabledStages":"codeReview"}`, wantErr: "disabledStages must be an array"},
		{name: "number", data: `{"agentSandbox":{},"disabledStages":1}`, wantErr: "disabledStages must be an array"},
		{name: "boolean", data: `{"agentSandbox":{},"disabledStages":true}`, wantErr: "disabledStages must be an array"},
		{name: "object", data: `{"agentSandbox":{},"disabledStages":{"codeReview":true}}`, wantErr: "disabledStages must be an array"},
		{name: "null entry", data: `{"agentSandbox":{},"disabledStages":["codeReview",null]}`, wantErr: "disabledStages[1] must be a string"},
		{name: "number entry", data: `{"agentSandbox":{},"disabledStages":[1]}`, wantErr: "disabledStages[0] must be a string"},
		{name: "array entry", data: `{"agentSandbox":{},"disabledStages":[["codeReview"]]}`, wantErr: "disabledStages[0] must be a string"},
		{name: "object entry", data: `{"agentSandbox":{},"disabledStages":[{}]}`, wantErr: "disabledStages[0] must be a string"},
		{name: "blank", data: `{"agentSandbox":{},"disabledStages":[""]}`, wantErr: `disabledStages[0]: unknown stage ""; ` + allowed},
		{name: "whitespace", data: `{"agentSandbox":{},"disabledStages":[" "]}`, wantErr: `disabledStages[0]: unknown stage " "`},
		{name: "padded", data: `{"agentSandbox":{},"disabledStages":[" codeReview"]}`, wantErr: `disabledStages[0]: unknown stage " codeReview"`},
		{name: "case mismatch", data: `{"agentSandbox":{},"disabledStages":["CodeReview"]}`, wantErr: `disabledStages[0]: unknown stage "CodeReview"`},
		{name: "kebab-case", data: `{"agentSandbox":{},"disabledStages":["security-review"]}`, wantErr: `disabledStages[0]: unknown stage "security-review"`},
		{name: "unknown after valid", data: `{"agentSandbox":{},"disabledStages":["codeReview","securityReveiw"]}`, wantErr: `disabledStages[1]: unknown stage "securityReveiw"; ` + allowed},
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
	data := `{"agentSandbox":{},"stages":{"codeReview":{"customSkill":"../escape"}},"disabledStages":["codeReview"]}`
	if err := os.WriteFile("software-factory.json", []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("software-factory.json"); err == nil || !strings.Contains(err.Error(), "stages.codeReview.customSkill must be") {
		t.Fatalf("expected custom skill validation error for disabled stage, got %v", err)
	}
}

func TestUnifiedSandboxConfiguration(t *testing.T) {
	for _, tt := range []struct {
		name, input string
		wantErr     bool
	}{
		{"shared and section pointers", `{"documents":["support.md"],"disabledStages":["reviewChanges"],"stages":{"implementation":{"agent":"stage-agent","model":"stage-model"},"codeReview":{"customSkill":"project"}},"agentSandbox":{"agent":"invalid-agent","model":"invalid-model","push":true,"file":["shared"],"run":{"filePrompt":"task path.txt","apiKey":"ignored","reviewers":["ignored"]},"resume":{"push":false,"branch":"","file":[],"apiKey":"ignored","reviewers":["ignored"]}}}`, false},
		{"absent", `{"documents":[]}`, true},
		{"null", `{"agentSandbox":null}`, false},
		{"empty", `{"agentSandbox":{}}`, false},
		{"unknown fields are not sandbox JSON validation", `{"agentSandbox":{"unknown":true}}`, false},
		{"bad JSON", `{"agentSandbox":`, true},
		{"bad shared type", `{"agentSandbox":{"push":"false"}}`, true},
		{"bad section type", `{"agentSandbox":{"resume":{"file":false}}}`, true},
		{"bad Factory property", `{"agentSandbox":{"agent":"anything"},"documents":[""]}`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile("software-factory.json", []byte(tt.input), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load("software-factory.json")
			if (err != nil) != tt.wantErr {
				t.Fatalf("load error = %v", err)
			}
			if tt.name == "absent" && !strings.Contains(err.Error(), "agentSandbox is required") {
				t.Fatalf("missing property error = %v", err)
			}
			if tt.name == "shared and section pointers" {
				if cfg.AgentSandbox.Agent != "invalid-agent" || cfg.AgentSandbox.Model != "invalid-model" || cfg.AgentSandbox.Resume.Push == nil || *cfg.AgentSandbox.Resume.Push || cfg.AgentSandbox.Resume.Branch == nil || *cfg.AgentSandbox.Resume.Branch != "" || cfg.AgentSandbox.Resume.File == nil || len(*cfg.AgentSandbox.Resume.File) != 0 || cfg.Stages.Implementation.Agent != "stage-agent" {
					t.Fatalf("lost unified settings: section pointers or Factory fields")
				}
			}
			if tt.name == "null" {
				if cfg.AgentSandbox.Run != nil || cfg.AgentSandbox.Agent != "" {
					t.Fatal("null sandbox settings should be empty")
				}
			}
		})
	}
}

func TestLegacyOnlyDoesNotSupplyFactoryConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("agent-sandbox.json", []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("software-factory.json"); err == nil {
		t.Fatal("legacy-only project unexpectedly loaded")
	}
}

func TestLoadReviewStageCustomSkill(t *testing.T) {
	for _, stage := range []string{"codeReview", "securityReview", "riskClassification", "reviewChanges"} {
		for _, tc := range []struct {
			name, entry, wantSkill, wantAgent, wantModel string
			wantErr                                      bool
		}{
			{name: "null stage", entry: "null"},
			{name: "omitted property", entry: `{}`},
			{name: "agent and model only", entry: `{"agent":"stage-agent","model":"stage-model"}`, wantAgent: "stage-agent", wantModel: "stage-model"},
			{name: "empty", entry: `{"customSkill":""}`},
			{name: "null", entry: `{"customSkill":null}`},
			{name: "skill only", entry: `{"customSkill":"Project-Skill"}`, wantSkill: "Project-Skill"},
			{name: "agent model and skill", entry: `{"agent":"stage-agent","model":"stage-model","customSkill":"project"}`, wantSkill: "project", wantAgent: "stage-agent", wantModel: "stage-model"},
			{name: "number", entry: `{"customSkill":1}`, wantErr: true},
			{name: "boolean", entry: `{"customSkill":false}`, wantErr: true},
			{name: "array", entry: `{"customSkill":[]}`, wantErr: true},
			{name: "object", entry: `{"customSkill":{}}`, wantErr: true},
		} {
			t.Run(stage+"/"+tc.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "software-factory.json")
				data := `{"agentSandbox":{},"stages":{"` + stage + `":` + tc.entry + `}}`
				if err := os.WriteFile(path, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
				cfg, err := Load(path)
				if tc.wantErr {
					if err == nil || !strings.Contains(err.Error(), "stages."+stage+".customSkill") {
						t.Fatalf("expected customSkill decoding error, got %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				got := map[string]ReviewStageConfig{
					"codeReview": cfg.Stages.CodeReview, "securityReview": cfg.Stages.SecurityReview,
					"riskClassification": cfg.Stages.RiskClassification, "reviewChanges": cfg.Stages.ReviewChanges,
				}[stage]
				if got.CustomSkill != tc.wantSkill || got.Agent != tc.wantAgent || got.Model != tc.wantModel {
					t.Fatalf("stage settings = %+v", got)
				}
			})
		}
	}
}

func TestLoadIgnoresUnsupportedCustomSkills(t *testing.T) {
	for _, input := range []string{
		`{"agentSandbox":{}}`,
		`{"agentSandbox":{},"customSkills":{"codeReview":"../invalid","securityReview":"missing","riskClassification":"missing","reviewChanges":"missing"}}`,
		`{"agentSandbox":{},"customSkills":false}`,
		`{"agentSandbox":{},"stages":{"implementation":{"agent":"codex","model":"example","customSkill":42}}}`,
	} {
		path := filepath.Join(t.TempDir(), "software-factory.json")
		if err := os.WriteFile(path, []byte(input), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Stages.CodeReview.CustomSkill != "" || cfg.Stages.SecurityReview.CustomSkill != "" || cfg.Stages.RiskClassification.CustomSkill != "" || cfg.Stages.ReviewChanges.CustomSkill != "" {
			t.Fatal("unsupported settings supplied a review skill")
		}
	}
}

func TestDocumentationConfigurationExamples(t *testing.T) {
	for _, doc := range []string{"../../README.md", "../../site-docs/guide/configuration.md"} {
		data, err := os.ReadFile(doc)
		if err != nil {
			t.Fatal(err)
		}
		for _, block := range strings.Split(string(data), "```json\n")[1:] {
			example := strings.SplitN(block, "```", 2)[0]
			var fields map[string]any
			if err := json.Unmarshal([]byte(example), &fields); err != nil {
				t.Fatalf("%s: %v", doc, err)
			}
			if fields["type"] == "service_account" {
				continue
			}
			// Partial configuration examples inherit an empty sandbox block.
			if _, ok := fields["agentSandbox"]; !ok {
				fields["agentSandbox"] = map[string]any{}
			}
			input, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "software-factory.json")
			if err := os.WriteFile(path, input, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err != nil {
				t.Fatalf("%s example: %v", doc, err)
			}
		}
	}
}
