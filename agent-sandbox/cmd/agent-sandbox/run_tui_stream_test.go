package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
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
					if n, err := bridge.Write([]byte(tc.text[offset:end])); err != nil || n != end-offset {
						t.Fatalf("Write = %d, %v", n, err)
					}
					text, events := bridge.drain(false)
					drained.WriteString(text)
					if len(events) != 0 {
						t.Fatal("agent JSON generated execution events")
					}
				}
				text, _ := bridge.drain(true)
				drained.WriteString(text)
				if drained.String() != tc.text {
					t.Fatal("TUI changed or filtered JSON text")
				}
				if transcript.String() != tc.text {
					t.Fatal("transcript changed or filtered JSON text")
				}
				if err := bridge.logError(); err != nil {
					t.Fatal(err)
				}
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
		if n, err := bridge.Write([]byte(event)); err != nil || n != len(event) {
			t.Fatalf("Write = %d, %v", n, err)
		}
	}
	text, _ := bridge.drain(true)
	if text != strings.Join(events, "") {
		t.Fatal("output lost after transcript failure")
	}
	if !errors.Is(bridge.logError(), failure) {
		t.Fatalf("log error = %v", bridge.logError())
	}
}
