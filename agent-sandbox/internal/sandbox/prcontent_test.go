package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-sandbox/internal/git"
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
				if err := os.Symlink(filepath.Join(dir, "README.md"), path); err != nil {
					t.Fatal(err)
				}
			case tc.directory:
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			default:
				if err := os.WriteFile(path, []byte(tc.data), 0o600); err != nil {
					t.Fatal(err)
				}
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
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want %q", err, tc.wantError)
				}
				if !tc.missing {
					if _, err := os.Lstat(path); err != nil {
						t.Fatalf("invalid artifact was removed: %v", err)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if content.Title != "Fix login" || content.Body != "Changes\n\nTests passed" {
				t.Fatalf("content = %#v", content)
			}
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatalf("artifact still exists: %v", err)
			}
			if err := requireCommittedWorktree(&git.Repo{Dir: dir}); err != nil {
				t.Fatal(err)
			}
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
				if err := os.WriteFile(path, []byte("user data"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.trackedMissing {
				runGit(t, dir, "add", prContentFile)
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			err := checkPRContentPath(context.Background(), &git.Repo{Dir: dir})
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, want error %t", err, tc.wantError)
			}
			if tc.existing {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != "user data" {
					t.Fatalf("existing file changed: %q, %v", data, err)
				}
			}
		})
	}
}
