package sandbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExecuteCancellationWaitsForChild(t *testing.T) {
	t.Chdir(t.TempDir())
	bin := t.TempDir()
	// The child acknowledges interruption before exiting.
	script := "#!/bin/sh\ntrap 'echo stopped > stopped; exit 0' INT\necho ready > ready\nwhile :; do :; done\n"
	if err := os.WriteFile(filepath.Join(bin, "agent-sandbox"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := execute(ExecuteOptions{Args: []string{"run"}, Context: ctx}); done <- err }()
	deadline := time.After(5 * time.Second)
	for {
		if _, err := os.Stat("ready"); err == nil {
			break
		}
		select {
		case <-deadline:
			t.Fatal("child did not start")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("child error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child did not stop")
	}
	if _, err := os.Stat("stopped"); err != nil {
		t.Fatalf("child did not acknowledge shutdown: %v", err)
	}
	if _, err := os.Stat("agent-sandbox.json"); !os.IsNotExist(err) {
		t.Fatalf("runtime remains: %v", err)
	}
}
