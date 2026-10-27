package executionlog

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func projectLogDirectory(workingDirectory string) (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("resolve home directory: %w", err)
	}
	return projectLogPath(home, workingDirectory)
}

func projectLogPath(home, workingDirectory string) (string, string, error) {
	if !filepath.IsAbs(home) {
		return "", "", fmt.Errorf("home directory must be absolute")
	}
	absolute, err := filepath.Abs(workingDirectory)
	if err != nil {
		return "", "", fmt.Errorf("resolve project directory: %w", err)
	}
	absolute = filepath.Clean(absolute)
	base := filepath.Base(absolute)
	if base == string(filepath.Separator) || base == "." {
		base = "project"
	}
	digest := sha256.Sum256([]byte(absolute))
	project := fmt.Sprintf("%s-%x", base, digest[:6])
	return filepath.Join(filepath.Clean(home), ".config", "software-factory", "logs"), project, nil
}

func ensureLogDirectory(root *os.Root, name, label string) error {
	if err := root.Mkdir(name, 0700); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("create %s log directory: %w", label, err)
		}
		info, err := root.Lstat(name)
		if err != nil {
			return fmt.Errorf("inspect %s log directory: %w", label, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("create %s log directory: %q is not a directory", label, name)
		}
	}
	return nil
}
