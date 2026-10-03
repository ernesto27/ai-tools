package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"agent-sandbox/internal/sandbox"
)

func TestRunArgsAcceptsFilePromptWithoutPositionalPrompt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prompt.md")
	want := "# Fix login.\n"
	if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	cmd := newRunCmd(&commandState{})
	if err := cmd.Flags().Set("file-prompt", path); err != nil {
		t.Fatalf("set file-prompt: %v", err)
	}

	if err := runArgs(cmd, nil); err != nil {
		t.Errorf("runArgs() error = %v, want nil", err)
	}
}

func TestImageFlagIsRepeatableWithQuery(t *testing.T) {
	cmd := newRunCmd(&commandState{})
	if err := cmd.Flags().Parse([]string{"--image", "first.png", "--image", "second.png", "-q", "describe them"}); err != nil {
		t.Fatal(err)
	}

	images, err := cmd.Flags().GetStringArray("image")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"first.png", "second.png"}; !reflect.DeepEqual(images, want) {
		t.Fatalf("--image values = %q, want %q", images, want)
	}
	if args := cmd.Flags().Args(); len(args) != 0 {
		t.Fatalf("positional args = %q, want none", args)
	}
	if query, err := cmd.Flags().GetString("query"); err != nil || query != "describe them" {
		t.Fatalf("query = %q, err = %v, want describe them", query, err)
	}
}

func TestRunArgsRejectsBothFlagPromptSources(t *testing.T) {
	cmd := newRunCmd(&commandState{})
	if err := cmd.Flags().Set("file-prompt", "prompt.md"); err != nil {
		t.Fatalf("set file-prompt: %v", err)
	}
	if err := cmd.Flags().Set("query", "different task"); err != nil {
		t.Fatalf("set query: %v", err)
	}

	err := runArgs(cmd, nil)
	assertUsageError(t, err)
}

func TestRunArgsRejectsMissingPromptSource(t *testing.T) {
	cmd := newRunCmd(&commandState{})
	assertUsageError(t, runArgs(cmd, nil))
}

func TestResumeRequiresBranch(t *testing.T) {
	cmd, _ := newRootCmd()
	cmd.SetArgs([]string{"resume", "-a", "codex", "-q", "continue"})

	assertUsageError(t, cmd.Execute())
}

func assertUsageError(t *testing.T, err error) {
	t.Helper()
	var usageErr sandbox.UsageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("error = %v, want UsageError", err)
	}
}
