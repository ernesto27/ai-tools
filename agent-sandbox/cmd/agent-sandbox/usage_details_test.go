package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentUsageDetails(t *testing.T) {
	const claudeUsage = `"usage":{"input_tokens":300,"cache_read_input_tokens":1000,"cache_creation_input_tokens":200,"output_tokens":100}`
	const codexUsage = `"input_tokens":1000,"cached_input_tokens":400,"output_tokens":100`
	const claudeCounts = `"input_tokens":1500,"cached_input_tokens":1000,"output_tokens":100,"total_tokens":1600`
	const codexCounts = `"input_tokens":1000,"cached_input_tokens":400,"output_tokens":100,"total_tokens":1100`
	for _, tc := range []struct {
		name, agent, input string
		want               []string
	}{
		{"claude cost", "claude", `{"type":"result","total_cost_usd":0.0123,` + claudeUsage + `}`, []string{`{` + claudeCounts + `,"total_cost_usd":0.0123}`}},
		{"claude zero", "claude", `{"type":"result","total_cost_usd":0,` + claudeUsage + `}`, []string{`{` + claudeCounts + `,"total_cost_usd":0}`}},
		{"claude absent", "claude", `{"type":"result",` + claudeUsage + `}`, []string{`{` + claudeCounts + `}`}},
		{"claude null", "claude", `{"type":"result","total_cost_usd":null,` + claudeUsage + `}`, []string{`{` + claudeCounts + `}`}},
		{"claude wrong fields", "claude", `{"type":"result","usage":{"input_tokens":300,"cache_read_input_tokens":1000,"cache_creation_input_tokens":200,"output_tokens":100,"total_cost_usd":12,"cache_write_input_tokens":200}}`, []string{`{` + claudeCounts + `}`}},
		{"claude cost without usage", "claude", `{"type":"result","total_cost_usd":0.0123}`, nil},
		{"claude cost null usage", "claude", `{"type":"result","total_cost_usd":0.0123,"usage":null}`, nil},
		{"claude error cost", "claude", `{"type":"result","is_error":true,"errors":["stopped"],"total_cost_usd":0.0123,` + claudeUsage + `}`, []string{`{` + claudeCounts + `,"total_cost_usd":0.0123}`}},
		{"claude successive results", "claude", `{"type":"result","total_cost_usd":0.0123,` + claudeUsage + "}\n" + `{"type":"result","total_cost_usd":0.02,` + claudeUsage + "}\n" + `{"type":"result",` + claudeUsage + `}`, []string{`{` + claudeCounts + `,"total_cost_usd":0.0123}`, `{` + claudeCounts + `,"total_cost_usd":0.02}`, `{` + claudeCounts + `}`}},
		{"codex writes", "codex", `{"type":"turn.completed","usage":{` + codexUsage + `,"cache_write_input_tokens":200}}`, []string{`{` + codexCounts + `,"cache_write_input_tokens":200}`}},
		{"codex zero", "codex", `{"type":"turn.completed","usage":{` + codexUsage + `,"cache_write_input_tokens":0}}`, []string{`{` + codexCounts + `,"cache_write_input_tokens":0}`}},
		{"codex absent", "codex", `{"type":"turn.completed","usage":{` + codexUsage + `}}`, []string{`{` + codexCounts + `}`}},
		{"codex null", "codex", `{"type":"turn.completed","usage":{` + codexUsage + `,"cache_write_input_tokens":null}}`, []string{`{` + codexCounts + `}`}},
		{"codex ignores cost", "codex", `{"type":"turn.completed","total_cost_usd":12,"usage":{` + codexUsage + `,"total_cost_usd":12,"cache_write_input_tokens":200}}`, []string{`{` + codexCounts + `,"cache_write_input_tokens":200}`}},
		{"codex successive turns", "codex", `{"type":"turn.completed","usage":{` + codexUsage + `,"cache_write_input_tokens":200}}` + "\n" + `{"type":"turn.completed","usage":{` + codexUsage + `,"cache_write_input_tokens":300}}` + "\n" + `{"type":"turn.completed","usage":{` + codexUsage + `}}`, []string{`{` + codexCounts + `,"cache_write_input_tokens":200}`, `{` + codexCounts + `,"cache_write_input_tokens":300}`, `{` + codexCounts + `}`}},
	} {
		for _, chunkSize := range []int{1, 4096} {
			for _, trailing := range []string{"", "\n"} {
				t.Run(fmt.Sprintf("%s/chunk=%d/newline=%t", tc.name, chunkSize, trailing != ""), func(t *testing.T) {
					var out bytes.Buffer
					writer := agentOutputFormatter(tc.agent)(&out, "branch")
					input := tc.input + trailing
					for offset := 0; offset < len(input); offset += chunkSize {
						_, err := writer.Write([]byte(input[offset:min(offset+chunkSize, len(input))]))
						require.NoError(t, err)
					}
					require.NoError(t, writer.Close())
					var usages []json.RawMessage
					for _, line := range strings.Split(out.String(), "\n") {
						if !strings.HasPrefix(line, `{"type":"usage"`) {
							continue
						}
						var event struct {
							Type  string          `json:"type"`
							Agent string          `json:"agent"`
							Usage json.RawMessage `json:"usage"`
						}
						require.NoError(t, json.Unmarshal([]byte(line), &event))
						require.Equal(t, tc.agent, event.Agent)
						usages = append(usages, event.Usage)
					}
					require.Len(t, usages, len(tc.want))
					for i, want := range tc.want {
						require.JSONEq(t, want, string(usages[i]))
					}
				})
			}
		}
	}
}
