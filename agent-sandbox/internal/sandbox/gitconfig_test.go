package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.directory != "" {
				if err := os.MkdirAll(filepath.Join(commonDir, tc.directory), 0700); err != nil {
					t.Fatal(err)
				}
			}
			mounts, err := gitConfigMounts(gitDir, commonDir)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want %q", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(mounts) != len(tc.files) {
				t.Fatalf("got %d mounts, want %d", len(mounts), len(tc.files))
			}
			for i, mount := range mounts {
				path := filepath.Join(commonDir, tc.files[i])
				if mount.Host != path || mount.Container != path || !mount.ReadOnly {
					t.Errorf("mount = %#v, want read-only %s", mount, path)
				}
			}
		})
	}
}
