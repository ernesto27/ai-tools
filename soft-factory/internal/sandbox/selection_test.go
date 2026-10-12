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
		{"missing model", `{"type":"system","subtype":"init"}`, "claude", ""},
		{"malformed model", `{"type":"system","subtype":"init","model":123}`, "claude", ""},
		{"unterminated header", "OpenAI Codex v0.100.0\n--------\nmodel: final-model", "codex", "final-model"},
		{"incomplete banner", "OpenAI Codex v0.100.0\n", "codex", ""},
		// One invocation runs one attempt, so startup markers repeated by its
		// prompt, messages, or tool output (for example go test -v of these
		// fixtures) must not replace the captured startup record.
		{"codex output repeats codex banner", codex + "exec\ngo test -v ./internal/sandbox\n" + strings.Replace(codex, "gpt-runtime", "printed-model", 1), "codex", "gpt-runtime"},
		{"codex output repeats claude init", codex + claude, "codex", "gpt-runtime"},
		{"codex output repeats model-less init", codex + `{"type":"system","subtype":"init"}`, "codex", "gpt-runtime"},
		{"codex output after header repeats model", codex + "model: printed-model\n--------\nmodel: printed-model\n", "codex", "gpt-runtime"},
		{"codex header without model", "OpenAI Codex v0.100.0\n--------\nworkdir: /workspace\n--------\nmodel: printed-model\n" + claude, "codex", ""},
		{"claude output repeats codex banner", claude + codex, "claude", "claude-runtime"},
		{"claude output repeats partial banner", claude + "OpenAI Codex v0.100.0\n--------\nmodel: printed-model", "claude", "claude-runtime"},
		{"claude output repeats claude init", claude + `{"type":"system","subtype":"init","model":"printed-model"}` + "\n", "claude", "claude-runtime"},
		{"model-less claude init is kept", `{"type":"system","subtype":"init"}` + "\n" + codex, "claude", ""},
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

// Codex writes its startup header to stderr while its later output can reach
// stdout; both streams share one record, so markers on the other stream after
// the startup record must not replace it.
func TestRuntimeSelectionAcrossStreams(t *testing.T) {
	var selection runtimeSelection
	stderr := &selectionStream{selection: &selection}
	stdout := &selectionStream{selection: &selection}
	for _, write := range []struct {
		stream *selectionStream
		data   string
	}{
		{stderr, "OpenAI Codex v0.100.0\n--------\n"},
		{stdout, `{"type":"system","subtype":"init","model":"printed-model"}` + "\n"},
		{stdout, "OpenAI Codex v0.100.0\n--------\nmodel: printed-model\n--------\n"},
		{stderr, "model: gpt-runtime\n--------\n"},
	} {
		if _, err := write.stream.Write([]byte(write.data)); err != nil {
			t.Fatal(err)
		}
	}
	stdout.flushLine()
	stderr.flushLine()
	if selection.agent != "codex" || selection.model != "gpt-runtime" {
		t.Fatalf("got %q/%q, want codex/gpt-runtime", selection.agent, selection.model)
	}
}
