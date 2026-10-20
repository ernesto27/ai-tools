package executionlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadRiskResult(t *testing.T) {
	for _, tc := range []struct {
		name string
		json string
		want string
	}{
		{"low", `{"classification":"LOW"}`, "LOW"},
		{"medium", `{"classification":"MEDIUM"}`, "MEDIUM"},
		{"high", `{"classification":"HIGH"}`, "HIGH"},
		{"unknown", `{"classification":"UNKNOWN"}`, "UNKNOWN"},
		{"custom", `{"classification":"Small"}`, "Small"},
		{"trim spaces", `{"classification":"  LOW  "}`, "LOW"},
		{"empty file", "", ""},
		{"malformed", `{"classification":`, ""},
		{"missing field", `{}`, ""},
		{"null field", `{"classification":null}`, ""},
		{"number", `{"classification":42}`, ""},
		{"boolean", `{"classification":true}`, ""},
		{"empty label", `{"classification":""}`, ""},
		{"blank label", `{"classification":"   "}`, ""},
		{"multiline", `{"classification":"LOW\nHIGH"}`, ""},
		{"carriage return", `{"classification":"LOW\rHIGH"}`, ""},
		{"tab", `{"classification":"LOW\tHIGH"}`, ""},
		{"control character", `{"classification":"LOW\u0000"}`, ""},
		{"line separator", `{"classification":"LOW\u2028HIGH"}`, ""},
		{"paragraph separator", `{"classification":"LOW\u2029HIGH"}`, ""},
		{"array", `[{"classification":"LOW"}]`, ""},
		{"string", `"LOW"`, ""},
		{"null object", `null`, ""},
		{"trailing object", `{"classification":"LOW"} {}`, ""},
		{"trailing prose", `{"classification":"LOW"} explanation`, ""},
		{"markdown", "```json\n{\"classification\":\"LOW\"}\n```", ""},
		{"narrative only", "The classification is LOW.", ""},
		{"unrelated field", `{"report":"Classification: LOW"}`, ""},
		{"invalid UTF-8", "{\"classification\":\"\xff\"}", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "result.json")
			require.NoError(t, os.WriteFile(path, []byte(tc.json), 0600))
			want := tc.want
			if want == "" {
				want = "Classification unavailable."
			}
			got, collected := readRiskResult(path)
			require.Equal(t, want, got)
			require.True(t, collected)

			_, err := os.Stat(path)
			require.NoError(t, err)
		})
	}
}

func TestReadRiskResultRejectsNonRegularPaths(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "valid.json")
	require.NoError(t, os.WriteFile(target, []byte(`{"classification":"LOW"}`), 0600))
	link := filepath.Join(dir, "link.json")
	require.NoError(t, os.Symlink(target, link))
	for _, path := range []string{"", filepath.Join(dir, "missing.json"), dir, link} {
		got, collected := readRiskResult(path)
		assert.Equal(t, "Classification unavailable.", got)
		assert.False(t, collected)
	}
}

func TestRiskResultPrompt(t *testing.T) {
	var disabled *Run
	prompt, filename, err := disabled.RiskResultPrompt("Assess risk.")
	require.NoError(t, err)
	require.Equal(t, "Assess risk.", prompt)
	require.Empty(t, filename)
	run := &Run{}
	seen := make(map[string]bool)
	for range 2 {
		prompt, filename, err := run.RiskResultPrompt("Assess risk.")
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(filename, ".soft-factory-risk-result-"))
		require.True(t, strings.HasSuffix(filename, ".json"))
		require.False(t, seen[filename])
		seen[filename] = true
		require.True(t, strings.HasPrefix(prompt, "Assess risk."))
		require.Contains(t, prompt, ContainerReportPath(filename))
	}
}
