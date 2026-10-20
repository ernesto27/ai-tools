package taskcontext

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"soft-factory/internal/config"
)

func TestBuildLocalDocumentContext(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{"one.md": "one {{literal}} <code> 100%", "empty.md": ""} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(text), 0600))
	}
	for _, tc := range []struct {
		paths []string
		want  string
	}{
		{nil, ""},
		{[]string{"one.md"}, "# Document: one.md\n\none {{literal}} <code> 100%\n\n"},
		{[]string{"empty.md", "one.md"}, "# Document: empty.md\n\n\n\n# Document: one.md\n\none {{literal}} <code> 100%\n\n"},
	} {
		got, err := Build(context.Background(), config.Config{Documents: tc.paths}, dir)
		require.NoError(t, err)
		require.Equal(t, tc.want, got)
	}

	got, err := Build(context.Background(), config.Config{Documents: []string{"missing.md"}}, dir)
	require.Error(t, err)
	require.Empty(t, got)
}
