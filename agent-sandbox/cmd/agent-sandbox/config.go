package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"agent-sandbox/internal/sandbox"
)

const localConfigFile = "agent-sandbox.json"
const configAPIKey = "api-key"
const configReviewers = "reviewers"

var reviewerUsername = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*$`)

type localConfig struct {
	Run         *configSection `json:"run"`
	Resume      *configSection `json:"resume"`
	Model       string         `json:"model"`
	Agent       string         `json:"agent"`
	BaseImage   string         `json:"base-image"`
	Push        bool           `json:"push"`
	PR          bool           `json:"pr"`
	HostNetwork bool           `json:"hn"`
}

// Pointers preserve the difference between an omitted field and an explicit
// zero value, which matters when JSON defaults are merged with CLI flags.
type configSection struct {
	Branch        *string   `json:"branch"`
	Agent         *string   `json:"agent"`
	APIKey        *string   `json:"api-key"`
	Model         *string   `json:"model"`
	BaseImage     *string   `json:"base-image"`
	Query         *string   `json:"query"`
	Push          *bool     `json:"push"`
	PR            *bool     `json:"pr"`
	HostNetwork   *bool     `json:"hn"`
	CommitMessage *string   `json:"commit-message"`
	FilePrompt    *string   `json:"file-prompt"`
	Image         *[]string `json:"image"`
	Reviewers     []string  `json:"reviewers"`
}

// configRunArgs merges local defaults after Cobra has parsed the command line
// but before it validates prompt sources. A JSON query or file-prompt can
// therefore satisfy the requirement, while a command-line source overrides it.
func configRunArgs(section string, flags *runFlags) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return runArgs(cmd, args)
		}
		if err := applyLocalConfig(cmd, section, flags); err != nil {
			return sandbox.NewUsageError(err)
		}
		return runArgs(cmd, args)
	}
}

// applyLocalConfig changes bound flag variables directly. Calling Flag.Set
// here would mark JSON defaults as command-line flags and lose the distinction
// needed for explicit false, empty strings, and replacement image lists.
func applyLocalConfig(cmd *cobra.Command, name string, flags *runFlags) error {
	config, err := readLocalConfig()
	if err != nil {
		return err
	}

	section := config.Run
	flags.Reviewers = nil
	if name == "resume" {
		section = config.Resume
	} else if section != nil {
		flags.Reviewers = section.Reviewers
	}

	// An explicit flag wins; a section model overrides the shared JSON model.
	if !cmd.Flags().Changed(flagModel) {
		flags.Model = config.Model
		if section != nil && section.Model != nil {
			flags.Model = *section.Model
		}
	}

	if !cmd.Flags().Changed(flagAgent) {
		flags.AgentName = config.Agent
		if section != nil && section.Agent != nil {
			flags.AgentName = *section.Agent
		}
	}

	if !cmd.Flags().Changed(flagBaseImage) {
		flags.BaseImage = config.BaseImage
		if section != nil && section.BaseImage != nil {
			flags.BaseImage = *section.BaseImage
		}
	}
	if !cmd.Flags().Changed(flagPush) {
		flags.Push = config.Push
		if section != nil && section.Push != nil {
			flags.Push = *section.Push
		}
	}
	if !cmd.Flags().Changed(flagPR) {
		flags.PR = config.PR
		if section != nil && section.PR != nil {
			flags.PR = *section.PR
		}
	}
	if !cmd.Flags().Changed(flagHostNetwork) {
		flags.HostNetwork = config.HostNetwork
		if section != nil && section.HostNetwork != nil {
			flags.HostNetwork = *section.HostNetwork
		}
	}

	if section != nil {
		if section.Branch != nil && !cmd.Flags().Changed(flagBranch) {
			flags.Branch = *section.Branch
		}
		if section.APIKey != nil {
			flags.APIKey = *section.APIKey
		}

		if section.Query != nil && !cmd.Flags().Changed(flagQuery) {
			flags.Prompt = *section.Query
		}
		if section.CommitMessage != nil && !cmd.Flags().Changed(flagCommitMessage) {
			flags.CommitMessage = *section.CommitMessage
		}
		if section.FilePrompt != nil && !cmd.Flags().Changed(flagFilePrompt) {
			flags.FilePrompt = *section.FilePrompt
		}
		if section.Image != nil && !cmd.Flags().Changed(flagImage) {
			flags.Images = *section.Image
		}
	}

	// An explicit command-line prompt source replaces the other source only
	// when it came from JSON. Two CLI sources are still rejected by runArgs.
	if cmd.Flags().Changed(flagQuery) && !cmd.Flags().Changed(flagFilePrompt) {
		flags.FilePrompt = ""
	}
	if cmd.Flags().Changed(flagFilePrompt) && !cmd.Flags().Changed(flagQuery) {
		flags.Prompt = ""
	}
	return nil
}

// readLocalConfig validates both supported sections even though only one is
// applied. This catches misspellings when a user first runs either command;
// worktree management verbs never call this function.
func readLocalConfig() (localConfig, error) {
	data, err := os.ReadFile(localConfigFile)
	if errors.Is(err, os.ErrNotExist) {
		return localConfig{}, nil
	}
	if err != nil {
		return localConfig{}, fmt.Errorf("%s: %w", localConfigFile, err)
	}

	// Decode raw fields first because encoding/json treats null as an omitted
	// pointer value. The validation pass rejects null and reports the offending
	// section and field before decoding into the typed configuration below.
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return localConfig{}, fmt.Errorf("%s: %w", localConfigFile, err)
	}
	if root == nil {
		return localConfig{}, fmt.Errorf("%s: expected a JSON object", localConfigFile)
	}

	for name, value := range root {
		if name == flagModel || name == flagAgent || name == flagBaseImage {
			var field string
			if string(value) == "null" || json.Unmarshal(value, &field) != nil {
				return localConfig{}, fmt.Errorf("%s: %s must be a JSON string", localConfigFile, name)
			}
			continue
		}
		if name == flagPush || name == flagPR || name == flagHostNetwork {
			var field bool
			if string(value) == "null" || json.Unmarshal(value, &field) != nil {
				return localConfig{}, fmt.Errorf("%s: %s must be a JSON boolean", localConfigFile, name)
			}
			continue
		}
		if name != "run" && name != "resume" {
			return localConfig{}, fmt.Errorf("%s: unknown section %q", localConfigFile, name)
		}
		var section map[string]json.RawMessage
		if err := json.Unmarshal(value, &section); err != nil || section == nil {
			return localConfig{}, fmt.Errorf("%s: %s must be a JSON object", localConfigFile, name)
		}
		for key, field := range section {
			if err := validateConfigField(name, key, field); err != nil {
				return localConfig{}, err
			}
		}
	}
	var config localConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return localConfig{}, fmt.Errorf("%s: %w", localConfigFile, err)
	}
	if config.Run != nil {
		config.Run.Reviewers = normalizeReviewers(config.Run.Reviewers)
	}
	return config, nil
}

func normalizeReviewers(reviewers []string) []string {
	seen := make(map[string]bool)
	var normalized []string
	for _, reviewer := range reviewers {
		reviewer = strings.TrimSpace(reviewer)
		key := strings.ToLower(reviewer)
		if !seen[key] {
			normalized = append(normalized, reviewer)
			seen[key] = true
		}
	}
	return normalized
}

func validateConfigField(section, key string, value json.RawMessage) error {
	fieldName := section + "." + key
	switch key {
	case configReviewers:
		if section != "run" {
			return fmt.Errorf("%s: unknown field %s", localConfigFile, fieldName)
		}
		var reviewers []json.RawMessage
		if string(value) == "null" || json.Unmarshal(value, &reviewers) != nil {
			return fmt.Errorf("%s: %s must be an array of JSON strings", localConfigFile, fieldName)
		}
		for i, value := range reviewers {
			var reviewer string
			if string(value) == "null" || json.Unmarshal(value, &reviewer) != nil {
				return fmt.Errorf("%s: %s[%d] must be a JSON string", localConfigFile, fieldName, i)
			}
			if !reviewerUsername.MatchString(strings.TrimSpace(reviewer)) {
				return fmt.Errorf("%s: %s[%d] must be a plain GitHub username (letters, digits, and hyphens, starting with a letter or digit)", localConfigFile, fieldName, i)
			}
		}
	case flagBranch, flagAgent, flagModel, flagBaseImage, flagQuery, flagCommitMessage, flagFilePrompt, configAPIKey:
		var field string
		if string(value) == "null" || json.Unmarshal(value, &field) != nil {
			return fmt.Errorf("%s: %s must be a JSON string", localConfigFile, fieldName)
		}
	case flagPush, flagPR, flagHostNetwork:
		var field bool
		if string(value) == "null" || json.Unmarshal(value, &field) != nil {
			return fmt.Errorf("%s: %s must be a JSON boolean", localConfigFile, fieldName)
		}
	case flagImage:
		var images []json.RawMessage
		if string(value) == "null" || json.Unmarshal(value, &images) != nil {
			return fmt.Errorf("%s: %s must be an array of JSON strings", localConfigFile, fieldName)
		}
		for i, image := range images {
			var path string
			if string(image) == "null" || json.Unmarshal(image, &path) != nil {
				return fmt.Errorf("%s: %s[%d] must be a JSON string", localConfigFile, fieldName, i)
			}
		}
	default:
		return fmt.Errorf("%s: unknown field %s", localConfigFile, fieldName)
	}
	return nil
}
