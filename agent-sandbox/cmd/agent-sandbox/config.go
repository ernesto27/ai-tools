package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ernesto27/ai-tools/agent-sandbox/config"
	"github.com/ernesto27/ai-tools/agent-sandbox/internal/sandbox"
)

const localConfigFile = "agent-sandbox.json"
const configAPIKey = "apiKey"
const configReviewers = "reviewers"

var reviewerUsername = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*$`)

// configRunArgs merges local defaults after Cobra has parsed the command line
// but before it validates prompt sources. A JSON query or filePrompt can
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
	path := localConfigFile
	optional := true
	if cmd.Flags().Changed(flagConfig) {
		var err error
		path, err = cmd.Flags().GetString(flagConfig)
		if err != nil {
			return err
		}
		if path == "" {
			return fmt.Errorf("--config requires a nonempty file path")
		}
		optional = false
	}
	config, err := readConfig(path, optional)
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

	// Explicit flags win; section values override shared JSON defaults.
	if !cmd.Flags().Changed(flagBranch) {
		flags.Branch = config.Branch
		if section != nil && section.Branch != nil {
			flags.Branch = *section.Branch
		}
	}

	if !cmd.Flags().Changed(flagModel) {
		flags.Model = config.Model
		if section != nil && section.Model != nil {
			flags.Model = *section.Model
		}
	}
	if !cmd.Flags().Changed(flagEffort) {
		flags.Effort = config.Effort
		if section != nil && section.Effort != nil {
			flags.Effort = *section.Effort
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
	if !cmd.Flags().Changed(flagFile) {
		flags.Files = config.File
		if section != nil && section.File != nil {
			flags.Files = *section.File
		}
	}

	if section != nil {
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
func readLocalConfig() (config.Config, error) {
	return readConfig(localConfigFile, true)
}

// readConfig never consults the project-local file when a caller selects a path.
// Paths inside the JSON remain relative to the process working directory.
func readConfig(path string, optional bool) (config.Config, error) {
	data, err := os.ReadFile(path)
	if optional && errors.Is(err, os.ErrNotExist) {
		return config.Config{}, nil
	}
	if err != nil {
		return config.Config{}, fmt.Errorf("%s: %w", path, err)
	}

	// Decode raw fields first because encoding/json treats null as an omitted
	// pointer value. The validation pass rejects null and reports the offending
	// section and field before decoding into the typed configuration below.
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return config.Config{}, fmt.Errorf("%s: %w", path, err)
	}
	if root == nil {
		return config.Config{}, fmt.Errorf("%s: expected a JSON object", path)
	}

	for name, value := range root {
		if name == flagFile {
			if err := validateConfigField(path, "", name, value); err != nil {
				return config.Config{}, err
			}
			continue
		}
		if name == flagBranch || name == flagModel || name == flagEffort || name == flagAgent || name == flagBaseImage {
			var field string
			if string(value) == "null" || json.Unmarshal(value, &field) != nil {
				return config.Config{}, fmt.Errorf("%s: %s must be a JSON string", path, name)
			}
			continue
		}
		if name == flagPush || name == flagPR || name == flagHostNetwork {
			var field bool
			if string(value) == "null" || json.Unmarshal(value, &field) != nil {
				return config.Config{}, fmt.Errorf("%s: %s must be a JSON boolean", path, name)
			}
			continue
		}
		if name != "run" && name != "resume" {
			return config.Config{}, fmt.Errorf("%s: unknown section %q", path, name)
		}
		var section map[string]json.RawMessage
		if err := json.Unmarshal(value, &section); err != nil || section == nil {
			return config.Config{}, fmt.Errorf("%s: %s must be a JSON object", path, name)
		}
		for key, field := range section {
			if err := validateConfigField(path, name, key, field); err != nil {
				return config.Config{}, err
			}
		}
	}
	var cfg config.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return config.Config{}, fmt.Errorf("%s: %w", path, err)
	}
	if cfg.Run != nil {
		cfg.Run.Reviewers = normalizeReviewers(cfg.Run.Reviewers)
	}
	return cfg, nil
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

func validateConfigField(path, section, key string, value json.RawMessage) error {
	fieldName := key
	if section != "" {
		fieldName = section + "." + key
	}
	switch key {
	case configReviewers:
		if section != "run" {
			return fmt.Errorf("%s: unknown field %s", path, fieldName)
		}
		var reviewers []json.RawMessage
		if string(value) == "null" || json.Unmarshal(value, &reviewers) != nil {
			return fmt.Errorf("%s: %s must be an array of JSON strings", path, fieldName)
		}
		for i, value := range reviewers {
			var reviewer string
			if string(value) == "null" || json.Unmarshal(value, &reviewer) != nil {
				return fmt.Errorf("%s: %s[%d] must be a JSON string", path, fieldName, i)
			}
			if !reviewerUsername.MatchString(strings.TrimSpace(reviewer)) {
				return fmt.Errorf("%s: %s[%d] must be a plain GitHub username (letters, digits, and hyphens, starting with a letter or digit)", path, fieldName, i)
			}
		}
	case flagBranch, flagAgent, flagModel, flagEffort, flagBaseImage, flagQuery, flagCommitMessage, flagFilePrompt, configAPIKey:
		var field string
		if string(value) == "null" || json.Unmarshal(value, &field) != nil {
			return fmt.Errorf("%s: %s must be a JSON string", path, fieldName)
		}
	case flagPush, flagPR, flagHostNetwork:
		var field bool
		if string(value) == "null" || json.Unmarshal(value, &field) != nil {
			return fmt.Errorf("%s: %s must be a JSON boolean", path, fieldName)
		}
	case flagImage, flagFile:
		var images []json.RawMessage
		if string(value) == "null" || json.Unmarshal(value, &images) != nil {
			return fmt.Errorf("%s: %s must be an array of JSON strings", path, fieldName)
		}
		for i, image := range images {
			var imagePath string
			if string(image) == "null" || json.Unmarshal(image, &imagePath) != nil {
				return fmt.Errorf("%s: %s[%d] must be a JSON string", path, fieldName, i)
			}
		}
	default:
		return fmt.Errorf("%s: unknown field %s", path, fieldName)
	}
	return nil
}
