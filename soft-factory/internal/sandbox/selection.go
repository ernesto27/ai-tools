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

var terminalEscape = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
var codexVersion = regexp.MustCompile(`^OpenAI Codex v[0-9]+\.[0-9]+\.[0-9]+(?:[ +\-].*)?$`)

// selectionWriter observes runtime startup records, never configured defaults
// or agent-authored change reports. The sandbox runs Codex with a TTY (its
// startup header therefore reaches stdout), and Claude with stream-json,
// formatted as multiline JSON by the sandbox. Unrecognized output supplies no
// selection evidence. Each startup replaces both fields, including omissions.
type selectionWriter struct {
	out       io.Writer
	log       *executionlog.Run
	line      []byte
	object    []byte
	discard   bool
	codex     bool
	separator bool
}

func (w *selectionWriter) Write(p []byte) (int, error) {
	for _, b := range p {
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
			} else {
				w.line = append(w.line, b)
			}
		}
	}
	return w.out.Write(p)
}

func (w *selectionWriter) flush() {
	if len(w.line) != 0 && !w.discard {
		w.observeLine(string(w.line))
	}
	w.line, w.object = nil, nil
}

func (w *selectionWriter) observeLine(line string) {
	line = strings.TrimSuffix(terminalEscape.ReplaceAllString(line, ""), "\r")
	if codexVersion.MatchString(line) {
		w.codex, w.separator = true, false
		w.object = nil
		w.log.RuntimeSelection("codex", "")
		return
	}
	if w.codex {
		if line == "--------" && !w.separator {
			w.separator = true
			return
		}
		if w.separator && strings.HasPrefix(line, "model:") {
			w.log.RuntimeSelection("codex", strings.TrimSpace(strings.TrimPrefix(line, "model:")))
			return
		}
		if line == "--------" || line == "user" || line == "" {
			w.codex = false
		}
	}

	// Only top-level JSON objects are candidates. Nested model fields and
	// assistant text cannot be mistaken for a Claude initialization record.
	if strings.HasPrefix(line, "{") {
		w.object = w.object[:0]
	} else if len(w.object) == 0 {
		return
	}
	if len(w.object)+len(line)+1 > selectionBufferLimit {
		w.object = nil
		return
	}
	w.object = append(w.object, line...)
	w.object = append(w.object, '\n')
	// Read complete top-level fields even if interruption truncates the rest
	// of an init event. A recognized new init must discard an earlier model.
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
		w.codex = false
		w.log.RuntimeSelection("claude", model)
	}
	if json.Valid(w.object) {
		w.object = nil
	}
}
