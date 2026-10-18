package sandbox

import (
	"encoding/json"
	"strings"
	"testing"

	sandboxconfig "github.com/ernesto27/ai-tools/agent-sandbox/config"
	"soft-factory/internal/config"
)

func TestUnifiedSelectionPrecedence(t *testing.T) {
	var settings sandboxconfig.Config
	if err := json.Unmarshal([]byte(`{"agent":"shared","model":"shared-model","run":{"agent":"run-agent","model":"run-model"},"resume":{"agent":"","model":""}}`), &settings); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, section                          string
		override                               config.StageConfig
		agent, model, agentSource, modelSource string
	}{
		{name: "run", section: "run", agent: "run-agent", model: "run-model", agentSource: "agentSandbox.run.agent", modelSource: "agentSandbox.run.model"},
		{name: "agent override", section: "run", override: config.StageConfig{Agent: "stage-agent"}, agent: "stage-agent", model: "run-model", agentSource: "stages.implementation.agent", modelSource: "agentSandbox.run.model"},
		{name: "model override", section: "run", override: config.StageConfig{Model: "stage-model"}, agent: "run-agent", model: "stage-model", agentSource: "agentSandbox.run.agent", modelSource: "stages.implementation.model"},
		{name: "explicit empty", section: "resume", agent: "", model: "", agentSource: "unavailable", modelSource: "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selection, agent, model, provider := selectionDetails(settings, tc.section, config.StageImplementation, tc.override)
			if selection.Agent != tc.agent || selection.Model != tc.model {
				t.Fatalf("selection = %+v", selection)
			}
			if !strings.Contains(agent, tc.agentSource) || !strings.Contains(model, tc.modelSource) {
				t.Fatalf("provenance = %q, %q", agent, model)
			}
			if strings.Contains(provider, "agent-sandbox.json") || strings.Contains(agent, "agent-sandbox.json") {
				t.Fatal("legacy provenance")
			}
		})
	}
}

func TestUnifiedTaskReadsSnapshot(t *testing.T) {
	preparePromptSandbox(t)
	cfg, err := config.Load("software-factory.json")
	if err != nil {
		t.Fatal(err)
	}
	// Existing legacy settings must not influence task assembly.
	writePromptTestFile(t, "agent-sandbox.json", `{"run":{"filePrompt":"does-not-exist"}}`)
	prompt, err := buildTaskPrompt(TaskContext{Config: cfg, Documents: "context"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "file task {{literal}} 100%") || !strings.Contains(prompt, "context") {
		t.Fatalf("prompt = %q", prompt)
	}
	prompt, err = buildTaskPrompt(TaskContext{TaskOverride: "Jira task", Config: config.Config{}})
	if err != nil || !strings.Contains(prompt, "Jira task") {
		t.Fatalf("override = %q, %v", prompt, err)
	}
	if _, err := buildTaskPrompt(TaskContext{Config: config.Config{}}); err == nil || !strings.Contains(err.Error(), "software-factory.json: agentSandbox.run.filePrompt") {
		t.Fatalf("missing task = %v", err)
	}
}
