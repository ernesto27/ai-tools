package sandbox

import (
	"errors"
	"fmt"
	"os"

	"soft-factory/internal/config"
)

// writeSandboxConfig transports the original embedded object for one invocation.
// CreateTemp uses mode 0600. The caller removes it after the child exits, including
// failure and cancellation. No cwd change or legacy file access is needed.
func writeSandboxConfig(cfg config.Config) (string, error) {
	file, err := os.CreateTemp("", "factory-agent-sandbox-*.json")
	if err != nil {
		return "", fmt.Errorf("create software-factory.json: agentSandbox transport: %w", err)
	}
	_, writeErr := file.Write(cfg.SandboxJSON())
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		os.Remove(file.Name())
		return "", fmt.Errorf("write software-factory.json: agentSandbox transport: %w", err)
	}
	return file.Name(), nil
}
