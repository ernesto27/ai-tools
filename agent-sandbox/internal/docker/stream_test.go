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
	"github.com/stretchr/testify/require"
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
			{
				_, err := io.WriteString(out, tc.stdout[:midpoint])
				require.NoError(t, err)
			}
			{
				_, err := io.WriteString(errOut, "stdin warning\n")
				require.NoError(t, err)
			}
			{
				_, err := io.WriteString(out, tc.stdout[midpoint:])
				require.NoError(t, err)
			}
			client := &Client{stdout: &stdout, stderr: &stderr}
			done := client.pump(types.HijackedResponse{Reader: bufio.NewReader(&frames)}, RunOptions{})
			require.NoError(t, awaitPump(t, done))
			require.Equal(t, tc.stdout, stdout.String())
			require.Equal(t, "stdin warning\n", stderr.String())
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
	require.NoError(t, writer.SetWriteDeadline(time.Now().Add(5*time.Second)))
	t.Cleanup(func() { reader.Close(); writer.Close() })
	output := &observedOutput{writes: make(chan string, 2)}
	client := &Client{stdout: output, stderr: io.Discard}
	done := client.pump(types.HijackedResponse{Reader: bufio.NewReader(reader)}, RunOptions{})
	partial := "{\"type\":\"stream_event\",\"text\":\"working\"}\n"
	{
		_, err := io.WriteString(stdcopy.NewStdWriter(writer, stdcopy.Stdout), partial)
		require.NoError(t, err)
	}
	select {
	case got := <-output.writes:
		require.Equal(t, partial, got)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "Unexpected test failure", "output did not arrive before EOF")
	}
	select {
	case err := <-done:
		require.FailNowf(t, "Unexpected test failure", "pump stopped before EOF: %v", err)
	default:
	}
	final := "{\"type\":\"result\",\"result\":\"done\"}"
	{
		_, err := io.WriteString(stdcopy.NewStdWriter(writer, stdcopy.Stdout), final)
		require.NoError(t, err)
	}
	writer.Close()
	require.NoError(t, awaitPump(t, done))
	require.Equal(t, partial+final, output.String())
}

type failedOutput struct{ err error }

func (w failedOutput) Write([]byte) (int, error) { return 0, w.err }

func TestPumpReportsOutputFailure(t *testing.T) {
	for _, stream := range []stdcopy.StdType{stdcopy.Stdout, stdcopy.Stderr} {
		t.Run(map[stdcopy.StdType]string{stdcopy.Stdout: "stdout", stdcopy.Stderr: "stderr"}[stream], func(t *testing.T) {
			var frames bytes.Buffer
			{
				_, err := io.WriteString(stdcopy.NewStdWriter(&frames, stream), "event\n")
				require.NoError(t, err)
			}
			failure := errors.New("output unavailable")
			client := &Client{stdout: failedOutput{failure}, stderr: failedOutput{failure}}
			done := client.pump(types.HijackedResponse{Reader: bufio.NewReader(&frames)}, RunOptions{})
			require.ErrorIs(t, awaitPump(t, done), failure)
		})
	}
}

func awaitPump(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		require.FailNow(t, "Unexpected test failure", "output pump did not finish")
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
			{
				_, err := io.WriteString(stdcopy.NewStdWriter(&frames, stdcopy.Stdout), "event without newline")
				require.NoError(t, err)
			}
			{
				_, err := io.WriteString(stdcopy.NewStdWriter(&frames, stdcopy.Stderr), "Warning: unchanged\n")
				require.NoError(t, err)
			}
			formatter := &bufferedTestFormatter{closeErr: tc.closeErr}
			opts := RunOptions{FormatStdout: func(out io.Writer) io.WriteCloser { formatter.out = out; return formatter }}
			client := &Client{stdout: &stdout, stderr: &stderr}
			err := awaitPump(t, client.pump(types.HijackedResponse{Reader: bufio.NewReader(&frames)}, opts))
			require.ErrorIs(t, err, tc.closeErr)
			require.True(t, formatter.closed, "formatter was not closed")
			if tc.closeErr == nil {
				require.Equal(t, "EVENT WITHOUT NEWLINE", stdout.String())
			}
			require.Equal(t, "Warning: unchanged\n", stderr.String())
		})
	}
}
