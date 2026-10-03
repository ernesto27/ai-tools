package agent

import (
	"path/filepath"

	"agent-sandbox/internal/docker"
)

// opencode runs opencode. It writes inside $HOME (~/.cache) and Docker creates
// the parent directories of a mount as root, so HOME is a tmpfs: a writable
// home that lives only in RAM.
type opencode struct{}

func (opencode) Name() string         { return "opencode" }
func (opencode) Binary() string       { return "opencode" }
func (opencode) Package() string      { return "opencode-ai" }
func (opencode) BuildArg() string     { return "OPENCODE_VERSION" }
func (opencode) DefaultModel() string { return "" }
func (opencode) SupportsImages() bool { return false }

func (o opencode) Container(home string) (docker.RunOptions, error) {
	config := filepath.Join(home, ".config", "opencode")
	data := filepath.Join(home, ".local", "share", "opencode")
	state := filepath.Join(home, ".local", "state", "opencode")

	for _, dir := range []string{config, data, state} {
		if err := requireDir(dir, o.Name()); err != nil {
			return docker.RunOptions{}, err
		}
	}

	return docker.RunOptions{
		Env: []string{"HOME=/opencode-home", "OPENCODE_DB=/tmp/opencode.db"},
		Tmpfs: []docker.Tmpfs{
			{Path: "/opencode-home", Options: "rw,exec,mode=1777"},
		},
		Mounts: []docker.Mount{
			{Host: config, Container: "/opencode-home/.config/opencode"},
			{Host: data, Container: "/opencode-home/.local/share/opencode"},
			{Host: state, Container: "/opencode-home/.local/state/opencode"},
		},
	}, nil
}

func (opencode) Args(model, prompt string, _ []string) []string {
	args := []string{"run", "--auto", "--variant", "high", "--print-logs"}
	if model != "" {
		args = append(args, "--model", model)
	}
	return append(args, prompt)
}
