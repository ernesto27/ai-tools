package sandbox

import (
	"strconv"
	"strings"

	sandboxconfig "github.com/ernesto27/ai-tools/agent-sandbox/config"
)

// sandboxArgs forwards configured settings unless Factory supplies an override.
func sandboxArgs(options ExecuteOptions) []string {
	shared := options.Config.AgentSandbox
	section := shared.Run
	if options.Args[0] == "resume" {
		section = shared.Resume
	}
	if section == nil {
		section = &sandboxconfig.Section{}
	}

	overridden := make(map[string]bool)
	aliases := map[string]string{"-a": "agent", "-m": "model", "-f": "filePrompt"}
	for i := 1; i < len(options.Args); i++ {
		flag, _, inline := strings.Cut(options.Args[i], "=")
		name := strings.TrimPrefix(flag, "--")
		if alias, ok := aliases[flag]; ok {
			name = alias
		}
		overridden[name] = true
		if !inline {
			i++
		}
	}
	if overridden["filePrompt"] || overridden["query"] {
		overridden["filePrompt"], overridden["query"] = true, true
	}

	args := append([]string(nil), options.Args...)
	addString := func(name, value string, override *string) {
		if overridden[name] || (override == nil && value == "") {
			return
		}
		if override != nil {
			value = *override
		}
		args = append(args, "--"+name, value)
	}
	addBool := func(name string, value bool, override *bool) {
		if overridden[name] || (override == nil && !value) {
			return
		}
		if override != nil {
			value = *override
		}
		args = append(args, "--"+name+"="+strconv.FormatBool(value))
	}
	addList := func(name string, values []string, override *[]string) {
		if overridden[name] {
			return
		}
		if override != nil {
			values = *override
		}
		for _, value := range values {
			args = append(args, "--"+name, value)
		}
	}

	branch := section.Branch
	if options.BranchSet || options.Branch != "" {
		branch = &options.Branch
	}
	addString("agent", shared.Agent, section.Agent)
	addString("model", shared.Model, section.Model)
	addString("effort", shared.Effort, section.Effort)
	addString("baseImage", shared.BaseImage, section.BaseImage)
	addString("branch", shared.Branch, branch)
	addBool("push", shared.Push, section.Push)
	addBool("pr", shared.PR, section.PR)
	addBool("hn", shared.HostNetwork, section.HostNetwork)
	addList("file", shared.File, section.File)
	addString("query", "", section.Query)
	addString("filePrompt", "", section.FilePrompt)
	addString("commitMessage", "", section.CommitMessage)
	addList("image", nil, section.Image)
	return args
}
