package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Documents       []string           `json:"documents"`
	GoogleDrive     *GoogleDriveConfig `json:"google_drive,omitempty"`
	CodeReviewSkill *string            `json:"code-review-skill,omitempty"`
}

type GoogleDriveConfig struct {
	Folders []string `json:"folders"`
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
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return Config{}, fmt.Errorf("parse factory configuration: %w", err)
	}
	if value, ok := fields["code-review-skill"]; ok && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return Config{}, fmt.Errorf("validate factory configuration: code-review-skill must be a skill name, not null")
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate factory configuration: %w", err)
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.CodeReviewSkill != nil {
		name := *c.CodeReviewSkill
		if strings.TrimSpace(name) == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
			return fmt.Errorf("code-review-skill must be a single nonempty skill name")
		}
	}
	for i, path := range c.Documents {
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("documents[%d] must not be empty", i)
		}
	}
	if c.GoogleDrive != nil {
		if len(c.GoogleDrive.Folders) == 0 {
			return fmt.Errorf("google_drive.folders must not be empty")
		}
		for i, name := range c.GoogleDrive.Folders {
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("google_drive.folders[%d] must not be empty", i)
			}
		}
	}

	return nil
}
