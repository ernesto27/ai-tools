package config

import "github.com/ernesto27/ai-tools/agent-sandbox/internal/agent"

// DefaultModel returns the agent's default model. An empty string means the
// agent selects its own default.
func DefaultModel(name string) (string, error) {
	selected, err := agent.Lookup(name)
	if err != nil {
		return "", err
	}
	return selected.DefaultModel(), nil
}
