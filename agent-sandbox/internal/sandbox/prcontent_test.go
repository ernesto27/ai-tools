package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/git"
)

func TestConsumePRContent(t *testing.T) {
	for _, tc := range []struct {
		name, data                                                   string
		missing, symlink, directory, tracked, staged, stagedDeletion bool
		wantError                                                    string
	}{
		{name: "valid", data: `{"title":" Fix login ","body":"Changes\n\nTests passed"}`},
		{name: "missing", missing: true, wantError: "reading generated PR artifact"},
		{name: "invalid JSON", data: `{`, wantError: "invalid generated PR JSON"},
		{name: "empty title", data: `{"title":" ","body":"Changes"}`, wantError: "nonempty strings"},
		{name: "empty body", data: `{"title":"Fix","body":" "}`, wantError: "nonempty strings"},
		{name: "multiline title", data: `{"title":"Fix\nlogin","body":"Changes"}`, wantError: "single line"},
		{name: "unicode multiline title", data: `{"title":"Fix\u2028login","body":"Changes"}`, wantError: "single line"},
		{name: "wrong field type", data: `{"title":1,"body":"Changes"}`, wantError: "invalid generated PR JSON"},
		{name: "multiple objects", data: `{"title":"Fix","body":"Changes"} {}`, wantError: "exactly one JSON object"},
		{name: "null", data: `null`, wantError: "nonempty strings"},
		{name: "symlink", symlink: true, wantError: "regular file"},
		{name: "directory", directory: true, wantError: "regular file"},
		{name: "committed artifact", tracked: true, data: `{"title":"Fix","body":"Changes"}`, wantError: "must not be staged or committed"},
		{name: "staged artifact", staged: true, data: `{"title":"Fix","body":"Changes"}`, wantError: "must not be staged or committed"},
		{name: "staged deletion", tracked: true, stagedDeletion: true, data: `{"title":"Fix","body":"Changes"}`, wantError: "must not be staged or committed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupDeleteTestRepo(t)
			path := filepath.Join(dir, prContentFile)
			switch {
			case tc.missing:
			case tc.symlink:
				require.NoError(t, os.Symlink(filepath.Join(dir, "README.md"), path))
			case tc.directory:
				require.NoError(t, os.Mkdir(path, 0o700))
			default:
				require.NoError(t, os.WriteFile(path, []byte(tc.data), 0o600))
			}
			if tc.tracked || tc.staged {
				runGit(t, dir, "add", prContentFile)
				if tc.tracked {
					runGit(t, dir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "artifact")
				}
				if tc.stagedDeletion {
					runGit(t, dir, "rm", "--cached", prContentFile)
				}
			}
			content, err := consumePRContent(context.Background(), &git.Repo{Dir: dir})
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
				if !tc.missing {
					{
						_, err := os.Lstat(path)
						require.NoError(t, err)
					}
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, "Fix login", content.Title)
			require.Equal(t, "Changes\n\nTests passed", content.Body)
			{
				_, err := os.Lstat(path)
				require.True(t, os.IsNotExist(err), "artifact still exists: %v", err)
			}
			require.NoError(t, requireCommittedWorktree(&git.Repo{Dir: dir}))
		})
	}
}

func TestCheckPRContentPath(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		existing, trackedMissing bool
		wantError                bool
	}{
		{name: "available"},
		{name: "existing file", existing: true, wantError: true},
		{name: "tracked missing file", trackedMissing: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupDeleteTestRepo(t)
			path := filepath.Join(dir, prContentFile)
			if tc.existing || tc.trackedMissing {
				require.NoError(t, os.WriteFile(path, []byte("user data"), 0o600))
			}
			if tc.trackedMissing {
				runGit(t, dir, "add", prContentFile)
				require.NoError(t, os.Remove(path))
			}
			err := checkPRContentPath(context.Background(), &git.Repo{Dir: dir})
			require.Equal(t, tc.wantError, (err != nil))
			if tc.existing {
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, "user data", string(data))
			}
		})
	}
}
