package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilePromptContents(t *testing.T) {
	t.Chdir(t.TempDir())
	for path, content := range map[string]string{"context.md": "# Context\r\n\n", "task, one.md": "  Fix café\n", "empty.md": ""} {
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	require.NoError(t, os.Mkdir("directory", 0o700))
	absolute, err := filepath.Abs("context.md")
	require.NoError(t, err)
	for _, tc := range []struct {
		name          string
		paths         []string
		want, wantErr string
	}{
		{name: "omitted", want: "query"},
		{name: "empty list", paths: []string{}, want: "query"},
		{name: "single exact content", paths: []string{"context.md"}, want: "# Context\r\n\n"},
		{name: "ordered exact contents", paths: []string{"context.md", "task, one.md"}, want: "# Context\r\n\n\n\n  Fix café\n"},
		{name: "duplicates", paths: []string{"task, one.md", "task, one.md"}, want: "  Fix café\n\n\n  Fix café\n"},
		{name: "absolute", paths: []string{absolute}, want: "# Context\r\n\n"},
		{name: "empty file", paths: []string{"empty.md"}},
		{name: "empty files retain separators", paths: []string{"empty.md", "empty.md"}, want: "\n\n"},
		{name: "empty path", paths: []string{""}, wantErr: "--filePrompt[0] path must not be empty"},
		{name: "missing first", paths: []string{"missing.md", "context.md"}, wantErr: "missing.md"},
		{name: "missing later", paths: []string{"context.md", "missing.md"}, wantErr: "missing.md"},
		{name: "directory", paths: []string{"directory"}, wantErr: "directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := NewOptions(Options{AgentName: "codex", Branch: "test", Prompt: "query", FilePrompt: tc.paths})
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				require.Equal(t, Options{}, opts)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, opts.Prompt)
			require.Equal(t, tc.paths, opts.FilePrompt)
		})
	}
}

func TestFilePromptPermissionFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unreadable.md")
	require.NoError(t, os.WriteFile(path, []byte("instructions"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	if _, err := os.ReadFile(path); err == nil {
		t.Skip("filesystem permits reading mode-000 files")
	}
	_, err := NewOptions(Options{AgentName: "codex", Branch: "test", FilePrompt: []string{path}})
	require.ErrorIs(t, err, os.ErrPermission)
	require.ErrorContains(t, err, path)
}
