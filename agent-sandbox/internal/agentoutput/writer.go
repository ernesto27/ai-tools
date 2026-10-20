package agentoutput

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// Writer belongs to one execution. Docker's stdout pump serializes writes.
type Writer struct {
	out     io.Writer
	decoder Decoder
	pending []byte
	started map[eventKey]time.Time
	now     func() time.Time
	err     error
	closed  bool
}

// New starts a feed for the resolved worktree branch. It never closes out.
func New(out io.Writer, branch string, decoder Decoder) *Writer {
	w := &Writer{out: out, decoder: decoder, started: make(map[eventKey]time.Time), now: time.Now}
	w.write(decoder.Name() + " / " + strings.Join(strings.Fields(clean(branch)), " ") + "\n" + separator)
	return w
}

// Write buffers partial records because Docker can split JSON and UTF-8 at
// arbitrary byte boundaries. Complete records are rendered before EOF.
func (w *Writer) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.closed {
		return 0, io.ErrClosedPipe
	}
	w.pending = append(w.pending, p...)
	for {
		end := bytes.IndexByte(w.pending, '\n')
		if end < 0 {
			if len(w.pending) == 0 {
				w.pending = nil
			}
			return len(p), nil
		}
		w.consume(w.pending[:end])
		w.pending = w.pending[end+1:]
		if w.err != nil {
			return 0, w.err
		}
	}
}

// Close flushes trailing output, including on cancellation. The footer makes
// no claim about execution or publication; the container exit is authoritative.
func (w *Writer) Close() error {
	if w.closed {
		return w.err
	}
	w.closed = true
	if len(w.pending) > 0 && w.err == nil {
		w.consume(w.pending)
	}
	w.pending = nil
	w.started = nil
	w.write(separator)
	return w.err
}

func (w *Writer) consume(line []byte) {
	if len(bytes.TrimSpace(line)) == 0 {
		return
	}
	now := w.now()
	events, err := w.decoder.Decode(line)
	if err != nil {
		w.entry(now, LabelOutput, "Unrecognized "+w.decoder.Name()+" output: "+err.Error()+"\n"+string(line))
		return
	}
	for _, event := range events {
		w.render(now, event)
	}
}

func (w *Writer) render(now time.Time, e Event) {
	if e.Kind == KindUsage {
		if e.Usage == nil {
			return
		}
		record := struct {
			Type  Kind        `json:"type"`
			Agent string      `json:"agent"`
			Usage *TokenUsage `json:"usage"`
		}{Type: KindUsage, Agent: w.decoder.Name(), Usage: e.Usage}
		data, err := json.Marshal(record)
		if err != nil {
			w.err = err
			return
		}
		// Usage stays on one unprefixed line so callers can parse it from stdout.
		w.write(string(data) + "\n")
		return
	}
	if e.Phase == PhaseStarted {
		if e.ID != "" {
			if _, exists := w.started[e.key()]; !exists {
				w.started[e.key()] = now
			}
		}
		label := LabelToolStart
		if e.Kind == KindCommand {
			label = LabelCommand
		}
		w.entry(now, label, e.Subject)
		return
	}
	var label string
	text := e.Subject
	switch e.Kind {
	case KindCommand, KindTool, KindFile:
		label = LabelResult
		switch e.Kind {
		case KindTool:
			label = LabelTool
		case KindFile:
			label = LabelFile
		}
		result := string(e.Status)
		if e.Kind == KindCommand && e.ExitCode != nil {
			result = fmt.Sprintf("exit %d", *e.ExitCode)
		} else if e.Detail != "" {
			result = e.Detail
		}
		text += "\n" + result + w.duration(e.key(), now)
	case KindAgent:
		label = LabelAgent
	case KindThinking:
		label = LabelThinking
	case KindSearch:
		label = LabelSearch
	case KindPlan:
		label = LabelPlan
	case KindFailure:
		label = LabelFailed
	case KindNotice:
		label = LabelEvent
	}
	if e.failed() {
		label = LabelFailed
		if e.Output != "" {
			text += "\n" + e.Output
		}
	}
	w.entry(now, label, text)
}

func (w *Writer) entry(now time.Time, label, text string) {
	w.write(now.Format(timeLayout) + "  " + label + "\n")
	text = strings.TrimRight(clean(text), "\n")
	if text != "" {
		w.write(detailIndent + strings.ReplaceAll(text, "\n", "\n"+detailIndent) + "\n")
	}
	w.write("\n")
}

func (w *Writer) write(text string) {
	if w.err != nil {
		return
	}
	n, err := io.WriteString(w.out, text)
	if err == nil && n != len(text) {
		err = io.ErrShortWrite
	}
	w.err = err
}

// Durations measure receipt on the host, not execution timing supplied by an agent.
func (w *Writer) duration(key eventKey, now time.Time) string {
	started, ok := w.started[key]
	delete(w.started, key)
	if !ok {
		return ""
	}
	return fmt.Sprintf(" / %.1fs", now.Sub(started).Seconds())
}

// Both plain output and the TUI must prevent agent text from controlling the terminal.
func clean(text string) string {
	text = ansi.Strip(strings.ToValidUTF8(text, "�"))
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return ' '
		}
		if r != '\n' && unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
}
