package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuditArchivesGeneratedReport(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "sandbox failure"
		}
		t.Run(name, func(t *testing.T) {
			capture := preparePromptSandbox(t)
			if fail {
				t.Setenv("SOFT_FACTORY_PROMPT_FAIL", "1")
			}
			cfg := decodeConfig(t, `{"agent":"codex","branch":"wrong","resume":{"branch":"main","file":["unrelated.txt"]}}`)
			cfg.Audit.Model = "audit-model"
			logPath := filepath.Join(t.TempDir(), "01-implementation.log")
			require.NoError(t, os.WriteFile(logPath, []byte("Original log evidence\n"), 0600))
			destination := filepath.Join(filepath.Dir(logPath), "01-implementation-audit.html")
			require.NoError(t, os.WriteFile(destination, []byte("previous"), 0600))
			err := Audit(logPath, cfg)
			want := "No code changes made.\n"
			if fail {
				require.Error(t, err)
				want = "previous"
			} else {
				require.NoError(t, err)
			}
			got, err := os.ReadFile(destination)
			require.NoError(t, err)
			require.Equal(t, want, string(got))
			got, err = os.ReadFile(logPath)
			require.NoError(t, err)
			require.Equal(t, "Original log evidence\n", string(got))
			recorded := readPromptCapture(t, capture)
			require.Contains(t, recorded.Prompt, "/agent-sandbox-files/1/01-implementation.log")
			require.Contains(t, recorded.Prompt, "/workspace/.soft-factory-audit-")
			require.Contains(t, recorded.Args, logPath)
			require.Contains(t, recorded.Args, "audit-model")
			require.Contains(t, recorded.Args, "main")
			require.NotContains(t, recorded.Args, "unrelated.txt")
			leftovers, err := filepath.Glob(".soft-factory-audit-*.html")
			require.NoError(t, err)
			require.Empty(t, leftovers)
		})
	}
}
