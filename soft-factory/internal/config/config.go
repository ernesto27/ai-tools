package config

import (
	"bytes"
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

	// Reject malformed and non-object configuration before checking names.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return Config{}, fmt.Errorf("parse factory configuration: %w", err)
	}
	// Check names before decoding because encoding/json ignores unknown
	// fields and matches field names case-insensitively.
	if err := checkPropertyNames(data); err != nil {
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
// names regardless of their values. Other unknown names are ignored. data
// must be valid JSON.
func checkPropertyNames(data []byte) error {
	fields, err := objectMembers(data)
	if err != nil {
		return err
	}
	if err := checkObjectNames("", fields, topLevelProperties); err != nil {
		return err
	}
	// Check every occurrence of a repeated parent because the typed decode
	// merges all of them.
	for _, field := range fields {
		names, ok := nestedProperties[field.name]
		if !ok {
			continue
		}
		// Non-object values have no members and are reported by the typed
		// decode.
		nested, err := objectMembers(field.value)
		if err != nil {
			return err
		}
		if err := checkObjectNames(field.name+".", nested, names); err != nil {
			return err
		}
	}
	return nil
}

// member is one name and value of a JSON object.
type member struct {
	name  string
	value json.RawMessage
}

// objectMembers returns every member of a valid JSON object in document
// order, including repeated names, which a map keeps only once. Non-object
// values have no members.
func objectMembers(data []byte) ([]member, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("read object: %w", err)
	}
	if tok != json.Delim('{') {
		return nil, nil
	}
	var members []member
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("read object name: %w", err)
		}
		name, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("read object name: unexpected %v", tok)
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		members = append(members, member{name, value})
	}
	return members, nil
}

func checkObjectNames(prefix string, fields []member, names propertyNames) error {
	keys := make([]string, 0, len(fields))
	for _, field := range fields {
		keys = append(keys, field.name)
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
