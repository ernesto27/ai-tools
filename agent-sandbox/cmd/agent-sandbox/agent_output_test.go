package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestJSONEventWriter(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"event", `{"type":"system","tools":["Bash","Read"],"model":"sonnet"}` + "\n", "{\n  \"type\": \"system\",\n  \"tools\": [\n    \"Bash\",\n    \"Read\"\n  ],\n  \"model\": \"sonnet\"\n}\n"},
		{"several events", "{\"type\":\"stream_event\"}\n{\"type\":\"result\"}\n", "{\n  \"type\": \"stream_event\"\n}\n{\n  \"type\": \"result\"\n}\n"},
		{"diagnostics", "[claude-code:unrecognized_model] {\"model\":\"gpt-6-luna\"}\n\n{broken\n", "[claude-code:unrecognized_model] {\"model\":\"gpt-6-luna\"}\n\n{broken\n"},
		{"final without newline", "{\"is_error\":true}", "{\n  \"is_error\": true\n}"},
		{"unfinished final", "{\"type\":", "{\"type\":"},
		{"empty", "", ""},
	} {
		for _, size := range []int{1, 4096} {
			t.Run(tc.name+"/"+map[int]string{1: "byte chunks", 4096: "large chunks"}[size], func(t *testing.T) {
				var out bytes.Buffer
				writer := agentOutputFormatter("claude")(&out)
				for offset := 0; offset < len(tc.input); offset += size {
					end := min(offset+size, len(tc.input))
					if n, err := writer.Write([]byte(tc.input[offset:end])); n != end-offset || err != nil {
						t.Fatalf("Write = %d, %v", n, err)
					}
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				if got := out.String(); got != tc.want {
					t.Fatalf("formatted output = %q, want %q", got, tc.want)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				if out.String() != tc.want {
					t.Fatal("Close repeated output")
				}
			})
		}
	}
	for _, name := range []string{"codex", "opencode", "pi"} {
		if agentOutputFormatter(name) != nil {
			t.Fatalf("unexpected formatter for %s", name)
		}
	}
}

func TestJSONEventWriterPreservesContentAndStreams(t *testing.T) {
	var out bytes.Buffer
	writer := &jsonEventWriter{out: &out}
	input := `{"type":"future_event","n":9007199254740993,"text":"你好\n\t\u001b[31m","payload":"` + strings.Repeat("x", 128*1024) + `"}`
	if _, err := writer.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatal("incomplete line emitted early")
	}
	if _, err := writer.Write([]byte("\n")); err != nil {
		t.Fatal(err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, out.Bytes()); err != nil {
		t.Fatal(err)
	}
	if compact.String() != input {
		t.Fatal("JSON fields, order, numbers, or escapes changed")
	}
	// The first event must already be visible while later output is pending.
	before := out.String()
	if _, err := writer.Write([]byte("{\"type\":")); err != nil {
		t.Fatal(err)
	}
	if out.String() != before {
		t.Fatal("incomplete next event affected completed event")
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if out.String() != before+"{\"type\":" {
		t.Fatal("partial final event lost")
	}
}

type shortJSONOutput struct{}

func (shortJSONOutput) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestJSONEventWriterOutputFailures(t *testing.T) {
	failure := errors.New("output unavailable")
	for _, tc := range []struct {
		name string
		out  io.Writer
		want error
	}{
		{"error", failedTranscript{failure}, failure},
		{"short write", shortJSONOutput{}, io.ErrShortWrite},
	} {
		for _, newline := range []bool{true, false} {
			t.Run(tc.name+"/"+map[bool]string{true: "write", false: "close"}[newline], func(t *testing.T) {
				writer := &jsonEventWriter{out: tc.out}
				input := "{\"type\":\"result\"}"
				if newline {
					input += "\n"
				}
				_, err := writer.Write([]byte(input))
				if newline {
					if !errors.Is(err, tc.want) {
						t.Fatalf("Write error = %v", err)
					}
					if _, err := writer.Write([]byte("more")); !errors.Is(err, tc.want) {
						t.Fatalf("subsequent error = %v", err)
					}
				} else if err != nil {
					t.Fatal(err)
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
	writer := agentOutputFormatter("claude")(bridge)
	input := "{\"type\":\"result\",\"result\":\"done\"}\n"
	if _, err := writer.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	text, _ := bridge.drain(false)
	want := "{\n  \"type\": \"result\",\n  \"result\": \"done\"\n}\n"
	if text != want || log.String() != want {
		t.Fatal("formatted event did not reach TUI and transcript")
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
}
