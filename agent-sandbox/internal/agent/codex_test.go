package agent

import (
	"reflect"
	"testing"
)

func TestCodexArgsWithoutImagesKeepsCurrentInvocation(t *testing.T) {
	got := codex{}.Args("gpt-5.6-sol", "inspect this", nil)
	want := []string{
		"--dangerously-bypass-approvals-and-sandbox",
		"--model", "gpt-5.6-sol",
		"--config", `model_reasoning_effort="high"`,
		"exec", "inspect this",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Args() = %q, want %q", got, want)
	}
}

func TestCodexArgsAttachImagesBeforePrompt(t *testing.T) {
	got := codex{}.Args("", "compare them", []string{"/agent-sandbox-images/1.png", "/agent-sandbox-images/2.png"})
	want := []string{
		"--dangerously-bypass-approvals-and-sandbox",
		"--config", `model_reasoning_effort="high"`,
		"exec",
		"-i", "/agent-sandbox-images/1.png",
		"-i", "/agent-sandbox-images/2.png",
		"--", "compare them",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Args() = %q, want %q", got, want)
	}
}
