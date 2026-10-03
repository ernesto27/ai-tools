package agent

import (
	"reflect"
	"testing"
)

func TestClaudeArgsWithoutImagesKeepsCurrentInvocation(t *testing.T) {
	got := claude{}.Args("sonnet", "inspect this", nil)
	want := []string{
		"--print", "--permission-mode", "bypassPermissions", "--effort", "high",
		"--model", "sonnet", "inspect this",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Args() = %q, want %q", got, want)
	}
}

func TestClaudeArgsAddsOrderedImagePathsBeforePrompt(t *testing.T) {
	got := claude{}.Args("", "compare them", []string{
		"/agent-sandbox-images/1.png",
		"/agent-sandbox-images/2.jpg",
	})
	want := []string{
		"--print", "--permission-mode", "bypassPermissions", "--effort", "high",
		"Analyze these attached images:\n" +
			"/agent-sandbox-images/1.png\n" +
			"/agent-sandbox-images/2.jpg\n\n" +
			"compare them",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Args() = %q, want %q", got, want)
	}
}
