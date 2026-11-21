package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	sandboxconfig "github.com/ernesto27/ai-tools/agent-sandbox/config"
	"github.com/stretchr/testify/require"
)

func TestReadTaskFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile("first.txt", []byte(" first\n"), 0600))
	require.NoError(t, os.WriteFile("a,b c.md", []byte("second "), 0600))
	require.NoError(t, os.WriteFile("empty.txt", nil, 0600))
	absolute, err := filepath.Abs("a,b c.md")
	require.NoError(t, err)
	for _, tc := range []struct {
		name            string
		paths           []string
		want, wantError string
		missing         bool
	}{
		{name: "one", paths: []string{"first.txt"}, want: " first\n"},
		{name: "ordered", paths: []string{"first.txt", "a,b c.md"}, want: " first\n\n\nsecond "},
		{name: "reversed", paths: []string{"a,b c.md", "first.txt"}, want: "second \n\n first\n"},
		{name: "duplicates", paths: []string{"first.txt", "first.txt"}, want: " first\n\n\n first\n"},
		{name: "empty content", paths: []string{"empty.txt"}, want: ""},
		{name: "empty between files", paths: []string{"first.txt", "empty.txt", "a,b c.md"}, want: " first\n\n\n\n\nsecond "},
		{name: "absolute", paths: []string{absolute}, want: "second "},
		{name: "empty list", paths: []string{}, wantError: "must contain at least one path"},
		{name: "empty path", paths: []string{"first.txt", ""}, wantError: `filePrompt[1] ""`, missing: true},
		{name: "missing second file", paths: []string{"first.txt", "missing.txt"}, wantError: `filePrompt[1] "missing.txt"`, missing: true},
		{name: "directory", paths: []string{"."}, wantError: `filePrompt[0] "."`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paths := tc.paths
			cfg := sandboxconfig.Config{Run: &sandboxconfig.Section{FilePrompt: &paths}}
			got, err := readTaskFile(cfg)
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
				require.Empty(t, got)
				if tc.missing {
					require.ErrorIs(t, err, os.ErrNotExist)
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
