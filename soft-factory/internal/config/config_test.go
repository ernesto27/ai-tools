package config

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			require.NoError(t, os.WriteFile("software-factory.json", []byte(tc.data), 0600))
			cfg, err := Load("software-factory.json")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, "validate factory configuration:")
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, cfg.GoogleDrive)
		})
	}
}

func TestLoadRejectsInvalidCustomSkillNames(t *testing.T) {
	for _, stage := range []string{"codeReview", "securityReview", "riskClassification", "reviewChanges"} {
		for _, name := range []string{" ", ".", "..", "nested/skill", `nested\skill`} {
			t.Run(stage+"/"+name, func(t *testing.T) {
				t.Chdir(t.TempDir())
				// Marshal names to preserve backslashes in JSON.
				data, err := json.Marshal(map[string]any{"agentSandbox": map[string]any{}, "customSkills": map[string]string{stage: name}})
				require.NoError(t, err)
				require.NoError(t, os.WriteFile("software-factory.json", data, 0600))
				_, err = Load("software-factory.json")
				require.ErrorContains(t, err, "customSkills."+stage+" must be")
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
			require.NoError(t, os.WriteFile("software-factory.json", []byte(tc.data), 0600))
			cfg, err := Load("software-factory.json")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				if !strings.Contains(tc.wantErr, "parse") {
					require.ErrorContains(t, err, "validate factory configuration: disabledStages")
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, cfg.DisabledStages)
			for _, stage := range Stages {
				got, want := cfg.StageDisabled(stage), slices.Contains(tc.want, stage)
				assert.Equal(t, want, got, "stage %s", stage)
			}
		})
	}
}

func TestLoadDisabledStagesKeepsOtherValidation(t *testing.T) {
	t.Chdir(t.TempDir())
	data := `{"agentSandbox":{},"customSkills":{"codeReview":"../escape"},"disabledStages":["codeReview"]}`
	require.NoError(t, os.WriteFile("software-factory.json", []byte(data), 0600))

	_, err := Load("software-factory.json")
	require.ErrorContains(t, err, "customSkills.codeReview must be")
}

func TestUnifiedSandboxConfiguration(t *testing.T) {
	for _, tt := range []struct {
		name, input string
		wantErr     bool
	}{
		{"shared and section pointers", `{"documents":["support.md"],"customSkills":{"codeReview":"project"},"disabledStages":["reviewChanges"],"stages":{"implementation":{"agent":"stage-agent","model":"stage-model"}},"agentSandbox":{"agent":"invalid-agent","model":"invalid-model","push":true,"file":["shared"],"run":{"filePrompt":"task path.txt","apiKey":"ignored","reviewers":["ignored"]},"resume":{"push":false,"branch":"","file":[],"apiKey":"ignored","reviewers":["ignored"]}}}`, false},
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
			require.NoError(t, os.WriteFile("software-factory.json", []byte(tt.input), 0600))
			cfg, err := Load("software-factory.json")
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if tt.name == "absent" {
				require.ErrorContains(t, err, "agentSandbox is required")
			}
			if tt.name == "shared and section pointers" {
				require.Equal(t, "invalid-agent", cfg.AgentSandbox.Agent)
				require.Equal(t, "invalid-model", cfg.AgentSandbox.Model)
				require.NotNil(t, cfg.AgentSandbox.Resume.Push)
				require.False(t, *cfg.AgentSandbox.Resume.Push)
				require.NotNil(t, cfg.AgentSandbox.Resume.Branch)
				require.Empty(t, *cfg.AgentSandbox.Resume.Branch)
				require.NotNil(t, cfg.AgentSandbox.Resume.File)
				require.Len(t, *cfg.AgentSandbox.Resume.File, 0)
				require.Equal(t, "stage-agent", cfg.Stages.Implementation.Agent)
			}
			if tt.name == "null" {
				require.Nil(t, cfg.AgentSandbox.Run)
				require.Empty(t, cfg.AgentSandbox.Agent)
			}
		})
	}
}

func TestLegacyOnlyDoesNotSupplyFactoryConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile("agent-sandbox.json", []byte(`{}`), 0600))

	_, err := Load("software-factory.json")
	require.Error(t, err)
}
