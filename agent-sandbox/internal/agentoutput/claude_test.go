package agentoutput

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaudeDecoderTextState(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
		want  [][]string
	}{
		{
			name: "suppress duplicate final result",
			lines: []string{
				`{"type":"assistant","message":{"id":"m1","content":[{"type":"text","text":"Done"}]}}`,
				`{"type":"result","result":"Done"}`,
			},
			want: [][]string{{"Done"}, nil},
		},
		{
			name: "normalize whitespace for final comparison",
			lines: []string{
				`{"type":"assistant","message":{"id":"m1","content":[{"type":"text","text":"All done"}]}}`,
				`{"type":"result","result":"All\n done"}`,
			},
			want: [][]string{{"All done"}, nil},
		},
		{
			name: "reset text when message ID changes",
			lines: []string{
				`{"type":"assistant","message":{"id":"m1","content":[{"type":"text","text":"Working"}]}}`,
				`{"type":"assistant","message":{"id":"m2","content":[{"type":"text","text":"Done"}]}}`,
				`{"type":"result","result":"Done"}`,
			},
			want: [][]string{{"Working"}, {"Done"}, nil},
		},
		{
			name: "accumulate blocks with the same message ID",
			lines: []string{
				`{"type":"assistant","message":{"id":"m1","content":[{"type":"text","text":"First"}]}}`,
				`{"type":"assistant","message":{"id":"m1","content":[{"type":"text","text":"Second"}]}}`,
				`{"type":"result","result":"First\nSecond"}`,
			},
			want: [][]string{{"First"}, {"Second"}, nil},
		},
		{
			name: "emit a different final result",
			lines: []string{
				`{"type":"assistant","message":{"id":"m1","content":"Working"}}`,
				`{"type":"result","result":"Done"}`,
			},
			want: [][]string{{"Working"}, {"Done"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decoder := NewClaudeDecoder()
			for i, line := range tc.lines {
				var want []Event
				for _, subject := range tc.want[i] {
					want = append(want, Event{Kind: KindAgent, Scope: "\x00", Subject: subject})
				}
				assertClaudeEvents(t, decoder, line, want)
			}
		})
	}
}

func TestClaudeDecoderToolScopes(t *testing.T) {
	for _, tc := range []struct {
		name              string
		sessions, parents [2]string
	}{
		{"subagents", [2]string{"session", "session"}, [2]string{"parent-a", "parent-b"}},
		{"main and subagent", [2]string{"session", "session"}, [2]string{"", "parent-a"}},
		{"sessions", [2]string{"session-a", "session-b"}, [2]string{"parent", "parent"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decoder := NewClaudeDecoder()
			commands := [2]string{"echo first", "echo second"}
			for i := range commands {
				line := claudeScopedMessage(t, "assistant", tc.sessions[i], tc.parents[i],
					`[{"type":"tool_use","id":"tool-1","name":"Bash","input":{"command":"`+commands[i]+`"}}]`)
				assertClaudeEvents(t, decoder, line, []Event{{
					ID: "tool-1", Scope: tc.sessions[i] + "\x00" + tc.parents[i], Kind: KindCommand,
					Phase: PhaseStarted, Subject: commands[i], Status: StatusRunning,
				}})
			}
			// Reverse completion order exposes correlations that use only the tool ID.
			for _, i := range []int{1, 0} {
				line := claudeScopedMessage(t, "user", tc.sessions[i], tc.parents[i],
					`[{"type":"tool_result","tool_use_id":"tool-1","content":"`+commands[i]+`"}]`)
				assertClaudeEvents(t, decoder, line, []Event{{
					ID: "tool-1", Scope: tc.sessions[i] + "\x00" + tc.parents[i], Kind: KindCommand,
					Phase: PhaseCompleted, Subject: commands[i], Status: StatusCompleted, Output: commands[i],
				}})
			}
		})
	}
}

