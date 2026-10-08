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

func (c Config) Validate() error {
	skillNames := map[string]string{
		"codeReview":         c.CustomSkills.CodeReview,
		"securityReview":     c.CustomSkills.SecurityReview,
		"riskClassification": c.CustomSkills.RiskClassification,
		"reviewChanges":      c.CustomSkills.ReviewChanges,
	}

	for property, name := range skillNames {
		if name == "" {
			continue
		}

		if strings.TrimSpace(name) == "" ||
			name == "." ||
			name == ".." ||
			strings.ContainsAny(name, `/\`) {
			return fmt.Errorf(
				"customSkills.%s must be a single nonempty skill name",
				property,
			)
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

// propertyNames describes the known property names of one configuration object.
type propertyNames struct {
	canonical []string
	// obsolete maps removed names to the canonical path that replaces them.
	obsolete map[string]string
}

var (
	topLevelProperties = propertyNames{
		canonical: []string{"documents", "googleDrive", "customSkills"},
		obsolete: map[string]string{
			"google_drive":      "googleDrive",
			"custom-skills":     "customSkills",
			"code-review-skill": "customSkills.codeReview",
		},
	}
	nestedProperties = map[string]propertyNames{
		"googleDrive": {canonical: []string{"folders", "files"}},
		"customSkills": {
			canonical: []string{"codeReview", "securityReview", "riskClassification", "reviewChanges"},
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
// names, which JSON decoding would otherwise ignore or match case-insensitively.
func checkPropertyNames(fields map[string]json.RawMessage) error {
	if err := topLevelProperties.check("", fields); err != nil {
		return err
	}
	for _, name := range topLevelProperties.canonical {
		names, ok := nestedProperties[name]
		if !ok {
			continue
		}
		// Typed decoding reports values that are not objects.
		var nested map[string]json.RawMessage
		if json.Unmarshal(fields[name], &nested) != nil {
			continue
		}
		if err := names.check(name+".", nested); err != nil {
			return err
		}
	}
	return nil
}

func (p propertyNames) check(prefix string, fields map[string]json.RawMessage) error {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	for _, key := range keys {
		if slices.Contains(p.canonical, key) {
			continue
		}
		for old, replacement := range p.obsolete {
			if strings.EqualFold(key, old) {
				return fmt.Errorf("%s%s is unsupported; use %s", prefix, key, replacement)
			}
		}
		for _, name := range p.canonical {
			if strings.EqualFold(key, name) {
				return fmt.Errorf("%s%s is unsupported; use %s%s", prefix, key, prefix, name)
			}
		}
	}
	return nil
}
