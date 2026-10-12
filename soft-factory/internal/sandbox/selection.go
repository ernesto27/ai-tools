package sandbox

import (
	"encoding/json"
	"regexp"
	"strings"
	"sync"
)

// runtimeSelection is shared by the two output streams. Only startup metadata
// replaces it; configured values and agent-authored change reports never do.
type runtimeSelection struct {
	mu           sync.Mutex
	agent, model string
}

func (s *runtimeSelection) set(agent, model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agent, s.model = agent, model
}

// selectionStream observes existing agent startup output without altering it.
// Codex's startup banner and Claude's system/init event establish a started
// attempt. Each new startup replaces both fields, even when its model is absent.
// Other agents currently expose no supported selection metadata through the
// sandbox CLI, so their selections remain unknown. Never infer them from flags.
type selectionStream struct {
	selection *runtimeSelection
	line      []byte
	longLine  bool
	json      []byte
	depth     int
	inString  bool
	escaped   bool
	largeJSON bool
	codex     int
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
	// An absent or malformed model still establishes the agent, and clears the
	// previous attempt's model rather than treating it as the current selection.
	_ = json.Unmarshal(event.Model, &model)
	s.selection.set("claude", model)
	s.codex = 0
}

// flushLine also collects an unterminated final banner line after cancellation.
func (s *selectionStream) flushLine() {
	if !s.longLine {
		line := strings.TrimSpace(terminalEscape.ReplaceAllString(string(s.line), ""))
		switch {
		case strings.HasPrefix(line, "OpenAI Codex v"):
			s.selection.set("codex", "")
			s.codex = 1
		case line == "--------" && s.codex == 1:
			s.codex = 2
		case line == "--------" && s.codex == 2:
			s.codex = 0
		case s.codex == 2 && strings.HasPrefix(line, "model:"):
			model := strings.TrimSpace(strings.TrimPrefix(line, "model:"))
			if fields := strings.Fields(model); len(fields) > 0 {
				model = fields[0]
			}
			s.selection.set("codex", model)
		}
	}
	s.line, s.longLine = nil, false
}
