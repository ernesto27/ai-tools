package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	sandboxconfig "github.com/ernesto27/ai-tools/agent-sandbox/config"

	"soft-factory/internal/googledrive"
)

type Config struct {
	AgentSandbox sandboxconfig.Config `json:"agentSandbox"`
	Documents    []string             `json:"documents"`
	GoogleDrive  *GoogleDriveConfig   `json:"googleDrive,omitempty"`
	Stages       StagesConfig         `json:"stages"`
}

type StageConfig struct {
	Agent       string  `json:"agent,omitempty"`
	Model       string  `json:"model,omitempty"`
	Effort      *string `json:"effort,omitempty"`
	Disabled    bool    `json:"disabled,omitempty"`
	CustomSkill string  `json:"customSkill,omitempty"`
}

// SecurityReviewConfig keeps Factory's reporting option out of sandbox settings
// and out of the other stages.
type SecurityReviewConfig struct {
	StageConfig
	SaveFindings bool `json:"saveFindings,omitempty"`
}

type StagesConfig struct {
	Implementation     StageConfig          `json:"implementation"`
	CodeReview         StageConfig          `json:"codeReview"`
	SecurityReview     SecurityReviewConfig `json:"securityReview"`
	RiskClassification StageConfig          `json:"riskClassification"`
	ReviewChanges      StageConfig          `json:"reviewChanges"`
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

// StageDisabled reports whether the stage's configuration disables execution.
func (c Config) StageDisabled(stage Stage) bool {
	switch stage {
	case StageImplementation:
		return c.Stages.Implementation.Disabled
	case StageCodeReview:
		return c.Stages.CodeReview.Disabled
	case StageSecurityReview:
		return c.Stages.SecurityReview.Disabled
	case StageRiskClassification:
		return c.Stages.RiskClassification.Disabled
	case StageReviewChanges:
		return c.Stages.ReviewChanges.Disabled
	default:
		return false
	}
}

type GoogleDriveConfig struct {
	Folders []string `json:"folders"`
	Files   []string `json:"files"`
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
		AgentSandbox json.RawMessage `json:"agentSandbox"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("parse factory configuration: %w", err)
	}
	if raw.AgentSandbox == nil {
		return Config{}, fmt.Errorf("validate factory configuration: agentSandbox is required")
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate factory configuration: %w", err)
	}

	return cfg, nil
}

func (c Config) Validate() error {
	skillNames := []struct{ stage, name string }{
		{"implementation", c.Stages.Implementation.CustomSkill},
		{"codeReview", c.Stages.CodeReview.CustomSkill},
		{"securityReview", c.Stages.SecurityReview.CustomSkill},
		{"riskClassification", c.Stages.RiskClassification.CustomSkill},
		{"reviewChanges", c.Stages.ReviewChanges.CustomSkill},
	}

	for _, skill := range skillNames {
		if skill.name == "" {
			continue
		}

		if strings.TrimSpace(skill.name) == "" ||
			skill.name == "." ||
			skill.name == ".." ||
			strings.ContainsAny(skill.name, `/\`) {
			return fmt.Errorf("stages.%s.customSkill must be a single nonempty skill name", skill.stage)
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
