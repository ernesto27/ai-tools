package main

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

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
					{
						n, err := writer.Write([]byte(tc.input[offset:end]))
						require.Equal(t, end-offset, n)
						require.NoError(t, err)
					}
				}
				require.NoError(t, writer.Close())
				want := tc.agent + " / fix-login\n" + separator + tc.want + separator
				require.Equal(t, want, auditText(out.String()))
				before := out.String()
				require.NoError(t, writer.Close())
				require.Equal(t, before, out.String())
				{
					_, err := writer.Write([]byte("more"))
					require.ErrorIs(t, err, io.ErrClosedPipe)
				}
			})
		}
	}
	for _, name := range []string{"opencode", "pi", "unknown"} {
		require.Nil(t, agentOutputFormatter(name))
	}
}

func TestAgentOutputFormatterStreamsLargeEvents(t *testing.T) {
	var out bytes.Buffer
	writer := agentOutputFormatter(agentoutput.AgentClaude)(&out, "fix-login")
	header := out.String()
	message := "你好" + strings.Repeat("x", 128*1024)
	input := `{"type":"result","result":"` + message + `"}`
	{
		_, err := writer.Write([]byte(input))
		require.NoError(t, err)
	}
	require.Equal(t, header, out.String())
	{
		_, err := writer.Write([]byte("\n"))
		require.NoError(t, err)
	}
	require.Equal(t, header+"TIME    AGENT\n            "+message+"\n\n", auditText(out.String()))
	before := out.String()
	{
		_, err := writer.Write([]byte(`{"type":`))
		require.NoError(t, err)
	}
	require.Equal(t, before, out.String())
	require.NoError(t, writer.Close())
	require.Contains(t, out.String()[len(before):], "! OUTPUT\n            Unrecognized claude output: unexpected end of JSON input\n            {\"type\":\n")
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
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, tc.want)
					{
						_, err := writer.Write([]byte("more"))
						require.ErrorIs(t, err, tc.want)
					}
				}
				require.ErrorIs(t, writer.Close(), tc.want)
			})
		}
	}
}

func TestFormattedClaudeOutputInLiveBridge(t *testing.T) {
	var log bytes.Buffer
	bridge := &liveBridge{log: &log}
	writer := agentOutputFormatter(agentoutput.AgentClaude)(bridge, "fix-login")
	{
		_, err := writer.Write([]byte("{\"type\":\"result\",\"result\":\"done\"}\n"))
		require.NoError(t, err)
	}
	text, _ := bridge.drain(false)
	want := "claude / fix-login\n---------------------------------------------\nTIME    AGENT\n            done\n\n"
	require.Equal(t, want, auditText(text))
	require.Equal(t, text, log.String())
	require.NoError(t, writer.Close())
	footer, _ := bridge.drain(false)
	require.Equal(t, "---------------------------------------------\n", footer)
	require.Equal(t, text+footer, log.String())
}
