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
	discard     bool
	state       selectionState
	separator   bool
	headerLines int
	object      claudeObject
}

// claudeObject scans one candidate top-level JSON object incrementally. Each
// byte is scanned once, and each top-level member is decoded once when it
// ends, so the work per output byte stays constant however the object is
// split across lines or writes.
type claudeObject struct {
	active    bool
	size      int // bytes of the object's lines, bounded by selectionBufferLimit
	depth     int
	inString  bool
	escaped   bool
	invalid   bool   // a malformed member ends field extraction
	member    []byte // current top-level member, without its separator
	eventType string
	subtype   string
	model     string
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
				w.endObject()
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
	if w.state != selectionLocked {
		w.endObject()
	}
	w.line = nil
}

// lock ends observation once the first startup record is complete.
func (w *selectionWriter) lock() {
	w.state = selectionLocked
	w.line, w.object = nil, claudeObject{}
}

func (w *selectionWriter) observeLine(line string) {
	line = strings.TrimSuffix(terminalEscape.ReplaceAllString(line, ""), "\r")
	switch w.state {
	case selectionCodex:
		w.observeCodexHeader(line)
	case selectionPending:
		if w.launched != "claude" && codexVersion.MatchString(line) {
			// A version line ends any candidate object; its complete trailing
			// field may still identify an earlier Claude init event.
			if w.endObject(); w.state == selectionClaude {
				w.lock()
				return
			}
			w.state, w.separator, w.headerLines = selectionCodex, false, 0
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
		// A new top-level object ends any earlier candidate, which may still
		// complete an init event through its trailing field.
		if w.endObject(); w.state == selectionClaude {
			// The init event ended truncated.
			w.lock()
			return
		}
		w.object = claudeObject{active: true, member: w.object.member[:0]}
	} else if !w.object.active {
		return
	}
	if w.object.size+len(line)+1 > selectionBufferLimit {
		// An oversized object is not a startup record. Stop observing rather
		// than keep scanning output that has supplied no selection, so the
		// pre-lock work stays bounded and the selection stays as recorded.
		w.endObject()
		w.lock()
		return
	}
	w.object.size += len(line) + 1
	w.scanClaudeObject(line)
}

// scanClaudeObject advances the candidate object through one line (and its
// newline). Complete top-level fields are read even if interruption truncates
// the rest of an init event; fields on later lines complete the same event.
func (w *selectionWriter) scanClaudeObject(line string) {
	o := &w.object
	for i := 0; i <= len(line); i++ {
		c := byte('\n')
		if i < len(line) {
			c = line[i]
		}
		if o.inString {
			o.member = append(o.member, c)
			switch {
			case o.escaped:
				o.escaped = false
			case c == '\\':
				o.escaped = true
			case c == '"':
				o.inString = false
			}
			continue
		}
		switch c {
		case '"':
			o.inString = true
		case '{', '[':
			o.depth++
			if o.depth == 1 {
				continue // the object's opening brace
			}
		case '}', ']':
			o.depth--
			if o.depth == 0 {
				if c == ']' {
					o.invalid = true
				}
				w.claudeMember(true)
				w.object = claudeObject{member: o.member[:0]}
				if w.state == selectionClaude {
					w.lock()
				}
				return
			}
		case ',':
			if o.depth == 1 {
				w.claudeMember(false)
				continue
			}
		}
		o.member = append(o.member, c)
	}
}

// endObject abandons the candidate object after reading its complete
// trailing field, if any.
func (w *selectionWriter) endObject() {
	if w.object.active && w.object.depth == 1 && !w.object.inString {
		w.claudeMember(true)
	}
	w.object = claudeObject{member: w.object.member[:0]}
}

// claudeMember decodes the completed top-level member and reports the
// selection once the object is recognized as an init event. last reports
// that no separator followed the member, so it may be empty.
func (w *selectionWriter) claudeMember(last bool) {
	o := &w.object
	member := bytes.TrimSpace(o.member)
	o.member = o.member[:0]
	if o.invalid {
		return
	}
	if len(member) == 0 {
		o.invalid = !last
		return
	}
	var field map[string]json.RawMessage
	wrapped := append(append(append(make([]byte, 0, len(member)+2), '{'), member...), '}')
	if err := json.Unmarshal(wrapped, &field); err != nil || len(field) != 1 {
		o.invalid = true
		return
	}
	for key, value := range field {
		switch key {
		case "type":
			_ = json.Unmarshal(value, &o.eventType)
		case "subtype":
			_ = json.Unmarshal(value, &o.subtype)
		case "model":
			_ = json.Unmarshal(value, &o.model)
		}
	}
	if o.eventType == "system" && o.subtype == "init" {
		w.state = selectionClaude
		w.log.RuntimeSelection("claude", o.model)
	}
}
