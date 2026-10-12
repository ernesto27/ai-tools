package sandbox

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"soft-factory/internal/executionlog"
)

const selectionBufferLimit = 64 * 1024

type selectionState int

const (
	selectionPending selectionState = iota
	selectionCodex
	selectionLocked
)

// selectionWriter passes output through and records the first startup record
// for the launched agent. Later agent-printed lookalikes are ignored; each
// sandbox attempt gets a new writer.
type selectionWriter struct {
	out       io.Writer
	log       *executionlog.Run
	launched  string
	line      []byte
	object    []byte
	discard   bool
	state     selectionState
	separator bool
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
				if len(w.object) != 0 || w.state == selectionCodex {
					w.finishClaudeObject()
					w.lock()
				}
				w.line = nil
				w.discard = true
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
	w.finishClaudeObject()
	w.line = nil
}

func (w *selectionWriter) lock() {
	w.state = selectionLocked
	w.line, w.object = nil, nil
}

func (w *selectionWriter) observeLine(line string) {
	line = strings.TrimSuffix(line, "\r")
	if w.state == selectionCodex {
		switch {
		case line == "--------" && !w.separator:
			w.separator = true
		case w.separator && strings.HasPrefix(line, "model:"):
			w.log.RuntimeSelection("codex", strings.TrimSpace(strings.TrimPrefix(line, "model:")))
			w.lock()
		case line == "--------" || line == "user" || line == "":
			w.lock()
		}
		return
	}

	// A terminal title or color sequence can precede the version banner.
	banner := line
	if strings.HasPrefix(banner, "\x1b") {
		if i := strings.Index(banner, "OpenAI Codex v"); i >= 0 {
			banner = banner[i:]
		}
	}
	version, ok := strings.CutPrefix(banner, "OpenAI Codex v")
	if w.launched != "claude" && ok && len(version) > 0 && version[0] >= '0' && version[0] <= '9' {
		w.finishClaudeObject()
		if w.state != selectionLocked {
			w.state = selectionCodex
			w.log.RuntimeSelection("codex", "")
		}
		return
	}
	if w.launched == "codex" {
		return
	}

	if strings.HasPrefix(line, "{") {
		w.finishClaudeObject()
		if w.state == selectionLocked {
			return
		}
	} else if len(w.object) == 0 {
		return
	}
	if len(w.object)+len(line)+1 > selectionBufferLimit {
		w.finishClaudeObject()
		w.lock()
		return
	}
	w.object = append(w.object, line...)
	w.object = append(w.object, '\n')
	// The sandbox emits indented objects with an unindented closing brace,
	// or a compact JSON object on one line. Decode once at the boundary.
	if line == "}" || strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
		w.finishClaudeObject()
	}
}

// finishClaudeObject reads top-level fields using Go's JSON decoder. Complete
// fields remain available if the event was truncated by interruption or the
// buffer limit, without rescanning the object after each output line.
func (w *selectionWriter) finishClaudeObject() {
	if len(w.object) == 0 {
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(w.object))
	w.object = nil
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return
	}
	var eventType, subtype, model string
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			break
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
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
		w.log.RuntimeSelection("claude", model)
		w.lock()
	}
}