func TestClaudeDecoderContentForms(t *testing.T) {
	for _, tc := range []struct {
		name, role, content string
		want                []Event
	}{
		{"assistant string", "assistant", `"Hello"`, []Event{{Kind: KindAgent, Scope: "\x00", Subject: "Hello"}}},
		{"assistant block", "assistant", `[{"type":"text","text":"Hello"}]`, []Event{{Kind: KindAgent, Scope: "\x00", Subject: "Hello"}}},
		{"user string", "user", `"Forwarded prompt"`, nil},
		{"user text block", "user", `[{"type":"text","text":"Forwarded prompt"}]`, nil},
		{"user string tool result", "user", `[{"type":"tool_result","tool_use_id":"tool-1","content":"PASS"}]`,
			[]Event{{ID: "tool-1", Scope: "\x00", Kind: KindCommand, Phase: PhaseCompleted, Subject: "go test ./...", Status: StatusCompleted, Output: "PASS"}}},
		{"user block tool result", "user", `[{"type":"tool_result","tool_use_id":"tool-1","content":[{"type":"text","text":"PASS"},{"type":"text","text":"ok"}]}]`,
			[]Event{{ID: "tool-1", Scope: "\x00", Kind: KindCommand, Phase: PhaseCompleted, Subject: "go test ./...", Status: StatusCompleted, Output: "PASS\nok"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decoder := NewClaudeDecoder()
			assertClaudeEvents(t, decoder,
				`{"type":"assistant","message":{"id":"m0","content":[{"type":"tool_use","id":"tool-1","name":"Bash","input":{"command":"go test ./..."}}]}}`,
				[]Event{{ID: "tool-1", Scope: "\x00", Kind: KindCommand, Phase: PhaseStarted, Subject: "go test ./...", Status: StatusRunning}})
			assertClaudeEvents(t, decoder, claudeScopedMessage(t, tc.role, "", "", tc.content), tc.want)
		})
	}
}

func TestClaudeDecoderMCPInputs(t *testing.T) {
	for _, field := range []string{"command", "file_path", "notebook_path", "path", "pattern", "query"} {
		for _, value := range []struct{ name, raw string }{
			{"object", `{"nested":"value"}`},
			{"array", `["value"]`},
			{"number", `42`},
			{"boolean", `true`},
			{"null", `null`},
		} {
			t.Run(field+"/"+value.name, func(t *testing.T) {
				line := `{"type":"assistant","message":{"id":"m1","content":[{"type":"tool_use","id":"tool-1","name":"mcp__docs__query","input":{"` + field + `":` + value.raw + `}}]}}`
				assertClaudeEvents(t, NewClaudeDecoder(), line, []Event{{
					ID: "tool-1", Scope: "\x00", Kind: KindTool, Phase: PhaseStarted,
					Subject: "mcp__docs__query", Status: StatusRunning,
				}})
			})
		}
	}
	t.Run("use later string field", func(t *testing.T) {
		assertClaudeEvents(t, NewClaudeDecoder(),
			`{"type":"assistant","message":{"id":"m1","content":[{"type":"tool_use","id":"tool-1","name":"mcp__docs__query","input":{"file_path":{},"path":[],"pattern":42,"query":"Go contexts"}}]}}`,
			[]Event{{ID: "tool-1", Scope: "\x00", Kind: KindTool, Phase: PhaseStarted, Subject: "mcp__docs__query > Go contexts", Status: StatusRunning}})
	})
}

func assertClaudeEvents(t *testing.T, decoder *ClaudeDecoder, line string, want []Event) {
	t.Helper()
	got, err := decoder.Decode([]byte(line))
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func claudeScopedMessage(t *testing.T, role, session, parent, content string) string {
	t.Helper()
	message := map[string]any{
		"type": role, "session_id": session, "parent_tool_use_id": parent,
		"message": map[string]any{"id": "m1", "content": json.RawMessage(content)},
	}
	line, err := json.Marshal(message)
	require.NoError(t, err)
	return string(line)
}
