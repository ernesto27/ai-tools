package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestExplicitConfigInput(t *testing.T) {
	for _, section := range []string{"run", "resume"} {
		for _, tc := range []struct {
			name, data, wantErr string
			extra               []string
		}{
			{name: "section and explicit zeros", data: `{"agent":"codex","model":"shared","push":true,"pr":true,"baseImage":"shared","run":{"model":"","push":false,"pr":false,"baseImage":"","query":"task"},"resume":{"model":"","push":false,"pr":false,"baseImage":"","query":"task"}}`},
			{name: "CLI independent agent override", data: `{"agent":"codex","model":"shared","run":{"query":"task"},"resume":{"query":"task"}}`, extra: []string{"-a", "claude"}},
			{name: "CLI independent model override", data: `{"agent":"codex","model":"shared","run":{"query":"task"},"resume":{"query":"task"}}`, extra: []string{"-m", "cli-model"}},
			{name: "unknown field", data: `{"run":{"typo":true}}`, wantErr: "unknown field run.typo"},
			{name: "unknown root", data: `{"typo":true}`, wantErr: `unknown section "typo"`},
			{name: "null section", data: `{"run":null}`, wantErr: "run must be a JSON object"},
			{name: "null field", data: `{"resume":{"push":null}}`, wantErr: "resume.push must be a JSON boolean"},
			{name: "invalid image element", data: `{"run":{"image":[false]}}`, wantErr: "run.image[0] must be a JSON string"},
			{name: "null root", data: `null`, wantErr: "expected a JSON object"},
			{name: "malformed", data: `{`, wantErr: "unexpected end"},
			{name: "missing", wantErr: "no such file"},
		} {
			t.Run(section+"/"+tc.name, func(t *testing.T) {
				t.Chdir(t.TempDir())
				const legacy = `{"agent":"invalid-legacy","run":{"typo":true}}`
				if err := os.WriteFile(localConfigFile, []byte(legacy), 0600); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(t.TempDir(), "transport.json")
				if tc.data != "" {
					if err := os.WriteFile(path, []byte(tc.data), 0600); err != nil {
						t.Fatal(err)
					}
				}
				var flags runFlags
				cmd := &cobra.Command{}
				cmd.Flags().StringVarP(&flags.Branch, flagBranch, "b", "", "")
				flags.bind(cmd)
				args := append([]string{"--config", path}, tc.extra...)
				if err := cmd.Flags().Parse(args); err != nil {
					t.Fatal(err)
				}
				err := configRunArgs(section, &flags)(cmd, nil)
				if tc.wantErr != "" {
					if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), path) {
						t.Fatalf("error = %v, want source and %q", err, tc.wantErr)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				switch tc.name {
				case "section and explicit zeros":
					if flags.AgentName != "codex" || flags.Model != "" || flags.BaseImage != "" || flags.Push || flags.PR {
						t.Fatal("lost section precedence")
					}
				case "CLI independent agent override":
					if flags.AgentName != "claude" || flags.Model != "shared" {
						t.Fatal("lost independent agent override")
					}
				case "CLI independent model override":
					if flags.AgentName != "codex" || flags.Model != "cli-model" {
						t.Fatal("lost independent model override")
					}
				}
				if got, err := os.ReadFile(localConfigFile); err != nil || string(got) != legacy {
					t.Fatal("legacy file changed")
				}
			})
		}
	}
}

func TestExplicitConfigKeepsWorkingDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("prompt.txt", []byte("relative task"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"run":{"agent":"codex","filePrompt":"prompt.txt"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	var flags runFlags
	cmd := &cobra.Command{}
	cmd.Flags().StringVarP(&flags.Branch, flagBranch, "b", "", "")
	flags.bind(cmd)
	if err := cmd.Flags().Parse([]string{"--config", path}); err != nil {
		t.Fatal(err)
	}
	if err := configRunArgs("run", &flags)(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if flags.FilePrompt != "prompt.txt" {
		t.Fatalf("file prompt rebased: %q", flags.FilePrompt)
	}
}

func TestEmptyExplicitConfigDoesNotFallBack(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(localConfigFile, []byte(`{"run":{"query":"legacy"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	var flags runFlags
	cmd := &cobra.Command{}
	flags.bind(cmd)
	if err := cmd.Flags().Parse([]string{"--config="}); err != nil {
		t.Fatal(err)
	}
	if err := configRunArgs("run", &flags)(cmd, nil); err == nil || !strings.Contains(err.Error(), "--config requires") {
		t.Fatalf("error = %v", err)
	}
}
