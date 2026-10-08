package docker

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
)

func TestPumpPreservesStreamJSON(t *testing.T) {
	for _, tc := range []struct{ name, stdout string }{
		{"events", "{\"type\":\"stream_event\",\"text\":\"hello\"}\n{\"type\":\"assistant\",\"text\":\"hello\"}\n{\"type\":\"result\",\"result\":\"hello\"}\n"},
		{"unknown and non JSON", "{\"type\":\"future_event\"}\nplain diagnostic\n\n"},
		{"Unicode and escapes", "{\"text\":\"¡你好!\\n\\t\\u001b[31m\"}\n"},
		{"large event", "{\"text\":\"" + strings.Repeat("x", 128*1024) + "\"}\n"},
		{"no final newline", "{\"type\":\"result\"}"},
		{"empty", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var frames, stdout, stderr bytes.Buffer
			out := stdcopy.NewStdWriter(&frames, stdcopy.Stdout)
			errOut := stdcopy.NewStdWriter(&frames, stdcopy.Stderr)
			// Split even within Unicode and JSON tokens, with stderr interleaved.
			midpoint := len(tc.stdout) / 2
			if _, err := io.WriteString(out, tc.stdout[:midpoint]); err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(errOut, "stdin warning\n"); err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(out, tc.stdout[midpoint:]); err != nil {
				t.Fatal(err)
			}
			client := &Client{stdout: &stdout, stderr: &stderr}
			done := client.pump(types.HijackedResponse{Reader: bufio.NewReader(&frames)}, RunOptions{})
			if err := awaitPump(t, done); err != nil {
				t.Fatal(err)
			}
			if stdout.String() != tc.stdout {
				t.Fatal("stdout changed or lost bytes")
			}
			if stderr.String() != "stdin warning\n" {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

// A channel observes writes without reading the pump's buffer concurrently.
type observedOutput struct {
	bytes.Buffer
	writes chan string
}

func (w *observedOutput) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	w.writes <- string(p)
	return n, err
}

func TestPumpForwardsBeforeEOF(t *testing.T) {
	reader, writer := net.Pipe()
	if err := writer.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reader.Close(); writer.Close() })
	output := &observedOutput{writes: make(chan string, 2)}
	client := &Client{stdout: output, stderr: io.Discard}
	done := client.pump(types.HijackedResponse{Reader: bufio.NewReader(reader)}, RunOptions{})
	partial := "{\"type\":\"stream_event\",\"text\":\"working\"}\n"
	if _, err := io.WriteString(stdcopy.NewStdWriter(writer, stdcopy.Stdout), partial); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-output.writes:
		if got != partial {
			t.Fatalf("live output = %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("output did not arrive before EOF")
	}
	select {
	case err := <-done:
		t.Fatalf("pump stopped before EOF: %v", err)
	default:
	}
	final := "{\"type\":\"result\",\"result\":\"done\"}"
	if _, err := io.WriteString(stdcopy.NewStdWriter(writer, stdcopy.Stdout), final); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	if err := awaitPump(t, done); err != nil {
		t.Fatal(err)
	}
	if output.String() != partial+final {
		t.Fatal("final output changed or lost bytes")
	}
}

type failedOutput struct{ err error }

func (w failedOutput) Write([]byte) (int, error) { return 0, w.err }

func TestPumpReportsOutputFailure(t *testing.T) {
	for _, stream := range []stdcopy.StdType{stdcopy.Stdout, stdcopy.Stderr} {
		t.Run(map[stdcopy.StdType]string{stdcopy.Stdout: "stdout", stdcopy.Stderr: "stderr"}[stream], func(t *testing.T) {
			var frames bytes.Buffer
			if _, err := io.WriteString(stdcopy.NewStdWriter(&frames, stream), "event\n"); err != nil {
				t.Fatal(err)
			}
			failure := errors.New("output unavailable")
			client := &Client{stdout: failedOutput{failure}, stderr: failedOutput{failure}}
			done := client.pump(types.HijackedResponse{Reader: bufio.NewReader(&frames)}, RunOptions{})
			if err := awaitPump(t, done); !errors.Is(err, failure) {
				t.Fatalf("error = %v, want %v", err, failure)
			}
		})
	}
}

func awaitPump(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("output pump did not finish")
		return nil
	}
}

// A formatter can retain a trailing event until the attachment reaches EOF.
type bufferedTestFormatter struct {
	out      io.Writer
	pending  bytes.Buffer
	closeErr error
	closed   bool
}

func (w *bufferedTestFormatter) Write(p []byte) (int, error) { return w.pending.Write(p) }
func (w *bufferedTestFormatter) Close() error {
	w.closed = true
	if w.closeErr != nil {
		return w.closeErr
	}
	_, err := w.out.Write(bytes.ToUpper(w.pending.Bytes()))
	return err
}

func TestPumpFormatsOnlyStdoutAndFlushes(t *testing.T) {
	failure := errors.New("formatter flush failed")
	for _, tc := range []struct {
		name     string
		closeErr error
	}{{"flush trailing event", nil}, {"flush failure", failure}} {
		t.Run(tc.name, func(t *testing.T) {
			var frames, stdout, stderr bytes.Buffer
			if _, err := io.WriteString(stdcopy.NewStdWriter(&frames, stdcopy.Stdout), "event without newline"); err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(stdcopy.NewStdWriter(&frames, stdcopy.Stderr), "Warning: unchanged\n"); err != nil {
				t.Fatal(err)
			}
			formatter := &bufferedTestFormatter{closeErr: tc.closeErr}
			opts := RunOptions{FormatStdout: func(out io.Writer) io.WriteCloser { formatter.out = out; return formatter }}
			client := &Client{stdout: &stdout, stderr: &stderr}
			err := awaitPump(t, client.pump(types.HijackedResponse{Reader: bufio.NewReader(&frames)}, opts))
			if !errors.Is(err, tc.closeErr) {
				t.Fatalf("pump error = %v, want %v", err, tc.closeErr)
			}
			if !formatter.closed {
				t.Fatal("formatter was not closed")
			}
			if tc.closeErr == nil && stdout.String() != "EVENT WITHOUT NEWLINE" {
				t.Fatalf("stdout = %q", stdout.String())
			}
			if stderr.String() != "Warning: unchanged\n" {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}
