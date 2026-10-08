package main

import (
	"bytes"
	"encoding/json"
	"io"
)

func agentOutputFormatter(name string) func(io.Writer) io.WriteCloser {
	if name != "claude" {
		return nil
	}
	return func(out io.Writer) io.WriteCloser { return &jsonEventWriter{out: out} }
}

// Claude emits one JSON event per line, but Docker can split that line across
// writes. Buffer only the incomplete event so completed events appear live.
type jsonEventWriter struct {
	out     io.Writer
	pending []byte
	err     error
}

func (w *jsonEventWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
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
		line := w.pending[:end]
		if err := w.writeEvent(line, true); err != nil {
			w.err = err
			return 0, err
		}
		w.pending = w.pending[end+1:]
	}
}

func (w *jsonEventWriter) writeEvent(line []byte, newline bool) error {
	var formatted bytes.Buffer
	// Indent preserves field order, numbers, and string escapes. Diagnostics
	// that are not valid JSON pass through without becoming formatting errors.
	if err := json.Indent(&formatted, line, "", "  "); err == nil {
		line = formatted.Bytes()
	}
	if newline {
		line = append(line, '\n')
	}
	n, err := w.out.Write(line)
	if err == nil && n != len(line) {
		err = io.ErrShortWrite
	}
	return err
}

// Close flushes a final event without a newline, including on cancellation.
// The destination belongs to the CLI and remains open for its status messages.
func (w *jsonEventWriter) Close() error {
	if w.err != nil {
		return w.err
	}
	if len(w.pending) == 0 {
		return nil
	}
	line := w.pending
	w.pending = nil
	return w.writeEvent(line, false)
}
