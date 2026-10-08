package config

import (
	"encoding/json"
	"fmt"
	"maps"
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

// propertyNames lists the accepted property names of one configuration object.
type propertyNames struct {
	canonical []string
	// renamed maps obsolete names to their canonical replacement paths.
	renamed map[string]string
}

var (
	topLevelNames = propertyNames{
		canonical: []string{"documents", "googleDrive", "customSkills"},
		renamed: map[string]string{
			"google_drive":      "googleDrive",
			"custom-skills":     "customSkills",
			"code-review-skill": "customSkills.codeReview",
		},
	}
	nestedNames = map[string]propertyNames{
		"googleDrive": {canonical: []string{"folders", "files"}},
		"customSkills": {
			canonical: []string{"codeReview", "securityReview", "riskClassification", "reviewChanges"},
			renamed: map[string]string{
				"code-review":         "customSkills.codeReview",
				"security-review":     "customSkills.securityReview",
				"risk-classification": "customSkills.riskClassification",
				"review-changes":      "customSkills.reviewChanges",
			},
		},
	}
)

// check rejects obsolete names and case variants of known names, which the
// JSON decoder would otherwise ignore or match case-insensitively.
func (n propertyNames) check(parent string, fields map[string]json.RawMessage) error {
	for _, key := range slices.Sorted(maps.Keys(fields)) {
		if slices.Contains(n.canonical, key) {
			continue
		}
		replacement := ""
		for _, name := range n.canonical {
			if strings.EqualFold(key, name) {
				replacement = propertyPath(parent, name)
			}
		}
		for old, path := range n.renamed {
			if strings.EqualFold(key, old) {
				replacement = path
			}
		}
		if replacement != "" {
			return fmt.Errorf("%s is unsupported; use %s", propertyPath(parent, key), replacement)
		}
	}
	return nil
}

func propertyPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "." + name
}

// checkPropertyNames runs before decoding so obsolete settings are never
// silently dropped.
func checkPropertyNames(fields map[string]json.RawMessage) error {
	if err := topLevelNames.check("", fields); err != nil {
		return err
	}
	for _, key := range slices.Sorted(maps.Keys(nestedNames)) {
		var nested map[string]json.RawMessage
		// Non-object values are left for the decoder to report.
		if json.Unmarshal(fields[key], &nested) != nil {
			continue
		}
		if err := nestedNames[key].check(key, nested); err != nil {
			return err
		}
	}
	return nil
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
