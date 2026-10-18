package sandbox

import (
	"strconv"
	"strings"

	sandboxconfig "github.com/ernesto27/ai-tools/agent-sandbox/config"
)

// sandboxArgs transports typed settings as flags. Command-specific values
// override shared settings; explicit workflow and stage arguments win over both.
// API keys and reviewers have no CLI flags and are deliberately omitted.
func sandboxArgs(settings sandboxconfig.Config, args []string) []string {
	section := settings.Run
	if args[0] == "resume" {
		section = settings.Resume
	}
	if section == nil {
		section = &sandboxconfig.Section{}
	}
	result := []string{args[0]}
	addString := func(name, short, value string) {
		if !hasFlag(args[1:], name, short) {
			result = append(result, "--"+name, value)
		}
	}
	addString("branch", "b", settingValue(settings.Branch, section.Branch))
	addString("agent", "a", settingValue(settings.Agent, section.Agent))
	addString("model", "m", settingValue(settings.Model, section.Model))
	addString("effort", "", settingValue(settings.Effort, section.Effort))
	addString("baseImage", "i", settingValue(settings.BaseImage, section.BaseImage))
	addString("commitMessage", "c", settingValue("", section.CommitMessage))
	// A generated stage prompt replaces both configured prompt sources.
	if !hasFlag(args[1:], "query", "q") && !hasFlag(args[1:], "filePrompt", "f") {
		addString("query", "q", settingValue("", section.Query))
		addString("filePrompt", "f", settingValue("", section.FilePrompt))
	}
	for _, field := range []struct {
		name, short string
		value       bool
	}{
		{"push", "p", settingValue(settings.Push, section.Push)},
		{"pr", "", settingValue(settings.PR, section.PR)},
		{"hn", "", settingValue(settings.HostNetwork, section.HostNetwork)},
	} {
		if !hasFlag(args[1:], field.name, field.short) {
			result = append(result, "--"+field.name+"="+strconv.FormatBool(field.value))
		}
	}
	for _, field := range []struct {
		name   string
		values []string
	}{
		{"file", settingValue(settings.File, section.File)},
		{"image", settingValue(nil, section.Image)},
	} {
		if !hasFlag(args[1:], field.name, "") {
			for _, value := range field.values {
				result = append(result, "--"+field.name, value)
			}
		}
	}
	return append(result, args[1:]...)
}

func settingValue[T any](shared T, section *T) T {
	if section != nil {
		return *section
	}
	return shared
}

// hasFlag examines Factory's own arguments, whose value flags use separate
// arguments and whose booleans use --flag=value.
func hasFlag(args []string, name, short string) bool {
	for i := 0; i < len(args); i++ {
		flag, _, equals := strings.Cut(args[i], "=")
		if flag == "--"+name || (short != "" && flag == "-"+short) {
			return true
		}
		if !equals && flag != "--push" && flag != "-p" && flag != "--pr" && flag != "--hn" {
			i++ // Skip a value, which may itself look like a flag.
		}
	}
	return false
}
