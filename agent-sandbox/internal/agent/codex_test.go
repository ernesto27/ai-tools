package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexArgsWithoutImages(t *testing.T) {
	got := codex{}.Args("gpt-5.6-sol", "high", "inspect this", nil)
	want := []string{
		"--dangerously-bypass-approvals-and-sandbox",
		"--model", "gpt-5.6-sol",
		"--config", `model_reasoning_effort="high"`,
		"exec", "--json", "inspect this",
	}
	require.Equal(t, want, got)
}

func TestCodexArgsAttachImagesBeforePrompt(t *testing.T) {
	got := codex{}.Args("", "high", "compare them", []string{"/agent-sandbox-images/1.png", "/agent-sandbox-images/2.png"})
	want := []string{
		"--dangerously-bypass-approvals-and-sandbox",
		"--config", `model_reasoning_effort="high"`,
		"exec", "--json",
		"-i", "/agent-sandbox-images/1.png",
		"-i", "/agent-sandbox-images/2.png",
		"--", "compare them",
	}
	require.Equal(t, want, got)
}
