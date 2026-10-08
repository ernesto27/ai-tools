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
	// DisabledStages lists stages that must not execute. Load parses it
	// separately so type errors can name the offending entry.
	DisabledStages []Stage `json:"-"`
}

// Stage is a public, case-sensitive pipeline stage identifier.
type Stage string

const (
	StageImplementation     Stage = "implementation"
	StageCodeReview         Stage = "codeReview"
	StageSecurityReview     Stage = "securityReview"
	StageRiskClassification Stage = "riskClassification"
	StageReviewChanges      Stage = "reviewChanges"
)

// Stages lists every configurable stage in pipeline order.
var Stages = []Stage{
	StageImplementation,
	StageCodeReview,
	StageSecurityReview,
	StageRiskClassification,
	StageReviewChanges,
}

// StageDisabled reports whether stage is listed in disabledStages.
func (c Config) StageDisabled(stage Stage) bool {
	return slices.Contains(c.DisabledStages, stage)
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

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse factory configuration: %w", err)
	}
	var raw struct {
		DisabledStages json.RawMessage `json:"disabledStages"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("parse factory configuration: %w", err)
	}
	cfg.DisabledStages, err = parseDisabledStages(raw.DisabledStages)
	if err != nil {
		return Config{}, fmt.Errorf("validate factory configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate factory configuration: %w", err)
	}

	return cfg, nil
}

// parseDisabledStages requires an array of strings when the field is present.
// Validate checks the names.
func parseDisabledStages(data json.RawMessage) ([]Stage, error) {
	if data == nil {
		return nil, nil
	}
	var entries []json.RawMessage
	if string(data) == "null" || json.Unmarshal(data, &entries) != nil {
		return nil, fmt.Errorf("disabledStages must be an array of stage names")
	}
	stages := make([]Stage, 0, len(entries))
	for i, entry := range entries {
		var name string
		if string(entry) == "null" || json.Unmarshal(entry, &name) != nil {
			return nil, fmt.Errorf("disabledStages[%d] must be a string", i)
		}
		stages = append(stages, Stage(name))
	}
	return stages, nil
}

func (c Config) Validate() error {
	for i, stage := range c.DisabledStages {
		if !slices.Contains(Stages, stage) {
			allowed := make([]string, len(Stages))
			for j, name := range Stages {
				allowed[j] = string(name)
			}
			return fmt.Errorf("disabledStages[%d]: unknown stage %q; allowed: %s",
				i, stage, strings.Join(allowed, ", "))
		}
	}

	skillNames := []struct{ stage, name string }{
		{"codeReview", c.CustomSkills.CodeReview},
		{"securityReview", c.CustomSkills.SecurityReview},
		{"riskClassification", c.CustomSkills.RiskClassification},
		{"reviewChanges", c.CustomSkills.ReviewChanges},
	}

	for _, skill := range skillNames {
		if skill.name == "" {
			continue
		}

		if strings.TrimSpace(skill.name) == "" ||
			skill.name == "." ||
			skill.name == ".." ||
			strings.ContainsAny(skill.name, `/\`) {
			return fmt.Errorf("customSkills.%s must be a single nonempty skill name", skill.stage)
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
