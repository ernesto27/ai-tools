package sandbox

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"soft-factory/internal/executionlog"
)

const selectionBufferLimit = 64 * 1024

// codexHeaderLimit bounds how many lines after the version line may precede
// the header's model line. The real header reaches it within three lines
// (separator, workdir, model); the margin covers added header fields.
const codexHeaderLimit = 12

// terminalEscape matches CSI sequences and OSC sequences (such as window
// titles) terminated by BEL or ST.
var terminalEscape = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)
var codexVersion = regexp.MustCompile(`^OpenAI Codex v[0-9]+\.[0-9]+\.[0-9]+(?:[ +\-].*)?$`)

// selectionState tracks the first runtime startup record in one sandbox run.
type selectionState int

const (
	selectionPending selectionState = iota // no startup record recognized yet
	selectionCodex                         // Codex header block in progress
	selectionClaude                        // Claude init event in progress
	selectionLocked                        // first startup record ended
)

// selectionWriter observes runtime startup records, never configured defaults
// or agent-authored change reports. The sandbox runs Codex with a TTY (its
// startup header therefore reaches stdout), and Claude with stream-json,
// formatted as multiline JSON by the sandbox. Unrecognized output supplies no
// selection evidence. One writer observes one sandbox run, which launches one
// agent, so only the first startup record counts: once it ends, lookalike
// records printed later by the agent or its commands are ignored and output is
// only passed through. A later execute call is a new attempt with its own
// writer, and its startup record replaces both fields, including omissions.
//
// When the launched agent is known, only its own record kind is accepted, and
// agents without a recognized record (opencode, pi, ...) are not observed at
// all, so lookalikes they print cannot supply a selection. The launched agent
// is a filter only; it is never reported as the selection.
type selectionWriter struct {
	out         io.Writer
	log         *executionlog.Run
	launched    string // "" when undetermined: accept either record kind
	line        []byte
	object      []byte
	discard     bool
	state       selectionState
	separator   bool
	headerLines int
}

func newSelectionWriter(out io.Writer, log *executionlog.Run, launched string) *selectionWriter {
	w := &selectionWriter{out: out, log: log, launched: launched}
	if launched != "" && launched != "codex" && launched != "claude" {
		w.lock()
	}
	return w
}

func (w *selectionWriter) Write(p []byte) (int, error) {
	for _, b := range p {
		if w.state == selectionLocked {
			break
		}
		if b == '\n' {
			if !w.discard {
				w.observeLine(string(w.line))
			}
			w.line = w.line[:0]
			w.discard = false
		} else if !w.discard {
			if len(w.line) == selectionBufferLimit {
				w.line = w.line[:0]
				w.object = nil
				w.discard = true
				if w.state != selectionPending {
					// An oversized line cannot continue a startup record.
					w.lock()
				}
			} else {
				w.line = append(w.line, b)
			}
		}
	}
	return w.out.Write(p)
}

func (w *selectionWriter) flush() {
	if w.state != selectionLocked && len(w.line) != 0 && !w.discard {
		w.observeLine(string(w.line))
	}
	w.line, w.object = nil, nil
}

// lock ends observation once the first startup record is complete.
func (w *selectionWriter) lock() {
	w.state = selectionLocked
	w.line, w.object = nil, nil
}

func (w *selectionWriter) observeLine(line string) {
	line = strings.TrimSuffix(terminalEscape.ReplaceAllString(line, ""), "\r")
	switch w.state {
	case selectionCodex:
		w.observeCodexHeader(line)
	case selectionPending:
		if w.launched != "claude" && codexVersion.MatchString(line) {
			w.state, w.separator, w.headerLines, w.object = selectionCodex, false, 0, nil
			w.log.RuntimeSelection("codex", "")
			return
		}
		if w.launched != "codex" {
			w.observeClaudeInit(line)
		}
	case selectionClaude:
		w.observeClaudeInit(line)
	}
}

// observeCodexHeader completes the header that began with the version line.
// Version lines or JSON events appearing inside the header are ignored, and a
// header that has not reached its model line within codexHeaderLimit lines
// ends with the model unknown.
func (w *selectionWriter) observeCodexHeader(line string) {
	w.headerLines++
	switch {
	case w.headerLines > codexHeaderLimit:
		w.lock()
	case line == "--------" && !w.separator:
		w.separator = true
	case w.separator && strings.HasPrefix(line, "model:"):
		w.log.RuntimeSelection("codex", strings.TrimSpace(strings.TrimPrefix(line, "model:")))
		w.lock()
	case line == "--------" || line == "user" || line == "":
		w.lock()
	}
}

func (w *selectionWriter) observeClaudeInit(line string) {
	// Only top-level JSON objects are candidates. Nested model fields and
	// assistant text cannot be mistaken for a Claude initialization record.
	if strings.HasPrefix(line, "{") {
		if w.state == selectionClaude {
			// A new top-level object means the init event ended truncated.
			w.lock()
			return
		}
		w.object = w.object[:0]
	} else if len(w.object) == 0 {
		return
	}
	if len(w.object)+len(line)+1 > selectionBufferLimit {
		w.object = nil
		if w.state == selectionClaude {
			w.lock()
		}
		return
	}
	w.object = append(w.object, line...)
	w.object = append(w.object, '\n')
	// Read complete top-level fields even if interruption truncates the rest
	// of an init event. Fields on later lines complete the same event.
	decoder := json.NewDecoder(bytes.NewReader(w.object))
	_, err := decoder.Token() // opening object delimiter
	var eventType, subtype, model string
	for err == nil && decoder.More() {
		var key json.Token
		key, err = decoder.Token()
		if err != nil {
			break
		}
		var value json.RawMessage
		if err = decoder.Decode(&value); err != nil {
			break
		}
		switch key {
		case "type":
			_ = json.Unmarshal(value, &eventType)
		case "subtype":
			_ = json.Unmarshal(value, &subtype)
		case "model":
			_ = json.Unmarshal(value, &model)
		}
	}
	if eventType == "system" && subtype == "init" {
		w.state = selectionClaude
		w.log.RuntimeSelection("claude", model)
	}
	if json.Valid(w.object) {
		w.object = nil
		if w.state == selectionClaude {
			w.lock()
		}
	}
}
