package sandbox

import (
	"encoding/json"
	"regexp"
	"strings"
	"sync"
)

// runtimeSelection is shared by the two output streams of one agent-sandbox
// invocation. agent-sandbox starts exactly one agent attempt per invocation, so
// that attempt's startup record is authoritative; a later attempt is a new
// execute call with a fresh selection, which SetStageSelection then replaces.
// Configured values and agent-authored change reports never replace it.
type runtimeSelection struct {
	mu           sync.Mutex
	agent, model string
	started      bool
}

// start records the attempt's startup agent and model. Once a startup record
// has been captured it reports false and changes nothing: the attempt's later
// prompt text, messages, and tool output can repeat startup markers verbatim.
func (s *runtimeSelection) start(agent, model string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return false
	}
	s.agent, s.model, s.started = agent, model, true
	return true
}

// setModel completes a started Codex record from its startup header.
func (s *runtimeSelection) setModel(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.model = model
}

// selectionStream observes existing agent startup output without altering it.
// Codex's startup banner and header, or Claude's system/init event, establish
// the attempt's selection. Markers after that record are ordinary output and
// are ignored. Other agents currently expose no supported selection metadata
// through the sandbox CLI, so their selections remain unknown. Never infer them
// from flags.
type selectionStream struct {
	selection *runtimeSelection
	line      []byte
	longLine  bool
	json      []byte
	depth     int
	inString  bool
	escaped   bool
	largeJSON bool
	// codex follows the header of the Codex banner this stream captured: 1
	// after the banner, 2 inside the header block, 0 otherwise. Only the first
	// model line inside that header may complete the record.
	codex int
}

const selectionBufferLimit = 64 * 1024

var terminalEscape = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

func (s *selectionStream) Write(p []byte) (int, error) {
	for _, b := range p {
		if s.depth == 0 && !s.longLine && b == '{' && strings.TrimSpace(string(s.line)) == "" {
			s.line = nil
			s.json = nil
			s.largeJSON = false
			s.inString, s.escaped = false, false
			s.depth = 1
			s.json = append(s.json, b)
			continue
		}
		if s.depth > 0 {
			if len(s.json) < selectionBufferLimit && !s.largeJSON {
				s.json = append(s.json, b)
			} else {
				s.largeJSON = true
				s.json = nil
			}
			if s.inString {
				if s.escaped {
					s.escaped = false
				} else if b == '\\' {
					s.escaped = true
				} else if b == '"' {
					s.inString = false
				}
			} else {
				switch b {
				case '"':
					s.inString = true
				case '{', '[':
					s.depth++
				case '}', ']':
					s.depth--
				}
			}
			if s.depth == 0 {
				s.observeJSON()
				s.json = nil
			}
			continue
		}
		if b == '\n' {
			s.flushLine()
		} else if len(s.line) < selectionBufferLimit && !s.longLine {
			s.line = append(s.line, b)
		} else {
			s.line = nil
			s.longLine = true
		}
	}
	return len(p), nil
}

func (s *selectionStream) observeJSON() {
	var event struct {
		Type    string          `json:"type"`
		Subtype string          `json:"subtype"`
		Model   json.RawMessage `json:"model"`
	}
	if s.largeJSON || json.Unmarshal(s.json, &event) != nil || event.Type != "system" || event.Subtype != "init" {
		return
	}
	var model string
	// An absent or malformed model still establishes the agent; the model then
	// stays unknown rather than being taken from later output.
	_ = json.Unmarshal(event.Model, &model)
	s.selection.start("claude", model)
}

// flushLine also collects an unterminated final banner line after cancellation.
func (s *selectionStream) flushLine() {
	if !s.longLine {
		line := strings.TrimSpace(terminalEscape.ReplaceAllString(string(s.line), ""))
		switch {
		case strings.HasPrefix(line, "OpenAI Codex v"):
			if s.selection.start("codex", "") {
				s.codex = 1
			}
		case line == "--------" && s.codex == 1:
			s.codex = 2
		case line == "--------" && s.codex == 2:
			s.codex = 0
		case s.codex == 2 && strings.HasPrefix(line, "model:"):
			model := strings.TrimSpace(strings.TrimPrefix(line, "model:"))
			if fields := strings.Fields(model); len(fields) > 0 {
				model = fields[0]
			}
			s.selection.setModel(model)
			// The header's first model line completes the record; any later
			// model line is ordinary output.
			s.codex = 0
		}
	}
	s.line, s.longLine = nil, false
}
