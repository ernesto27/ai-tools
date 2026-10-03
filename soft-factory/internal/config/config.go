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
