package sandbox

import (
	"strings"
	"testing"
)

func TestRuntimeSelection(t *testing.T) {
	const codex = "OpenAI Codex v0.100.0\n--------\nworkdir: /workspace\nmodel: gpt-runtime\nprovider: openai\n--------\n"
	const claude = `{"type":"system","subtype":"init","model":"claude-runtime"}` + "\n"
	for _, tc := range []struct {
		name, output, agent, model string
	}{
		{"codex", codex, "codex", "gpt-runtime"},
		{"colored banner", "\x1b[1mOpenAI Codex v0.100.0\x1b[0m\r\n--------\r\n\x1b[32mmodel: gpt-runtime (high)\x1b[0m\r\n--------\r\n", "codex", "gpt-runtime"},
		{"claude", claude, "claude", "claude-runtime"},
		{"retry", codex + codex, "codex", "gpt-runtime"},
		{"fallback", codex + claude, "claude", "claude-runtime"},
		{"reverse fallback", claude + codex, "codex", "gpt-runtime"},
		{"missing final model", codex + `{"type":"system","subtype":"init"}`, "claude", ""},
		{"malformed final model", codex + `{"type":"system","subtype":"init","model":123}`, "claude", ""},
		{"partial final banner", claude + "OpenAI Codex v0.100.0\n--------\nmodel: final-model", "codex", "final-model"},
		{"incomplete banner", claude + "OpenAI Codex v0.100.0\n", "codex", ""},
		{"generic fields are not evidence", "Agent: codex\nModel: configured\nmodel: configured\n", "", ""},
		{"agent text is not evidence", `{"type":"assistant","message":{"content":"Agent: claude","model":"invented"}}`, "", ""},
		{"pretty event", "{\n  \"type\": \"system\",\n  \"subtype\": \"init\",\n  \"model\": \"runtime\"\n}\n", "claude", "runtime"},
		{"escaped content", `{"type":"assistant","text":"\" } { model: fake"}` + claude, "claude", "claude-runtime"},
		{"oversized line", strings.Repeat("x", selectionBufferLimit+1) + "\n" + codex, "codex", "gpt-runtime"},
		{"oversized event", `{"type":"assistant","text":"` + strings.Repeat("x", selectionBufferLimit+1) + `"}` + claude, "claude", "claude-runtime"},
		{"malformed event", "{invalid}\n" + claude, "claude", "claude-runtime"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, chunkSize := range []int{1, 7, len(tc.output)} {
				var selection runtimeSelection
				stream := &selectionStream{selection: &selection}
				for start := 0; start < len(tc.output); start += chunkSize {
					chunk := tc.output[start:min(start+chunkSize, len(tc.output))]
					if n, err := stream.Write([]byte(chunk)); n != len(chunk) || err != nil {
						t.Fatalf("Write = %d, %v", n, err)
					}
				}
				stream.flushLine()
				if selection.agent != tc.agent || selection.model != tc.model {
					t.Fatalf("chunk size %d: got %q/%q, want %q/%q", chunkSize, selection.agent, selection.model, tc.agent, tc.model)
				}
			}
		})
	}
}
