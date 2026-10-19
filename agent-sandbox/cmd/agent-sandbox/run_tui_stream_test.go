package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLiveBridgePreservesStreamJSON(t *testing.T) {
	for _, tc := range []struct{ name, text string }{
		{"partial and final", "{\"type\":\"stream_event\",\"text\":\"hello\"}\n{\"type\":\"assistant\",\"text\":\"hello\"}\n{\"type\":\"result\",\"result\":\"hello\"}\n"},
		{"tools and unknown", "{\"type\":\"assistant\",\"content\":[{\"type\":\"tool_use\",\"name\":\"Bash\",\"input\":{\"command\":\"go test ./...\"}}]}\n{\"type\":\"user\",\"content\":[{\"type\":\"tool_result\",\"content\":\"PASS\"}]}\n{\"type\":\"future_event\"}\n"},
		{"Unicode and escapes", "{\"text\":\"¡你好!\\n\\t\\u001b[31m\"}\n\n"},
		{"large and no final newline", "{\"text\":\"" + strings.Repeat("x", 128*1024) + "\"}"},
		{"empty", ""},
	} {
		for _, size := range []int{1, 4096} {
			t.Run(tc.name+"/"+map[int]string{1: "byte chunks", 4096: "large chunks"}[size], func(t *testing.T) {
				var transcript, drained bytes.Buffer
				bridge := &liveBridge{log: &transcript}
				for offset := 0; offset < len(tc.text); offset += size {
					end := min(offset+size, len(tc.text))
					{
						n, err := bridge.Write([]byte(tc.text[offset:end]))
						require.NoError(t, err)
						require.Equal(t, end-offset, n)
					}
					text, events := bridge.drain(false)
					drained.WriteString(text)
					require.Empty(t, events)
				}
				text, _ := bridge.drain(true)
				drained.WriteString(text)
				require.Equal(t, tc.text, drained.String())
				require.Equal(t, tc.text, transcript.String())
				require.NoError(t, bridge.logError())
			})
		}
	}
}

type failedTranscript struct{ err error }

func (w failedTranscript) Write([]byte) (int, error) { return 0, w.err }

func TestLiveBridgeContinuesAfterTranscriptFailure(t *testing.T) {
	failure := errors.New("transcript unavailable")
	bridge := &liveBridge{log: failedTranscript{failure}}
	events := []string{"{\"type\":\"stream_event\"}\n", "{\"type\":\"result\"}\n"}
	for _, event := range events {
		{
			n, err := bridge.Write([]byte(event))
			require.NoError(t, err)
			require.Equal(t, len(event), n)
		}
	}
	text, _ := bridge.drain(true)
	require.Equal(t, strings.Join(events, ""), text)
	require.ErrorIs(t, bridge.logError(), failure)
}
