package main

import (
	"os"
	"strings"
	"testing"
)

func TestMissingSkillStopsBeforeImplementation(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	err := os.WriteFile("config.json", []byte(`{"code-review-skill":"missing"}`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = runWorkflow(workflowOptions{
		ConfigPath: "config.json",
		Implement:  true,
	})
	if err == nil || !strings.Contains(err.Error(), `code review skill "missing" not found`) {
		t.Fatalf("expected missing-skill error first, got %v", err)
	}
}
