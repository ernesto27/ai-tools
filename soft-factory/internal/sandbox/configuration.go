package sandbox

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	sandboxconfig "github.com/ernesto27/ai-tools/agent-sandbox/config"

	"soft-factory/internal/config"
)

// The unmodified executable always discovers this filename in its working
// directory, even with flags. Lstat rejects every entry without following or
// reading it, including dangling symlinks.
func requireNoLegacyConfig() error {
	if _, err := os.Lstat("agent-sandbox.json"); err == nil {
		return fmt.Errorf("agent-sandbox.json exists: manually migrate its settings into software-factory.json: agentSandbox, then move the old entry away before running Factory; do not create it during a Factory run")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect legacy agent-sandbox.json: %w", err)
	}
	return nil
}

// sandboxArgs resolves section overrides before expanding repeatable flags.
// Only invocation arguments constructed by Factory are accepted here; all
// configuration values go directly to exec, without shell interpretation.
func sandboxArgs(cfg config.Config, invocation []string, branch string, branchSet bool) []string {
	values := make(map[string][]string)
	shared := cfg.AgentSandbox
	stringValue := func(name, value string, section *string) {
		if cfg.SandboxFieldPresent(name) || value != "" {
			values[name] = []string{value}
		}
		if section != nil {
			values[name] = []string{*section}
		}
	}
	boolValue := func(name string, value bool, section *bool) {
		if cfg.SandboxFieldPresent(name) || value {
			values[name] = []string{strconv.FormatBool(value)}
		}
		if section != nil {
			values[name] = []string{strconv.FormatBool(*section)}
		}
	}
	section := shared.Run
	if invocation[0] == "resume" {
		section = shared.Resume
	}
	// Use the exported upstream section, including all of its pointer semantics.
	if section == nil {
		section = &sandboxconfig.Section{}
	}
	stringValue("agent", shared.Agent, section.Agent)
	stringValue("model", shared.Model, section.Model)
	stringValue("effort", shared.Effort, section.Effort)
	stringValue("baseImage", shared.BaseImage, section.BaseImage)
	stringValue("branch", shared.Branch, section.Branch)
	boolValue("push", shared.Push, section.Push)
	boolValue("pr", shared.PR, section.PR)
	boolValue("hn", shared.HostNetwork, section.HostNetwork)
	values["file"] = shared.File
	if section.File != nil {
		values["file"] = *section.File
	}
	if section.Image != nil {
		values["image"] = *section.Image
	}
	for name, value := range map[string]*string{"query": section.Query, "filePrompt": section.FilePrompt, "commitMessage": section.CommitMessage} {
		if value != nil {
			values[name] = []string{*value}
		}
	}
	branchInInvocation := false
	aliases := map[string]string{"-a": "agent", "-m": "model", "-f": "filePrompt"}
	for i := 1; i < len(invocation); i++ {
		flag := invocation[i]
		name, _, inline := strings.Cut(flag, "=")
		if alias, ok := aliases[name]; ok {
			name = alias
		} else {
			name = strings.TrimPrefix(name, "--")
		}
		if name == "branch" {
			branchInInvocation = true
		}
		delete(values, name)
		if name == "filePrompt" || name == "query" {
			// A Factory-generated prompt replaces both configured prompt sources.
			delete(values, "filePrompt")
			delete(values, "query")
		}
		if !inline {
			i++
		}
	}
	if (branchSet || branch != "") && !branchInInvocation {
		values["branch"] = []string{branch}
	}
	args := append([]string(nil), invocation...)
	for _, name := range []string{"agent", "model", "effort", "baseImage", "branch", "push", "pr", "hn", "file", "query", "filePrompt", "commitMessage", "image"} {
		for _, value := range values[name] {
			if name == "push" || name == "pr" || name == "hn" {
				args = append(args, "--"+name+"="+value)
			} else {
				args = append(args, "--"+name, value)
			}
		}
	}
	return args
}
