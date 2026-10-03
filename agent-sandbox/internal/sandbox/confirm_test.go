package sandbox

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestConfirm(t *testing.T) {
	const question = "Delete worktree? [y/N]: "

	tests := []struct {
		name       string
		input      string
		want       bool
		wantOutput string
	}{
		{name: "single-letter yes", input: "y\n", want: true, wantOutput: question},
		{name: "word yes with whitespace and case", input: "  YeS \n", want: true, wantOutput: question},
		{name: "no", input: "n\n", wantOutput: question},
		{name: "unrecognized answer", input: "delete\n", wantOutput: question},
		{name: "empty input", wantOutput: question + "\n"},
		{name: "yes at EOF", input: "yes", want: true, wantOutput: question + "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := confirm(strings.NewReader(tt.input), &out, question)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("confirm(%q) = %t, want %t", tt.input, got, tt.want)
			}
			if got := out.String(); got != tt.wantOutput {
				t.Fatalf("output = %q, want %q", got, tt.wantOutput)
			}
		})
	}
}

func TestConfirmReturnsReadError(t *testing.T) {
	want := errors.New("read failed")
	var out bytes.Buffer

	_, err := confirm(errorReader{err: want}, &out, "Continue? [y/N]: ")
	if !errors.Is(err, want) {
		t.Fatalf("confirm() error = %v, want %v", err, want)
	}
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}
