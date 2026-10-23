package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

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
				data, err := json.Marshal(map[string]any{"agentSandbox": map[string]any{}, "stages": map[string]any{stage: map[string]string{"customSkill": name}}})
				require.NoError(t, err)
				require.NoError(t, os.WriteFile("software-factory.json", data, 0600))
				_, err = Load("software-factory.json")
				require.ErrorContains(t, err, "stages."+stage+".customSkill must be")
			})
		}
	}
}

func TestLoadStageDisabled(t *testing.T) {
	for _, stage := range Stages {
		for _, tc := range []struct {
			name, value   string
			want, invalid bool
		}{
			{"omitted", "", false, false},
			{"false", "false", false, false},
			{"true", "true", true, false},
			{"null", "null", false, false},
			{"string", `"true"`, false, true},
			{"number", "1", false, true},
			{"array", "[]", false, true},
			{"object", "{}", false, true},
		} {
			t.Run(string(stage)+"/"+tc.name, func(t *testing.T) {
				property := ""
				if tc.value != "" {
					property = `,"disabled":` + tc.value
				}
				data := `{"agentSandbox":{},"stages":{"` + string(stage) +
					`":{"agent":"codex","model":"stage-model"` + property + `}}}`
				path := filepath.Join(t.TempDir(), "software-factory.json")
				require.NoError(t, os.WriteFile(path, []byte(data), 0600))
				cfg, err := Load(path)
				if tc.invalid {
					require.ErrorContains(t, err, "parse factory configuration")
					var typeErr *json.UnmarshalTypeError
					require.True(t, errors.As(err, &typeErr))
					return
				}
				require.NoError(t, err)
				for _, other := range Stages {
					require.Equal(t, other == stage && tc.want, cfg.StageDisabled(other))
				}
				require.False(t, cfg.StageDisabled(Stage("unknown")))
			})
		}
	}
}

func TestLoadStageDisabledKeepsOtherValidation(t *testing.T) {
	t.Chdir(t.TempDir())
	data := `{"agentSandbox":{},"stages":{"codeReview":{"disabled":true,"customSkill":"../escape"}}}`
	require.NoError(t, os.WriteFile("software-factory.json", []byte(data), 0600))

	_, err := Load("software-factory.json")
	require.ErrorContains(t, err, "stages.codeReview.customSkill must be")
}

func TestUnifiedSandboxConfiguration(t *testing.T) {
	for _, tt := range []struct {
		name, input string
		wantErr     bool
	}{
		{"shared and section pointers", `{"documents":["support.md"],"stages":{"reviewChanges":{"disabled":true},"implementation":{"agent":"stage-agent","model":"stage-model"},"codeReview":{"customSkill":"project"}},"agentSandbox":{"agent":"invalid-agent","model":"invalid-model","push":true,"file":["shared"],"run":{"filePrompt":"task path.txt","apiKey":"ignored","reviewers":["ignored"]},"resume":{"push":false,"branch":"","file":[],"apiKey":"ignored","reviewers":["ignored"]}}}`, false},
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
				require.True(t, cfg.StageDisabled(StageReviewChanges))
				require.Equal(t, "invalid-agent", cfg.AgentSandbox.Agent)
				require.Equal(t, "invalid-model", cfg.AgentSandbox.Model)
				require.NotNil(t, cfg.AgentSandbox.Resume.Push)
				require.False(t, *cfg.AgentSandbox.Resume.Push)
				require.NotNil(t, cfg.AgentSandbox.Resume.Branch)
				require.Empty(t, *cfg.AgentSandbox.Resume.Branch)
				require.NotNil(t, cfg.AgentSandbox.Resume.File)
				require.Len(t, *cfg.AgentSandbox.Resume.File, 0)
				require.Equal(t, "stage-agent", cfg.Stages.Implementation.Agent)
				require.Equal(t, "project", cfg.Stages.CodeReview.CustomSkill)
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

func TestNestedCustomSkillConfiguration(t *testing.T) {
	for _, stage := range []string{"codeReview", "securityReview", "riskClassification", "reviewChanges"} {
		for _, tc := range []struct {
			name, value, want string
			invalid           bool
		}{
			{"omitted", "", "", false},
			{"empty", `""`, "", false},
			{"null", "null", "", false},
			{"name", `"project"`, "project", false},
			{"array", `["project"]`, "", true},
			{"object", `{}`, "", true},
			{"number", `1`, "", true},
			{"boolean", `true`, "", true},
		} {
			t.Run(stage+"/"+tc.name, func(t *testing.T) {
				t.Chdir(t.TempDir())
				property := ""
				if tc.value != "" {
					property = `,"customSkill":` + tc.value
				}
				data := `{"agentSandbox":{},"stages":{"` + stage + `":{"agent":"codex","model":"stage-model"` + property + `}}}`
				require.NoError(t, os.WriteFile("software-factory.json", []byte(data), 0600))
				cfg, err := Load("software-factory.json")
				if tc.invalid {
					require.ErrorContains(t, err, "parse factory configuration")
					return
				}
				require.NoError(t, err)
				var selection StageConfig
				var skill string
				switch stage {
				case "codeReview":
					selection, skill = cfg.Stages.CodeReview.StageConfig, cfg.Stages.CodeReview.CustomSkill
				case "securityReview":
					selection, skill = cfg.Stages.SecurityReview.StageConfig, cfg.Stages.SecurityReview.CustomSkill
				case "riskClassification":
					selection, skill = cfg.Stages.RiskClassification.StageConfig, cfg.Stages.RiskClassification.CustomSkill
				case "reviewChanges":
					selection, skill = cfg.Stages.ReviewChanges.StageConfig, cfg.Stages.ReviewChanges.CustomSkill
				}
				require.Equal(t, tc.want, skill)
				require.Equal(t, StageConfig{Agent: "codex", Model: "stage-model"}, selection)
			})
		}
	}
}

func TestTopLevelCustomSkillsIgnored(t *testing.T) {
	for _, legacy := range []string{`{"codeReview":"../invalid"}`, `"ignored"`, `[]`, `1`, `true`, `null`} {
		t.Run(legacy, func(t *testing.T) {
			t.Chdir(t.TempDir())
			for _, nested := range []string{"", "project"} {
				data := `{"agentSandbox":{},"customSkills":` + legacy + `,"stages":{"codeReview":{"customSkill":"` + nested + `"}}}`
				require.NoError(t, os.WriteFile("software-factory.json", []byte(data), 0600))
				cfg, err := Load("software-factory.json")
				require.NoError(t, err)
				require.Equal(t, nested, cfg.Stages.CodeReview.CustomSkill)
			}
		})
	}
}

func TestImplementationCustomSkillIgnored(t *testing.T) {
	for _, value := range []string{`"../invalid"`, `[]`, `{}`, `1`, `true`, `null`} {
		t.Run(value, func(t *testing.T) {
			t.Chdir(t.TempDir())
			data := `{"agentSandbox":{},"stages":{"implementation":{"agent":"codex","model":"stage-model","customSkill":` + value + `}}}`
			require.NoError(t, os.WriteFile("software-factory.json", []byte(data), 0600))
			cfg, err := Load("software-factory.json")
			require.NoError(t, err)
			require.Equal(t, StageConfig{Agent: "codex", Model: "stage-model"}, cfg.Stages.Implementation)
		})
	}
}
