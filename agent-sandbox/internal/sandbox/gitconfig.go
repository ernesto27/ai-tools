package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"agent-sandbox/internal/docker"
)

func gitConfigMounts(gitDir, commonDir string) ([]docker.Mount, error) {
	paths := []string{
		filepath.Join(commonDir, "config"),
		filepath.Join(commonDir, "config.worktree"),
	}
	if gitDir != commonDir {
		paths = append(paths, filepath.Join(gitDir, "config.worktree"))
	}

	var mounts []docker.Mount
	for i, path := range paths {
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) && i > 0 {
			// Worktree-specific configuration is optional.
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("checking Git config %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("Git config is not a regular file: %s", path)
		}

		// Commits need writable metadata, but repository configuration
		// must remain protected from container commands.
		mounts = append(mounts, docker.Mount{
			Host:      path,
			Container: path,
			ReadOnly:  true,
		})
	}
	return mounts, nil
}
