package agentoutput

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type scriptedDecoder struct {
	events []Event
}

func (*scriptedDecoder) Name() string { return "test-agent" }

func (d *scriptedDecoder) Decode([]byte) ([]Event, error) {
	if len(d.events) == 0 {
		return nil, fmt.Errorf("unexpected decoder call")
	}
	event := d.events[0]
	d.events = d.events[1:]
	return []Event{event}, nil
}

func TestWriterRendering(t *testing.T) {
	zero, nonzero := 0, 1
	type step struct {
		after time.Duration
		event Event
		want  string
	}
	for _, tc := range []struct {
		name  string
		steps []step
	}{
		{
			name: "successful command hides output",
			steps: []step{
				{0, Event{ID: "c1", Kind: KindCommand, Phase: PhaseStarted, Subject: "go test ./...", Status: StatusRunning},
					"10:32:02  > COMMAND\n            go test ./...\n\n"},
				{800 * time.Millisecond, Event{ID: "c1", Kind: KindCommand, Phase: PhaseCompleted, Subject: "go test ./...", Status: StatusCompleted, ExitCode: &zero, Output: "verbose output"},
					"10:32:03  + RESULT\n            go test ./...\n            exit 0 / 0.8s\n\n"},
			},
		},
		{
			name: "nonzero exit overrides completed status",
			steps: []step{
				{0, Event{ID: "c1", Kind: KindCommand, Phase: PhaseStarted, Subject: "go test ./..."},
					"10:32:02  > COMMAND\n            go test ./...\n\n"},
				{800 * time.Millisecond, Event{ID: "c1", Kind: KindCommand, Phase: PhaseCompleted, Subject: "go test ./...", Status: StatusCompleted, ExitCode: &nonzero, Output: "TestLogin failed\nexpected /home"},
					"10:32:03  ! FAILED\n            go test ./...\n            exit 1 / 0.8s\n            TestLogin failed\n            expected /home\n\n"},
			},
		},
		{
			name: "tool lifecycle",
			steps: []step{
				{0, Event{ID: "t1", Kind: KindTool, Phase: PhaseStarted, Subject: "Context7 > query-docs"},
					"10:32:02  > TOOL\n            Context7 > query-docs\n\n"},
				{time.Second, Event{ID: "t1", Kind: KindTool, Phase: PhaseCompleted, Subject: "Context7 > query-docs", Status: StatusCompleted},
					"10:32:03  + TOOL\n            Context7 > query-docs\n            completed / 1.0s\n\n"},
			},
		},
		{
			name: "file completion detail",
			steps: []step{
				{0, Event{Kind: KindFile, Phase: PhaseCompleted, Subject: "auth/login.go", Status: StatusCompleted, Detail: "patch applied"},
					"10:32:02  * FILE\n            auth/login.go\n            patch applied\n\n"},
			},
		},
		{
			name: "completion without start has no duration",
			steps: []step{
				{0, Event{ID: "missing", Kind: KindCommand, Phase: PhaseCompleted, Subject: "pwd", ExitCode: &zero},
					"10:32:02  + RESULT\n            pwd\n            exit 0\n\n"},
			},
		},
		{
			name: "repeated start preserves original time",
			steps: []step{
				{0, Event{ID: "c1", Kind: KindCommand, Phase: PhaseStarted, Subject: "pwd"},
					"10:32:02  > COMMAND\n            pwd\n\n"},
				{time.Second, Event{ID: "c1", Kind: KindCommand, Phase: PhaseStarted, Subject: "pwd"},
					"10:32:03  > COMMAND\n            pwd\n\n"},
				{2 * time.Second, Event{ID: "c1", Kind: KindCommand, Phase: PhaseCompleted, Subject: "pwd", ExitCode: &zero},
					"10:32:04  + RESULT\n            pwd\n            exit 0 / 2.0s\n\n"},
			},
		},
		{
			name: "same ID has independent timing per scope",
			steps: []step{
				{0, Event{ID: "c1", Scope: "a", Kind: KindCommand, Phase: PhaseStarted, Subject: "first"},
					"10:32:02  > COMMAND\n            first\n\n"},
				{time.Second, Event{ID: "c1", Scope: "b", Kind: KindCommand, Phase: PhaseStarted, Subject: "second"},
					"10:32:03  > COMMAND\n            second\n\n"},
				{2 * time.Second, Event{ID: "c1", Scope: "b", Kind: KindCommand, Phase: PhaseCompleted, Subject: "second", ExitCode: &zero},
					"10:32:04  + RESULT\n            second\n            exit 0 / 1.0s\n\n"},
				{3 * time.Second, Event{ID: "c1", Scope: "a", Kind: KindCommand, Phase: PhaseCompleted, Subject: "first", ExitCode: &zero},
					"10:32:05  + RESULT\n            first\n            exit 0 / 3.0s\n\n"},
			},
		},
		{
			name: "sanitize rendered agent text",
			steps: []step{
				{0, Event{Kind: KindAgent, Subject: "\x1b[2J\x1b[31mHello\x1b[0m\r\nworld\t!\x07"},
					"10:32:02    AGENT\n            Hello\n            world !\n\n"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decoder := &scriptedDecoder{}
			for _, s := range tc.steps {
				decoder.events = append(decoder.events, s.event)
			}
			var out bytes.Buffer
			writer := New(&out, "fix-login", decoder)
			{
				got, want := out.String(), "test-agent / fix-login\n---------------------------------------------\n"
				require.Equal(t, want, got)
			}
			out.Reset()
			base := time.Date(2026, 1, 1, 10, 32, 2, 200_000_000, time.UTC)
			for i, s := range tc.steps {
				writer.now = func() time.Time { return base.Add(s.after) }
				{
					n, err := writer.Write([]byte("event\n"))
					require.NoError(t, err)
					require.Equal(t, len("event\n"), n)
				}
				require.Equal(t, s.want, out.String(), "step %d", i)
				out.Reset()
			}
			require.NoError(t, writer.Close())
			require.Equal(t, "---------------------------------------------\n", out.String())
		})
	}
}

type closeTrackingOutput struct {
	bytes.Buffer
	closed bool
}

func (w *closeTrackingOutput) Close() error {
	w.closed = true
	return nil
}

func TestWriterCloseWithPendingCommand(t *testing.T) {
	out := &closeTrackingOutput{}
	decoder := &scriptedDecoder{events: []Event{{ID: "c1", Kind: KindCommand, Phase: PhaseStarted, Subject: "long-running"}}}
	writer := New(out, "fix-login", decoder)
	writer.now = func() time.Time { return time.Date(2026, 1, 1, 10, 32, 2, 0, time.UTC) }
	// A canceled execution can end with a start event lacking a final newline.
	{
		_, err := writer.Write([]byte("start"))
		require.NoError(t, err)
	}
	out.Reset()
	require.NoError(t, writer.Close())
	want := "10:32:02  > COMMAND\n            long-running\n\n---------------------------------------------\n"
	require.Equal(t, want, out.String())
	require.False(t, out.closed, "writer closed its destination")
	{
		_, err := out.WriteString("host status\n")
		require.NoError(t, err)
	}
}

func TestClean(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"ANSI color", "\x1b[31mfailed\x1b[0m", "failed"},
		{"clear screen", "\x1b[2Jhello", "hello"},
		{"line endings", "first\r\nsecond\rthird", "first\nsecond\nthird"},
		{"tabs", "first\tsecond", "first second"},
		{"control characters", "a\x00\x07b", "ab"},
		{"invalid UTF-8", "a\xffb", "a�b"},
		{"Unicode", "你好", "你好"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, clean(tc.input))
		})
	}
}
