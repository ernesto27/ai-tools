// Package skills provides bundled review instructions for installed binaries.
package skills

import (
	"embed"
	"errors"
	"os"
	"path/filepath"
)

//go:embed */SKILL.md
var bundled embed.FS

// ReadFile uses a local skill when present, otherwise the bundled instructions.
func ReadFile(name string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join("skills", name))
	if !errors.Is(err, os.ErrNotExist) {
		return data, err
	}
	return bundled.ReadFile(name)
}
