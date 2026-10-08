package agent

import (
	"reflect"
	"testing"
)

func TestClaudeArgsStreamJSON(t *testing.T) {
	for _, tc := range []struct {
		name, model, prompt string
		images              []string
		wantTail            []string
	}{
		{name: "model", model: "sonnet", prompt: "inspect this", wantTail: []string{"--model", "sonnet", "inspect this"}},
		{name: "default model", prompt: "inspect this", wantTail: []string{"inspect this"}},
		{name: "images", prompt: "compare them", images: []string{"/agent-sandbox-images/1.png", "/agent-sandbox-images/2.jpg"}, wantTail: []string{"Analyze these attached images:\n/agent-sandbox-images/1.png\n/agent-sandbox-images/2.jpg\n\ncompare them"}},
		{name: "model and images", model: "sonnet", prompt: "inspect", images: []string{"/agent-sandbox-images/1.png"}, wantTail: []string{"--model", "sonnet", "Analyze these attached images:\n/agent-sandbox-images/1.png\n\ninspect"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []struct {
				name string
				args func(string, string, []string) []string
			}{{"host login", claude{}.Args}, {"API key", claude{}.APIKeyArgs}} {
				t.Run(mode.name, func(t *testing.T) {
					want := append([]string{"--print", "--output-format", "stream-json", "--verbose", "--include-partial-messages", "--permission-mode", "bypassPermissions", "--effort", "high"}, tc.wantTail...)
					if got := mode.args(tc.model, tc.prompt, tc.images); !reflect.DeepEqual(got, want) {
						t.Fatalf("Args() = %q, want %q", got, want)
					}
				})
			}
		})
	}
}
