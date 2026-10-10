package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime/debug"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/docker"
	"github.com/ernesto27/ai-tools/agent-sandbox/internal/sandbox"
)

// liveBridge batches writes without asking the renderer to keep up with each
// byte. Only the model retains scrollback; neither queue nor history has a cap.
type liveBridge struct {
	mu      sync.Mutex
	pending bytes.Buffer
	decoder liveTextDecoder
	events  []sandbox.Event
	log     io.Writer
	logErr  error
	mode    string
}

func (b *liveBridge) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	text := b.decoder.feed(p)
	b.pending.WriteString(text)
	b.writeLog(text)
	return len(p), nil
}

// Callers hold mu while writing the log. A failed log must never stop Docker's
// output pump: an undrained attachment can block the agent while we wait for
// its exit. Retain the first error and stop attempting further file writes.
func (b *liveBridge) writeLog(text string) {
	if b.log == nil || b.logErr != nil || text == "" {
		return
	}
	n, err := io.WriteString(b.log, text)
	if err == nil && n != len(text) {
		err = io.ErrShortWrite
	}
	if err != nil {
		b.logErr = fmt.Errorf("writing TUI transcript: %w", err)
	}
}

func (b *liveBridge) logError() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.logErr
}

func (b *liveBridge) observe(event sandbox.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, event)
	b.writeLog(fmt.Sprintf("\n[%s] %+v\n", b.mode, event))
}

func (b *liveBridge) drain(final bool) (string, []sandbox.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if final && len(b.decoder.partial) > 0 {
		partial := string(b.decoder.partial)
		b.decoder.partial = nil
		text := b.decoder.feed([]byte(strings.ToValidUTF8(partial, "�")))
		b.pending.WriteString(text)
		b.writeLog(text)
	}
	text, events := b.pending.String(), b.events
	b.pending.Reset()
	b.events = nil
	return text, events
}

// Escape parsing persists across writes, including split UTF-8, CSI and OSC
// sequences. Agent cursor movement and terminal commands cannot reach the host.
type liveTextDecoder struct {
	partial []byte
	state   string
	cr      bool
}

func (d *liveTextDecoder) feed(data []byte) string {
	data = append(d.partial, data...)
	d.partial = nil
	var out strings.Builder
	for len(data) > 0 {
		if !utf8.FullRune(data) {
			d.partial = append(d.partial, data...)
			break
		}
		r, n := utf8.DecodeRune(data)
		data = data[n:]
		switch d.state {
		case "escape":
			switch r {
			case '[':
				d.state = "csi"
			case ']', 'P', 'X', '^', '_':
				d.state = "string"
			default:
				if r >= 0x20 && r <= 0x2f {
					d.state = "intermediate"
				} else {
					d.state = ""
				}
			}
			continue
		case "intermediate":
			if r >= 0x30 && r <= 0x7e {
				d.state = ""
			}
			continue
		case "csi":
			if r >= 0x40 && r <= 0x7e {
				d.state = ""
			}
			continue
		case "string":
			if r == '\a' || r == 0x9c {
				d.state = ""
			} else if r == 0x1b {
				d.state = "string escape"
			}
			continue
		case "string escape":
			if r == '\\' || r == '\a' || r == 0x9c {
				d.state = ""
			} else if r != 0x1b {
				d.state = "string"
			}
			continue
		}
		switch r {
		case 0x1b:
			d.state = "escape"
		case 0x9b:
			d.state = "csi"
		case 0x90, 0x98, 0x9d, 0x9e, 0x9f:
			d.state = "string"
		case '\r':
			out.WriteByte('\n')
			d.cr = true
		case '\n':
			if !d.cr {
				out.WriteByte('\n')
			}
			d.cr = false
		case '\t':
			out.WriteString("    ")
			d.cr = false
		default:
			if r >= 0x20 && (r < 0x7f || r > 0x9f) {
				out.WriteRune(r)
				d.cr = false
			}
		}
	}
	return out.String()
}

func livePlain(text string) string {
	var decoder liveTextDecoder
	return decoder.feed([]byte(text))
}

type liveJob struct {
	ctx    context.Context
	mode   string
	start  chan struct{}
	done   chan struct{}
	sizes  chan docker.TerminalSize
	status int
	err    error
}

func newLiveJob(ctx context.Context, opts sandbox.Options, bridge *liveBridge, mode string) *liveJob {
	j := &liveJob{ctx: ctx, mode: mode, start: make(chan struct{}), done: make(chan struct{}),
		sizes: make(chan docker.TerminalSize, 1)}
	go func() {
		defer close(j.done)
		// A worker panic must become a logged error, rather than killing the
		// process while the terminal is still displaying the alternate screen.
		defer func() {
			if recovered := recover(); recovered != nil {
				j.err = fmt.Errorf("%s worker panic: %v", mode, recovered)
				bridge.Write([]byte(fmt.Sprintf("\nPANIC: %v\n%s\n", recovered, debug.Stack())))
			}
		}()
		select {
		case <-ctx.Done():
			j.err = ctx.Err()
			return
		case <-j.start:
		}
		execute := sandbox.Run
		if mode == "resume" {
			execute = sandbox.Resume
		}
		j.status, j.err = execute(ctx, opts, sandbox.Runtime{
			Output: bridge, Sizes: j.sizes, Observe: bridge.observe, ViewOwnsTerminal: true,
			FormatStdout: agentOutputFormatter(opts.Agent.Name()),
		})
		if j.err == nil && ctx.Err() != nil {
			j.err = ctx.Err()
		}
	}()
	return j
}
