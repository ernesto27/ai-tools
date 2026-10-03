package agent

import (
	"path/filepath"

	"agent-sandbox/internal/docker"
)

// pi runs the pi coding agent.
//
// HOME is a tmpfs: it lives only in RAM and keeps everything the agent runs
// (npm, caches) out of a HOME that Docker would leave unwritable.
//
// PI_OFFLINE=1 skips the version check at startup and the package update, which
// would need git (not in the image) and would write into the mount.
type pi struct{}

func (pi) Name() string         { return "pi" }
func (pi) Binary() string       { return "pi" }
func (pi) Package() string      { return "@earendil-works/pi-coding-agent" }
func (pi) BuildArg() string     { return "PI_VERSION" }
func (pi) DefaultModel() string { return "" }
func (pi) SupportsImages() bool { return false }

func (p pi) Container(home string) (docker.RunOptions, error) {
	agentDir := filepath.Join(home, ".pi", "agent")
	if err := requireDir(agentDir, p.Name()); err != nil {
		return docker.RunOptions{}, err
	}

	return docker.RunOptions{
		Env: []string{"HOME=/pi-home", "PI_CODING_AGENT_DIR=/pi-agent", "PI_OFFLINE=1"},
		Tmpfs: []docker.Tmpfs{
			{Path: "/pi-home", Options: "rw,exec,mode=1777"},
		},
		Mounts: []docker.Mount{
			{Host: agentDir, Container: "/pi-agent"},
		},
	}, nil
}

func (pi) Args(model, prompt string, _ []string) []string {
	args := []string{"-p", "--thinking", "high"}
	if model != "" {
		args = append(args, "--model", model)
	}
	return append(args, prompt)
}
