package main

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/agentoutput"
)

// Normalize wall-clock timestamps so expectations remain stable across runs.
func auditText(text string) string {
	return regexp.MustCompile(`(?m)^\d{2}:\d{2}:\d{2}  `).ReplaceAllString(text, "TIME  ")
}

func TestAgentOutputFormatter(t *testing.T) {
	const separator = "---------------------------------------------\n"
	for _, tc := range []struct{ name, agent, input, want string }{
		{"claude message", agentoutput.AgentClaude, `{"type":"result","result":"done"}` + "\n", "TIME    AGENT\n            done\n\n"},
		{"codex message", agentoutput.AgentCodex, `{"type":"item.completed","item":{"type":"agent_message","text":"done"}}` + "\n", "TIME    AGENT\n            done\n\n"},
		{"initialization", agentoutput.AgentClaude, `{"type":"system","subtype":"init","tools":["Bash","Read"],"model":"sonnet"}` + "\n", ""},
		{"partial events", agentoutput.AgentClaude, "{\"type\":\"stream_event\"}\n{\"type\":\"result\"}\n", ""},
		{"diagnostic", agentoutput.AgentClaude, "diagnostic\n\n", "TIME  ! OUTPUT\n            Unrecognized claude output: invalid character 'd' looking for beginning of value\n            diagnostic\n\n"},
		{"final without newline", agentoutput.AgentClaude, `{"type":"result","result":"done"}`, "TIME    AGENT\n            done\n\n"},
		{"missing type", agentoutput.AgentClaude, `{"is_error":true}`, "TIME  ! OUTPUT\n            Unrecognized claude output: missing message type\n            {\"is_error\":true}\n\n"},
		{"unfinished final", agentoutput.AgentClaude, `{"type":`, "TIME  ! OUTPUT\n            Unrecognized claude output: unexpected end of JSON input\n            {\"type\":\n\n"},
		{"empty", agentoutput.AgentClaude, "", ""},
	} {
		for _, chunk := range []struct {
			name string
			size int
		}{{"byte chunks", 1}, {"large chunks", 4096}} {
			t.Run(tc.name+"/"+chunk.name, func(t *testing.T) {
				var out bytes.Buffer
				writer := agentOutputFormatter(tc.agent)(&out, "fix-login")
				for offset := 0; offset < len(tc.input); offset += chunk.size {
					end := min(offset+chunk.size, len(tc.input))
					if n, err := writer.Write([]byte(tc.input[offset:end])); n != end-offset || err != nil {
						t.Fatalf("Write = %d, %v", n, err)
					}
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				want := tc.agent + " / fix-login\n" + separator + tc.want + separator
				if got := auditText(out.String()); got != want {
					t.Fatalf("formatted output = %q, want %q", got, want)
				}
				before := out.String()
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				if out.String() != before {
					t.Fatal("Close repeated output")
				}
				if _, err := writer.Write([]byte("more")); !errors.Is(err, io.ErrClosedPipe) {
					t.Fatalf("Write after Close = %v", err)
				}
			})
		}
	}
	for _, name := range []string{"opencode", "pi", "unknown"} {
		if agentOutputFormatter(name) != nil {
			t.Fatalf("unexpected formatter for %s", name)
		}
	}
}

func TestAgentOutputFormatterStreamsLargeEvents(t *testing.T) {
	var out bytes.Buffer
	writer := agentOutputFormatter(agentoutput.AgentClaude)(&out, "fix-login")
	header := out.String()
	message := "你好" + strings.Repeat("x", 128*1024)
	input := `{"type":"result","result":"` + message + `"}`
	if _, err := writer.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	if out.String() != header {
		t.Fatal("incomplete line emitted early")
	}
	if _, err := writer.Write([]byte("\n")); err != nil {
		t.Fatal(err)
	}
	if got := auditText(out.String()); got != header+"TIME    AGENT\n            "+message+"\n\n" {
		t.Fatal("large Unicode message was not rendered before EOF")
	}
	before := out.String()
	if _, err := writer.Write([]byte(`{"type":`)); err != nil {
		t.Fatal(err)
	}
	if out.String() != before {
		t.Fatal("incomplete next event affected completed event")
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String()[len(before):], "! OUTPUT\n            Unrecognized claude output: unexpected end of JSON input\n            {\"type\":\n") {
		t.Fatal("partial final event lost")
	}
}

type shortAuditOutput struct{}

func (shortAuditOutput) Write(p []byte) (int, error) { return len(p) - 1, nil }

// Allow the header through, then inject a failure during an event or EOF flush.
type switchAuditOutput struct{ destination io.Writer }

func (w *switchAuditOutput) Write(p []byte) (int, error) { return w.destination.Write(p) }

func TestAgentOutputFormatterOutputFailures(t *testing.T) {
	failure := errors.New("output unavailable")
	for _, tc := range []struct {
		name string
		out  io.Writer
		want error
	}{
		{"error", failedTranscript{failure}, failure},
		{"short write", shortAuditOutput{}, io.ErrShortWrite},
	} {
		for _, phase := range []string{"header", "write", "close"} {
			t.Run(tc.name+"/"+phase, func(t *testing.T) {
				out := &switchAuditOutput{destination: io.Discard}
				if phase == "header" {
					out.destination = tc.out
				}
				writer := agentOutputFormatter(agentoutput.AgentClaude)(out, "fix-login")
				out.destination = tc.out
				input := `{"type":"result","result":"done"}`
				if phase != "close" {
					input += "\n"
				}
				_, err := writer.Write([]byte(input))
				if phase == "close" {
					if err != nil {
						t.Fatal(err)
					}
				} else {
					if !errors.Is(err, tc.want) {
						t.Fatalf("Write error = %v", err)
					}
					if _, err := writer.Write([]byte("more")); !errors.Is(err, tc.want) {
						t.Fatalf("subsequent error = %v", err)
					}
				}
				if err := writer.Close(); !errors.Is(err, tc.want) {
					t.Fatalf("Close error = %v", err)
				}
			})
		}
	}
}

func TestFormattedClaudeOutputInLiveBridge(t *testing.T) {
	var log bytes.Buffer
	bridge := &liveBridge{log: &log}
	writer := agentOutputFormatter(agentoutput.AgentClaude)(bridge, "fix-login")
	if _, err := writer.Write([]byte("{\"type\":\"result\",\"result\":\"done\"}\n")); err != nil {
		t.Fatal(err)
	}
	text, _ := bridge.drain(false)
	want := "claude / fix-login\n---------------------------------------------\nTIME    AGENT\n            done\n\n"
	if auditText(text) != want || log.String() != text {
		t.Fatalf("formatted event did not reach TUI and transcript: %q / %q", text, log.String())
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	footer, _ := bridge.drain(false)
	if footer != "---------------------------------------------\n" || log.String() != text+footer {
		t.Fatal("footer did not reach TUI and transcript")
	}
}
