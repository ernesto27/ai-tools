package agent

import (
	"path/filepath"
	"strings"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/docker"
)

// claude runs Claude Code. Its HOME is redirected so each credential mode sees
// only its own mounted or disposable configuration.
type claude struct{}

func (claude) Name() string     { return "claude" }
func (claude) Binary() string   { return "claude" }
func (claude) Package() string  { return "@anthropic-ai/claude-code" }
func (claude) BuildArg() string { return "CLAUDE_CODE_VERSION" }
func (claude) DefaultModel() string {
	providers, err := readProvider()
	if err != nil {
		return ""
	}
	return providers["anthropic"].DefaultModel
}
func (claude) SupportsImages() bool { return true }

func (c claude) Container(home string) (docker.RunOptions, error) {
	claudeHome := filepath.Join(home, ".claude")
	if err := requireDir(claudeHome, c.Name()); err != nil {
		return docker.RunOptions{}, err
	}

	claudeConfig := filepath.Join(home, ".claude.json")
	if err := requireFile(claudeConfig, c.Name()); err != nil {
		return docker.RunOptions{}, err
	}

	return docker.RunOptions{
		Interactive: true,
		Env:         []string{"HOME=/claude-home"},
		Mounts: []docker.Mount{
			{Host: claudeHome, Container: "/claude-home/.claude"},
			{Host: claudeConfig, Container: "/claude-home/.claude.json"},
		},
	}, nil
}

// APIKeyContainer gives this run an isolated home so it cannot read or change
// the host's subscription login. Claude reads the key directly in print mode;
// no login step or saved credential is needed.
func (claude) APIKeyContainer(key string) docker.RunOptions {
	return docker.RunOptions{
		Interactive: true,
		Env: []string{
			"HOME=/claude-home",
			"ANTHROPIC_API_KEY=" + key,
			"CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST=1",
			"CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=0",
		},
		Tmpfs: []docker.Tmpfs{{Path: "/claude-home", Options: "rw,exec,mode=1777"}},
	}
}

func (c claude) APIKeyArgs(model, prompt string, images []string) []string {
	return c.Args(model, prompt, images)
}

func (claude) Args(model, prompt string, images []string) []string {
	// Streaming events expose tool activity and partial messages while Claude
	// works, rather than leaving the terminal quiet until its final response.
	args := []string{
		"--print", "--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--permission-mode", "bypassPermissions", "--effort", "high",
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	if len(images) > 0 {
		// Claude Code treats image paths in the initial prompt as visual input.
		// The sandbox-generated paths preserve image order without revealing host
		// filenames to the agent.
		prompt = "Analyze these attached images:\n" + strings.Join(images, "\n") + "\n\n" + prompt
	}
	return append(args, prompt)
}
