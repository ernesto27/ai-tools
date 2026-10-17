package executionlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
			if err := os.WriteFile(path, []byte(tc.json), 0600); err != nil {
				t.Fatal(err)
			}
			want := tc.want
			if want == "" {
				want = "Classification unavailable."
			}
			got, collected := readRiskResult(path)
			if got != want || !collected {
				t.Fatalf("readRiskResult = (%q, %v); want (%q, true)", got, collected, want)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("result removed before summary save: %v", err)
			}
		})
	}
}

func TestReadRiskResultRejectsNonRegularPaths(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "valid.json")
	if err := os.WriteFile(target, []byte(`{"classification":"LOW"}`), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"", filepath.Join(dir, "missing.json"), dir, link} {
		got, collected := readRiskResult(path)
		if got != "Classification unavailable." || collected {
			t.Errorf("readRiskResult(%q) = (%q, %v)", path, got, collected)
		}
	}
}

func TestRiskResultPrompt(t *testing.T) {
	var disabled *Run
	prompt, filename, err := disabled.RiskResultPrompt("Assess risk.")
	if err != nil || prompt != "Assess risk." || filename != "" {
		t.Fatalf("nil logger = (%q, %q, %v)", prompt, filename, err)
	}
	run := &Run{}
	seen := make(map[string]bool)
	for range 2 {
		prompt, filename, err := run.RiskResultPrompt("Assess risk.")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(filename, ".soft-factory-risk-result-") || !strings.HasSuffix(filename, ".json") {
			t.Fatalf("unexpected filename: %q", filename)
		}
		if seen[filename] {
			t.Fatalf("attempt reused result filename: %q", filename)
		}
		seen[filename] = true
		if !strings.HasPrefix(prompt, "Assess risk.") || !strings.Contains(prompt, ContainerReportPath(filename)) {
			t.Fatalf("prompt missing original task or result path: %s", prompt)
		}
	}
}
