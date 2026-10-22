// Package agentoutput decodes agent stdout into a shared terminal audit feed.
package agentoutput

import (
	"encoding/json"

	"github.com/ernesto27/ai-tools/agent-sandbox/agentoutput"
)

// Decoder translates one complete JSONL record into presentation-independent
// events. Each execution owns a decoder because tool correlation is stateful.
type Decoder interface {
	Name() string
	Decode(line []byte) ([]Event, error)
}

type Event struct {
	ID       string
	Scope    string
	Kind     Kind
	Phase    Phase
	Subject  string
	Status   Status
	ExitCode *int
	Output   string
	Detail   string
	Usage    *agentoutput.TokenUsage
}

// Scope keeps a subagent's tool IDs separate from the main conversation.
type eventKey struct {
	scope string
	id    string
}

func (e Event) key() eventKey { return eventKey{scope: e.Scope, id: e.ID} }

func (e Event) failed() bool {
	return e.Kind == KindFailure || e.Status == StatusFailed ||
		e.Status == StatusDeclined || (e.ExitCode != nil && *e.ExitCode != 0)
}

func jsonString(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return ""
	}
	return text
}

func errorMessage(raw json.RawMessage) string {
	if text := jsonString(raw); text != "" {
		return text
	}
	var detail struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &detail) == nil {
		return detail.Message
	}
	return ""
}
