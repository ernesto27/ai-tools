package agent

import (
	"path/filepath"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/docker"
)

// codex runs OpenAI Codex. It needs a TTY, so the container is started with -it.
type codex struct{}

func (codex) Name() string     { return "codex" }
func (codex) Binary() string   { return "codex" }
func (codex) Package() string  { return "@openai/codex" }
func (codex) BuildArg() string { return "CODEX_VERSION" }

func (codex) DefaultModel() string {
	providers, err := readProvider()
	if err != nil {
		return ""
	}
	return providers["openai"].DefaultModel
}

func (codex) SupportsImages() bool { return true }

func (c codex) Container(home string) (docker.RunOptions, error) {
	codexHome := filepath.Join(home, ".codex")
	if err := requireDir(codexHome, c.Name()); err != nil {
		return docker.RunOptions{}, err
	}

	return docker.RunOptions{
		TTY: true,
		Env: []string{"CODEX_HOME=/codex-home"},
		Mounts: []docker.Mount{
			{Host: codexHome, Container: "/codex-home"},
		},
	}, nil
}

// APIKeyContainer keeps Codex's home in the disposable container. Mounting
// the host's CODEX_HOME here would let this run see or change its subscription
// login, and writing a login cache would persist the API key after the run.
func (codex) APIKeyContainer(key string) docker.RunOptions {
	return docker.RunOptions{
		TTY: true,
		Env: []string{
			"HOME=/codex-home",
			"CODEX_HOME=/codex-home",
			"OPENAI_API_KEY=" + key,
		},
		Tmpfs: []docker.Tmpfs{{Path: "/codex-home", Options: "rw,exec,mode=1777"}},
	}
}

// APIKeyArgs selects a provider that reads the key for this process from its
// environment. The provider settings contain no secret and require no login.
func (c codex) APIKeyArgs(model, prompt string, images []string) []string {
	return codexArgs(model, prompt, images, true)
}

func (codex) Args(model, prompt string, images []string) []string {
	return codexArgs(model, prompt, images, false)
}

func codexArgs(model, prompt string, images []string, apiKey bool) []string {
	args := []string{"--dangerously-bypass-approvals-and-sandbox"}
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, "--config", `model_reasoning_effort="high"`)
	if apiKey {
		args = append(args,
			"--config", `model_provider="agent_sandbox_api"`,
			"--config", `model_providers.agent_sandbox_api={name="OpenAI API",base_url="https://api.openai.com/v1",env_key="OPENAI_API_KEY",wire_api="responses"}`,
		)
	}
	args = append(args, "exec")
	for _, image := range images {
		args = append(args, "-i", image)
	}
	// Codex accepts one or more values after -i. The option terminator keeps the
	// text prompt from being consumed as another attachment.
	if len(images) > 0 {
		args = append(args, "--")
	}
	return append(args, prompt)
}
