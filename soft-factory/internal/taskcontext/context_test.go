package taskcontext

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"soft-factory/internal/config"
)

func TestBuildLocalDocumentContext(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{"one.md": "one {{literal}} <code> 100%", "empty.md": ""} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
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
		if err != nil || got != tc.want {
			t.Fatalf("got %q, %v; want %q", got, err, tc.want)
		}
	}
	if got, err := Build(context.Background(), config.Config{Documents: []string{"missing.md"}}, dir); err == nil || got != "" {
		t.Fatalf("got %q, %v", got, err)
	}
}
