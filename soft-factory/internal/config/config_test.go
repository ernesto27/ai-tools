package config

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
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

func TestLoadCustomSkills(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
		want CustomSkills
	}{
		{name: "omitted", data: `{}`},
		{name: "empty", data: `{"customSkills":{}}`},
		{name: "empty names", data: `{"customSkills":{"codeReview":"","securityReview":"","riskClassification":"","reviewChanges":""}}`},
		{
			name: "all stages",
			data: `{"customSkills":{"codeReview":"go-tui-review","securityReview":"my-security","riskClassification":"my-risk","reviewChanges":"my-walkthrough"}}`,
			want: CustomSkills{
				CodeReview: "go-tui-review", SecurityReview: "my-security",
				RiskClassification: "my-risk", ReviewChanges: "my-walkthrough",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile("software-factory.json", []byte(tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load("software-factory.json")
			if err != nil {
				t.Fatal(err)
			}
			if cfg.CustomSkills != tc.want {
				t.Fatalf("CustomSkills = %+v; want %+v", cfg.CustomSkills, tc.want)
			}
		})
	}
}

func TestLoadRejectsInvalidCustomSkillNames(t *testing.T) {
	for _, property := range []string{"codeReview", "securityReview", "riskClassification", "reviewChanges"} {
		for _, name := range []string{" ", ".", "..", "nested/skill", `nested\skill`} {
			t.Run(property+"/"+name, func(t *testing.T) {
				t.Chdir(t.TempDir())
				// Marshal names to preserve backslashes in JSON.
				data, err := json.Marshal(map[string]any{"customSkills": map[string]string{property: name}})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile("software-factory.json", data, 0600); err != nil {
					t.Fatal(err)
				}
				_, err = Load("software-factory.json")
				if err == nil || !strings.Contains(err.Error(), "customSkills."+property) {
					t.Fatalf("expected validation error for %s = %q, got %v", property, name, err)
				}
			})
		}
	}
}

func TestLoadCanonicalExample(t *testing.T) {
	t.Chdir(t.TempDir())
	const documentURL = "https://docs.google.com/document/d/example-document-id/edit"
	data := `{
		"documents": ["task.txt"],
		"googleDrive": {"folders": ["Project documents"], "files": ["` + documentURL + `"]},
		"customSkills": {
			"codeReview": "test-code-review",
			"securityReview": "test-security-review",
			"riskClassification": "test-risk-classification",
			"reviewChanges": "test-review-changes"
		}
	}`
	if err := os.WriteFile("software-factory.json", []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("software-factory.json")
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Documents:   []string{"task.txt"},
		GoogleDrive: &GoogleDriveConfig{Folders: []string{"Project documents"}, Files: []string{documentURL}},
		CustomSkills: CustomSkills{
			CodeReview: "test-code-review", SecurityReview: "test-security-review",
			RiskClassification: "test-risk-classification", ReviewChanges: "test-review-changes",
		},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("Load() = %+v; want %+v", cfg, want)
	}
}

func TestLoadRejectsObsoleteProperties(t *testing.T) {
	const drive = `{"folders":["test1"]}`
	for _, tc := range []struct {
		name    string
		data    string
		wantErr string
	}{
		{name: "google_drive", data: `{"google_drive":` + drive + `}`, wantErr: "google_drive is unsupported; use googleDrive"},
		{name: "google_drive null", data: `{"google_drive":null}`, wantErr: "google_drive is unsupported; use googleDrive"},
		{name: "google_drive empty", data: `{"google_drive":{}}`, wantErr: "google_drive is unsupported; use googleDrive"},
		{name: "google_drive with googleDrive", data: `{"googleDrive":` + drive + `,"google_drive":` + drive + `}`, wantErr: "google_drive is unsupported; use googleDrive"},
		{name: "custom-skills", data: `{"custom-skills":{"code-review":"skill"}}`, wantErr: "custom-skills is unsupported; use customSkills"},
		{name: "custom-skills null", data: `{"custom-skills":null}`, wantErr: "custom-skills is unsupported; use customSkills"},
		{name: "custom-skills empty", data: `{"custom-skills":{}}`, wantErr: "custom-skills is unsupported; use customSkills"},
		{name: "custom-skills with customSkills", data: `{"customSkills":{"codeReview":"skill"},"custom-skills":{}}`, wantErr: "custom-skills is unsupported; use customSkills"},
		{name: "code-review-skill", data: `{"code-review-skill":"skill"}`, wantErr: "code-review-skill is unsupported; use customSkills.codeReview"},
		{name: "code-review-skill null", data: `{"code-review-skill":null}`, wantErr: "code-review-skill is unsupported"},
		{name: "code-review-skill empty", data: `{"code-review-skill":""}`, wantErr: "code-review-skill is unsupported"},
		{name: "code-review-skill with customSkills", data: `{"customSkills":{"codeReview":"skill"},"code-review-skill":"skill"}`, wantErr: "code-review-skill is unsupported"},
		{name: "nested code-review", data: `{"customSkills":{"code-review":"skill"}}`, wantErr: "customSkills.code-review is unsupported; use customSkills.codeReview"},
		{name: "nested security-review null", data: `{"customSkills":{"security-review":null}}`, wantErr: "customSkills.security-review is unsupported; use customSkills.securityReview"},
		{name: "nested risk-classification empty", data: `{"customSkills":{"risk-classification":""}}`, wantErr: "customSkills.risk-classification is unsupported; use customSkills.riskClassification"},
		{name: "nested review-changes with reviewChanges", data: `{"customSkills":{"reviewChanges":"skill","review-changes":"skill"}}`, wantErr: "customSkills.review-changes is unsupported; use customSkills.reviewChanges"},
		{name: "documents casing", data: `{"Documents":["task.txt"]}`, wantErr: "Documents is unsupported; use documents"},
		{name: "googleDrive casing", data: `{"googledrive":` + drive + `}`, wantErr: "googledrive is unsupported; use googleDrive"},
		{name: "customSkills casing", data: `{"CustomSkills":{}}`, wantErr: "CustomSkills is unsupported; use customSkills"},
		{name: "obsolete casing", data: `{"Google_Drive":` + drive + `}`, wantErr: "Google_Drive is unsupported; use googleDrive"},
		{name: "folders casing", data: `{"googleDrive":{"Folders":["test1"]}}`, wantErr: "googleDrive.Folders is unsupported; use googleDrive.folders"},
		{name: "files casing", data: `{"googleDrive":{"folders":["test1"],"FILES":[]}}`, wantErr: "googleDrive.FILES is unsupported; use googleDrive.files"},
		{name: "codeReview casing", data: `{"customSkills":{"codereview":"skill"}}`, wantErr: "customSkills.codereview is unsupported; use customSkills.codeReview"},
		{name: "reviewChanges casing", data: `{"customSkills":{"ReviewChanges":"skill"}}`, wantErr: "customSkills.ReviewChanges is unsupported; use customSkills.reviewChanges"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile("software-factory.json", []byte(tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Load("software-factory.json")
			if err == nil || !strings.Contains(err.Error(), "validate factory configuration: "+tc.wantErr) {
				t.Fatalf("expected configuration error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestLoadSandboxSettings(t *testing.T) {
	t.Chdir(t.TempDir())
	data := `{"run":{"branch":"implementation","file-prompt":"task.txt","agent":"codex","model":"model","push":false},"resume":{"branch":"reviews","agent":123},"version":1}`
	if err := os.WriteFile("agent-sandbox.json", []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadSandbox()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ section, key, want string }{
		{"run", "file-prompt", "task.txt"},
		{"run", "agent", "codex"},
		{"run", "model", "model"},
		{"run", "push", ""},
		{"resume", "agent", ""},
		{"missing", "branch", ""},
		{"version", "branch", ""},
	} {
		if got := cfg.String(tc.section, tc.key); got != tc.want {
			t.Errorf("String(%q, %q) = %q; want %q", tc.section, tc.key, got, tc.want)
		}
	}
	for _, tc := range []struct{ mode, want string }{{"run", "implementation"}, {"resume", "reviews"}} {
		got, err := cfg.Branch(tc.mode)
		if err != nil || got != tc.want {
			t.Errorf("Branch(%q) = %q, %v; want %q", tc.mode, got, err, tc.want)
		}
	}
	if _, err := cfg.Branch("other"); err == nil {
		t.Fatal("unsupported mode accepted")
	}
}

func TestLoadSandboxErrorsAndReload(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := LoadSandbox(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing configuration error = %v", err)
	}
	for _, data := range []string{"{", "[]"} {
		if err := os.WriteFile("agent-sandbox.json", []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadSandbox(); err == nil {
			t.Fatalf("invalid configuration %q accepted", data)
		}
	}
	for _, data := range []string{`{}`, `{"run":{"branch":" "}}`, `{"run":{"branch":123}}`} {
		if err := os.WriteFile("agent-sandbox.json", []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadSandbox()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := cfg.Branch("run"); err == nil {
			t.Fatalf("invalid branch in %s accepted", data)
		}
	}
	if err := os.WriteFile("agent-sandbox.json", []byte(`{"run":{"branch":"updated"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadSandbox()
	if err != nil {
		t.Fatal(err)
	}
	if branch, err := cfg.Branch("run"); err != nil || branch != "updated" {
		t.Fatalf("updated configuration not loaded: %q, %v", branch, err)
	}
}
