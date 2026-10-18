package config

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	sandboxconfig "github.com/ernesto27/ai-tools/agent-sandbox/config"
)

func TestUnifiedSandboxSnapshot(t *testing.T) {
	t.Chdir(t.TempDir())
	const settings = `{"agent":"unknown-agent","model":"unknown-model","branch":"shared","push":true,"pr":true,"run":{"agent":"codex","model":"","branch":"","filePrompt":"tasks/task.txt","apiKey":"secret","file":[],"image":[]},"resume":{"push":false,"pr":false,"hn":false,"branch":"","model":""},"futureSetting":{"secret":"keep-for-upstream"}}`
	const factory = `{"documents":["context.md"],"customSkills":{"codeReview":"project"},"disabledStages":["securityReview"],"stages":{"implementation":{"agent":"codex"},"codeReview":{"model":"review-model"}},"agentSandbox":` + settings + `}`
	if err := os.WriteFile("software-factory.json", []byte(factory), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("software-factory.json")
	if err != nil {
		t.Fatal(err)
	}
	var want sandboxconfig.Config
	if err := json.Unmarshal([]byte(settings), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.AgentSandbox, want) {
		t.Fatal("upstream configuration changed during decoding")
	}
	if cfg.AgentSandbox.Resume.Push == nil || *cfg.AgentSandbox.Resume.Push || cfg.AgentSandbox.Run.Model == nil || *cfg.AgentSandbox.Run.Model != "" {
		t.Fatal("explicit zero values lost")
	}
	if cfg.Stages.Implementation.Agent != "codex" || cfg.Stages.CodeReview.Model != "review-model" || !cfg.StageDisabled(StageSecurityReview) || cfg.Documents[0] != "context.md" || cfg.CustomSkills.CodeReview != "project" {
		t.Fatal("Factory settings lost")
	}
	if string(cfg.SandboxJSON()) != settings {
		t.Fatal("original JSON was not preserved for downstream validation")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Factory validated sandbox semantics: %v", err)
	}
	if err := os.WriteFile("software-factory.json", []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if string(cfg.SandboxJSON()) != settings {
		t.Fatal("loaded snapshot changed after source edit")
	}
	bytes := cfg.SandboxJSON()
	bytes[0] = '['
	if string(cfg.SandboxJSON()) != settings {
		t.Fatal("caller mutated loaded JSON")
	}
}

func TestUnifiedSandboxDecodingAndLegacyIsolation(t *testing.T) {
	for _, legacy := range []string{`{`, `{"agent":"legacy","run":{"filePrompt":"legacy.txt"}}`} {
		for _, tc := range []struct{ name, data, wantErr string }{
			{"omitted", `{}`, ""},
			{"null", `{"agentSandbox":null}`, ""},
			{"invalid semantic settings", `{"agentSandbox":{"agent":"not-an-agent","model":"not-a-model"}}`, ""},
			{"unknown field left upstream", `{"agentSandbox":{"run":{"typo":true}}}`, ""},
			{"null section left upstream", `{"agentSandbox":{"run":null}}`, ""},
			{"null pointer left upstream", `{"agentSandbox":{"resume":{"push":null}}}`, ""},
			{"malformed", `{"agentSandbox":`, "parse factory configuration"},
			{"wrong type", `{"agentSandbox":{"resume":{"push":"false"}}}`, "parse factory configuration"},
			{"wrong object", `{"agentSandbox":[]}`, "parse factory configuration"},
			{"Factory validation", `{"documents":[""],"agentSandbox":{"agent":"invalid"}}`, "documents[0]"},
		} {
			t.Run(legacy+"/"+tc.name, func(t *testing.T) {
				t.Chdir(t.TempDir())
				if err := os.WriteFile("agent-sandbox.json", []byte(legacy), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile("software-factory.json", []byte(tc.data), 0600); err != nil {
					t.Fatal(err)
				}
				cfg, err := Load("software-factory.json")
				if tc.wantErr != "" {
					if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
						t.Fatalf("error = %v, want %q", err, tc.wantErr)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if tc.name == "omitted" || tc.name == "null" {
					if !reflect.DeepEqual(cfg.AgentSandbox, sandboxconfig.Config{}) || string(cfg.SandboxJSON()) != "{}" {
						t.Fatal("nonempty sandbox defaults or legacy fallback")
					}
				}
				got, err := os.ReadFile("agent-sandbox.json")
				if err != nil || string(got) != legacy {
					t.Fatal("legacy file changed")
				}
			})
		}
	}
}

func TestOnlyLegacySandboxIsNotFactoryConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("agent-sandbox.json", []byte(`{"run":{"filePrompt":"task.txt"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("software-factory.json"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing Factory file: %v", err)
	}
}
