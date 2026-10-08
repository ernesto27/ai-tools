package config

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"soft-factory/internal/googledrive"
)

type Config struct {
	Documents    []string           `json:"documents"`
	GoogleDrive  *GoogleDriveConfig `json:"googleDrive,omitempty"`
	CustomSkills CustomSkills       `json:"customSkills"`
}

type CustomSkills struct {
	CodeReview         string `json:"codeReview,omitempty"`
	SecurityReview     string `json:"securityReview,omitempty"`
	RiskClassification string `json:"riskClassification,omitempty"`
	ReviewChanges      string `json:"reviewChanges,omitempty"`
}

type GoogleDriveConfig struct {
	Folders []string `json:"folders"`
	Files   []string `json:"files"`
}

// SandboxConfig exposes only individual settings needed by the factory.
type SandboxConfig struct {
	sections map[string]json.RawMessage
}

// LoadSandbox reads the local agent-sandbox configuration without caching it.
func LoadSandbox() (SandboxConfig, error) {
	data, err := os.ReadFile("agent-sandbox.json")
	if err != nil {
		return SandboxConfig{}, fmt.Errorf("read agent-sandbox configuration: %w", err)
	}
	var cfg SandboxConfig
	if err := json.Unmarshal(data, &cfg.sections); err != nil {
		return SandboxConfig{}, fmt.Errorf("parse agent-sandbox configuration: %w", err)
	}
	return cfg, nil
}

// String returns an empty string for absent or non-string settings.
func (c SandboxConfig) String(section, key string) string {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(c.sections[section], &fields); err != nil {
		return ""
	}
	var value string
	if err := json.Unmarshal(fields[key], &value); err != nil {
		return ""
	}
	return value
}

// Branch selects the worktree branch for an implementation or resumed stage.
func (c SandboxConfig) Branch(mode string) (string, error) {
	if mode != "run" && mode != "resume" {
		return "", fmt.Errorf("unsupported stage command %q", mode)
	}
	branch := c.String(mode, "branch")
	if strings.TrimSpace(branch) == "" {
		return "", fmt.Errorf("%s.branch is required to locate the sandbox worktree", mode)
	}
	return branch, nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read factory configuration: %w", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return Config{}, fmt.Errorf("parse factory configuration: %w", err)
	}
	// Check names before decoding because encoding/json ignores unknown
	// fields and matches field names case-insensitively.
	if err := checkPropertyNames(fields); err != nil {
		return Config{}, fmt.Errorf("validate factory configuration: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse factory configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate factory configuration: %w", err)
	}

	return cfg, nil
}

// propertyNames lists the supported names of one configuration object and
// the obsolete names it rejects, mapped to their canonical paths.
type propertyNames struct {
	supported []string
	obsolete  map[string]string
}

var (
	topLevelProperties = propertyNames{
		supported: []string{"documents", "googleDrive", "customSkills"},
		obsolete: map[string]string{
			"google_drive":      "googleDrive",
			"custom-skills":     "customSkills",
			"code-review-skill": "customSkills.codeReview",
		},
	}
	nestedProperties = map[string]propertyNames{
		"googleDrive": {supported: []string{"folders", "files"}},
		"customSkills": {
			supported: []string{"codeReview", "securityReview", "riskClassification", "reviewChanges"},
			obsolete: map[string]string{
				"code-review":         "customSkills.codeReview",
				"security-review":     "customSkills.securityReview",
				"risk-classification": "customSkills.riskClassification",
				"review-changes":      "customSkills.reviewChanges",
			},
		},
	}
)

// checkPropertyNames rejects obsolete and incorrectly cased known property
// names regardless of their values. Other unknown names are ignored.
func checkPropertyNames(fields map[string]json.RawMessage) error {
	if err := checkObjectNames("", fields, topLevelProperties); err != nil {
		return err
	}
	for _, parent := range []string{"googleDrive", "customSkills"} {
		var nested map[string]json.RawMessage
		// Non-object values are reported by the typed decode.
		if json.Unmarshal(fields[parent], &nested) != nil {
			continue
		}
		if err := checkObjectNames(parent+".", nested, nestedProperties[parent]); err != nil {
			return err
		}
	}
	return nil
}

func checkObjectNames(prefix string, fields map[string]json.RawMessage, names propertyNames) error {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	for _, key := range keys {
		if slices.Contains(names.supported, key) {
			continue
		}
		replacement := ""
		for _, name := range names.supported {
			if strings.EqualFold(key, name) {
				replacement = prefix + name
			}
		}
		for name, canonical := range names.obsolete {
			if strings.EqualFold(key, name) {
				replacement = canonical
			}
		}
		if replacement != "" {
			return fmt.Errorf("%s%s is unsupported; use %s", prefix, key, replacement)
		}
	}
	return nil
}

func (c Config) Validate() error {
	skillNames := []struct{ path, name string }{
		{"customSkills.codeReview", c.CustomSkills.CodeReview},
		{"customSkills.securityReview", c.CustomSkills.SecurityReview},
		{"customSkills.riskClassification", c.CustomSkills.RiskClassification},
		{"customSkills.reviewChanges", c.CustomSkills.ReviewChanges},
	}

	for _, skill := range skillNames {
		if skill.name == "" {
			continue
		}

		if strings.TrimSpace(skill.name) == "" ||
			skill.name == "." ||
			skill.name == ".." ||
			strings.ContainsAny(skill.name, `/\`) {
			return fmt.Errorf("%s must be a single nonempty skill name", skill.path)
		}
	}

	for i, path := range c.Documents {
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("documents[%d] must not be empty", i)
		}
	}
	if c.GoogleDrive != nil {
		if len(c.GoogleDrive.Folders) == 0 && len(c.GoogleDrive.Files) == 0 {
			return fmt.Errorf("googleDrive requires at least one folder or file")
		}
		for i, name := range c.GoogleDrive.Folders {
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("googleDrive.folders[%d] must not be empty", i)
			}
		}
		for i, fileURL := range c.GoogleDrive.Files {
			if _, err := googledrive.DocumentIDFromURL(fileURL); err != nil {
				return fmt.Errorf("googleDrive.files[%d]: %w", i, err)
			}
		}
	}

	return nil
}
