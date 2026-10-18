package agentoutput

import (
	"encoding/json"
	"testing"
)

func TestEventKey(t *testing.T) {
	first := Event{ID: "tool-1", Scope: "parent-a"}
	if first.key() != (Event{ID: "tool-1", Scope: "parent-a"}).key() {
		t.Fatal("same ID and scope should produce equal keys")
	}
	if first.key() == (Event{ID: "tool-1", Scope: "parent-b"}).key() {
		t.Fatal("different scopes must have different keys")
	}
	if first.key() == (Event{ID: "tool-2", Scope: "parent-a"}).key() {
		t.Fatal("different IDs must have different keys")
	}
}

func TestEventFailed(t *testing.T) {
	zero, nonzero := 0, 1
	for _, tc := range []struct {
		name  string
		event Event
		want  bool
	}{
		{"empty", Event{}, false},
		{"running", Event{Status: StatusRunning}, false},
		{"completed", Event{Status: StatusCompleted}, false},
		{"failure kind", Event{Kind: KindFailure}, true},
		{"failed status", Event{Status: StatusFailed}, true},
		{"declined status", Event{Status: StatusDeclined}, true},
		{"zero exit", Event{ExitCode: &zero}, false},
		{"nonzero exit", Event{ExitCode: &nonzero}, true},
		{"completed with nonzero exit", Event{Status: StatusCompleted, ExitCode: &nonzero}, true},
		{"failed with zero exit", Event{Status: StatusFailed, ExitCode: &zero}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.event.failed(); got != tc.want {
				t.Fatalf("failed() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestJSONString(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"string", `"hello"`, "hello"},
		{"escaped text", `"你好\nworld"`, "你好\nworld"},
		{"empty string", `""`, ""},
		{"null", `null`, ""},
		{"number", `42`, ""},
		{"boolean", `true`, ""},
		{"object", `{"text":"hello"}`, ""},
		{"array", `["hello"]`, ""},
		{"invalid JSON", `"unfinished`, ""},
		{"missing", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := jsonString(json.RawMessage(tc.input)); got != tc.want {
				t.Fatalf("jsonString() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestErrorMessage(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"string", `"request failed"`, "request failed"},
		{"object", `{"message":"request failed"}`, "request failed"},
		{"object with extra fields", `{"code":503,"message":"unavailable"}`, "unavailable"},
		{"empty message", `{"message":""}`, ""},
		{"non-string message", `{"message":42}`, ""},
		{"missing message", `{"code":503}`, ""},
		{"null", `null`, ""},
		{"number", `42`, ""},
		{"invalid JSON", `{broken`, ""},
		{"missing", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := errorMessage(json.RawMessage(tc.input)); got != tc.want {
				t.Fatalf("errorMessage() = %q, want %q", got, tc.want)
			}
		})
	}
}
