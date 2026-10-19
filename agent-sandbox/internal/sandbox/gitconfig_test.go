package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitConfigMounts(t *testing.T) {
	for _, tc := range []struct {
		name      string
		linked    bool
		files     []string
		directory string
		wantError string
	}{
		{name: "shared config", linked: true, files: []string{"config"}},
		{name: "worktree configs", linked: true, files: []string{"config", "config.worktree", "worktrees/feature/config.worktree"}},
		{name: "main worktree avoids duplicate mounts", files: []string{"config", "config.worktree"}},
		{name: "missing shared config fails", linked: true, wantError: "checking Git config"},
		{name: "shared config directory fails", linked: true, directory: "config", wantError: "not a regular file"},
		{name: "optional config directory fails", linked: true, files: []string{"config"}, directory: "config.worktree", wantError: "not a regular file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			commonDir := t.TempDir()
			gitDir := commonDir
			if tc.linked {
				gitDir = filepath.Join(commonDir, "worktrees", "feature")
			}
			for _, file := range tc.files {
				path := filepath.Join(commonDir, file)
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
				require.NoError(t, os.WriteFile(path, nil, 0600))
			}
			if tc.directory != "" {
				require.NoError(t, os.MkdirAll(filepath.Join(commonDir, tc.directory), 0700))
			}
			mounts, err := gitConfigMounts(gitDir, commonDir)
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, len(tc.files), len(mounts))
			for i, mount := range mounts {
				path := filepath.Join(commonDir, tc.files[i])
				assert.Equal(t, path, mount.Host)
				assert.Equal(t, path, mount.Container)
				assert.True(t, mount.ReadOnly, "mount: %#v", mount)
			}
		})
	}
}
