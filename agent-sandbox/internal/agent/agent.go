// Package agent describes the coding agents the sandbox can run and how each
// one has to be wired into the container.
package agent

import (
	"fmt"
	"os"
	"strings"

	"agent-sandbox/internal/docker"
)

// Agent is a coding agent installed in the sandbox image.
type Agent interface {
	// Name is the identifier used on the command line.
	Name() string
	// Binary is the executable inside the image, used as the container entrypoint.
	Binary() string
	// Package is the npm package the binary is installed from.
	Package() string
	// BuildArg is the Dockerfile ARG that pins the package version.
	BuildArg() string
	// DefaultModel is used when --model is omitted. An empty string means the
	// agent resolves the model on its own.
	DefaultModel() string
	// SupportsImages reports whether the agent can accept image files as part of
	// its initial prompt. The sandbox uses it to validate and mount attachments
	// only for agents that can consume them.
	SupportsImages() bool
	// Container returns the docker options the agent needs, or an error when its
	// configuration is missing from the host home directory. The caller fills in
	// the entrypoint, the arguments, the user and the workspace mount.
	Container(home string) (docker.RunOptions, error)
	// Args is the agent's command line inside the container. images are absolute
	// paths inside the container; agents that do not support initial image
	// attachments ignore them. An empty model means the flag is left out so the
	// agent picks its own.
	Args(model, prompt string, images []string) []string
}

// APIKeyAgent is implemented by agents that can use a key for one container
// run without relying on or changing credentials stored on the host.
type APIKeyAgent interface {
	APIKeyContainer(key string) docker.RunOptions
	APIKeyArgs(model, prompt string, images []string) []string
}

// registry keeps the agents in the order they are offered on the command line.
var registry = []Agent{codex{}, claude{}, opencode{}, pi{}}

// Lookup returns the agent registered under name.
func Lookup(name string) (Agent, error) {
	for _, a := range registry {
		if a.Name() == name {
			return a, nil
		}
	}
	return nil, fmt.Errorf("unknown agent: %s (expected %s)", name, NamesProse())
}

// Names lists the registered agents in command-line order.
func Names() []string {
	names := make([]string, 0, len(registry))
	for _, a := range registry {
		names = append(names, a.Name())
	}
	return names
}

// NamesProse is the same list written out for an error message: "a, b, c or d".
func NamesProse() string {
	names := Names()
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

// All returns every registered agent.
func All() []Agent {
	return registry
}

// requireDir fails when path is not an existing directory on the host.
func requireDir(path, agent string) error {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return hostConfigError(path, agent)
	}
	return nil
}

// requireFile fails when path is not an existing regular file on the host.
func requireFile(path, agent string) error {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return hostConfigError(path, agent)
	}
	return nil
}

// hostConfigError explains that the agent has to be authenticated on the host
// first: the container gets its credentials only from these mounts.
func hostConfigError(path, agent string) error {
	return fmt.Errorf("%s does not exist; run %s on the host first", path, agent)
}
