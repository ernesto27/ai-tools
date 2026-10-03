package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"agent-sandbox/internal/sandbox"
)

const localConfigFile = "agent-sandbox.json"

// configRunArgs merges local defaults after Cobra has parsed the command line
// but before it validates prompt sources. A JSON query or file-prompt can
// therefore satisfy the requirement, while a command-line source overrides it.
func configRunArgs(section string, branch *string, flags *runFlags) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return runArgs(cmd, args)
		}
		if err := applyLocalConfig(cmd, section, branch, flags); err != nil {
			return sandbox.NewUsageError(err)
		}
		return runArgs(cmd, args)
	}
}

// applyLocalConfig changes bound flag variables directly. Calling Flag.Set
// here would mark JSON defaults as command-line flags and lose the distinction
// needed for explicit false, empty strings, and replacement image lists.
func applyLocalConfig(cmd *cobra.Command, name string, branch *string, flags *runFlags) error {
	config, err := readLocalConfig()
	if err != nil {
		return err
	}

	for key, value := range config[name] {
		if cmd.Flags().Changed(key) {
			continue
		}
		switch key {
		case "branch":
			*branch = value.(string)
		case "agent":
			flags.AgentName = value.(string)
		case "api-key":
			flags.APIKey = value.(string)
		case "model":
			flags.Model = value.(string)
		case "base-image":
			flags.BaseImage = value.(string)
		case "query":
			flags.Prompt = value.(string)
		case "push":
			flags.Push = value.(bool)
		case "pr":
			flags.PR = value.(bool)
		case "hn":
			flags.HostNetwork = value.(bool)
		case "commit-message":
			flags.CommitMessage = value.(string)
		case "file-prompt":
			flags.FilePrompt = value.(string)
		case "image":
			images := value.([]any)
			flags.Images = make([]string, len(images))
			for i, image := range images {
				flags.Images[i] = image.(string)
			}
		}
	}

	// An explicit command-line prompt source replaces the other source only
	// when it came from JSON. Two CLI sources are still rejected by runArgs.
	if cmd.Flags().Changed("query") && !cmd.Flags().Changed("file-prompt") {
		flags.FilePrompt = ""
	}
	if cmd.Flags().Changed("file-prompt") && !cmd.Flags().Changed("query") {
		flags.Prompt = ""
	}
	return nil
}

// readLocalConfig validates both supported sections even though only one is
// applied. This catches misspellings when a user first runs either command;
// worktree management verbs never call this function.
func readLocalConfig() (map[string]map[string]any, error) {
	data, err := os.ReadFile(localConfigFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", localConfigFile, err)
	}

	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("%s: %w", localConfigFile, err)
	}
	root, ok := document.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: expected a JSON object", localConfigFile)
	}

	config := make(map[string]map[string]any, len(root))
	for name, value := range root {
		if name != "run" && name != "resume" {
			return nil, fmt.Errorf("%s: unknown section %q", localConfigFile, name)
		}
		section, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s: %s must be a JSON object", localConfigFile, name)
		}
		for key, field := range section {
			if err := validateConfigField(name, key, field); err != nil {
				return nil, err
			}
		}
		config[name] = section
	}
	return config, nil
}

func validateConfigField(section, key string, value any) error {
	fieldName := section + "." + key
	switch key {
	case "branch", "agent", "model", "base-image", "query", "commit-message", "file-prompt", "api-key":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s: %s must be a JSON string", localConfigFile, fieldName)
		}
	case "push", "pr", "hn":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: %s must be a JSON boolean", localConfigFile, fieldName)
		}
	case "image":
		images, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s: %s must be an array of JSON strings", localConfigFile, fieldName)
		}
		for i, image := range images {
			if _, ok := image.(string); !ok {
				return fmt.Errorf("%s: %s[%d] must be a JSON string", localConfigFile, fieldName, i)
			}
		}
	default:
		return fmt.Errorf("%s: unknown field %s", localConfigFile, fieldName)
	}
	return nil
}
