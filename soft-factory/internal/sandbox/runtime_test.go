package sandbox

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRuntimeRestore(t *testing.T) {
	for _, original := range []string{"", "{malformed legacy\n", `{"agent":"conflicting"}`} {
		t.Run(original, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if original != "" {
				if err := os.WriteFile(runtimeFilename, []byte(original), 0640); err != nil {
					t.Fatal(err)
				}
			}
			const data = `{"run":{"apiKey":"secret","model":null},"resume":{"push":false},"unknown":[]}`
			runtime, err := PrepareRuntime([]byte(data))
			if err != nil {
				t.Fatal(err)
			}
			actual, err := os.ReadFile(runtimeFilename)
			if err != nil || string(actual) != data {
				t.Fatalf("generated = %q, %v", actual, err)
			}
			info, err := os.Stat(runtimeFilename)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("mode = %v, %v", info, err)
			}
			if _, err := PrepareRuntime([]byte(`{}`)); err == nil || !strings.Contains(err.Error(), "workflow lock") {
				t.Fatalf("concurrent run = %v", err)
			}
			if err := runtime.Close(); err != nil {
				t.Fatal(err)
			}
			if err := runtime.Close(); err != nil {
				t.Fatal(err)
			}
			actual, err = os.ReadFile(runtimeFilename)
			if original == "" {
				if !os.IsNotExist(err) {
					t.Fatalf("generated file remains: %v", err)
				}
			} else {
				if err != nil || string(actual) != original {
					t.Fatalf("restored = %q, %v", actual, err)
				}
				info, err := os.Stat(runtimeFilename)
				if err != nil || info.Mode().Perm() != 0640 {
					t.Fatalf("original mode = %v, %v", info, err)
				}
			}
			if _, err := os.Lstat(runtimeDirectory); !os.IsNotExist(err) {
				t.Fatalf("lock remains: %v", err)
			}
		})
	}
}

func TestRuntimeRejectsNonRegularPaths(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		t.Run(map[bool]string{false: "directory", true: "symlink"}[symlink], func(t *testing.T) {
			t.Chdir(t.TempDir())
			if symlink {
				if err := os.WriteFile("original", []byte("untouched"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("original", runtimeFilename); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Mkdir(runtimeFilename, 0700); err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(runtimeFilename)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := PrepareRuntime([]byte(`{}`)); err == nil {
				t.Fatal("accepted nonregular path")
			}
			after, err := os.Lstat(runtimeFilename)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("original path changed: %v", err)
			}
			if _, err := os.Lstat(runtimeDirectory); !os.IsNotExist(err) {
				t.Fatalf("setup cleanup failed: %v", err)
			}
		})
	}
}

func TestRuntimePreservesUnexpectedReplacement(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(runtimeFilename, []byte("original"), 0640); err != nil {
		t.Fatal(err)
	}
	runtime, err := PrepareRuntime([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(runtimeFilename); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimeFilename, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Close(); err == nil || !strings.Contains(err.Error(), "replaced") {
		t.Fatalf("cleanup = %v", err)
	}
	for path, want := range map[string]string{runtimeFilename: "replacement", filepath.Join(runtimeDirectory, "original.json"): "original"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatalf("%s = %q, %v", path, data, err)
		}
	}
	if _, err := PrepareRuntime([]byte(`{}`)); err == nil {
		t.Fatal("ignored recovery artifacts")
	}
}

func TestRuntimeSetupFailureRestoresOriginal(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(runtimeFilename, []byte("original"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(runtimeDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	r := &Runtime{directory: runtimeDirectory, target: runtimeFilename, backup: filepath.Join(runtimeDirectory, "original.json")}
	// Force failure after backup and before publication.
	if err := os.Mkdir(filepath.Join(runtimeDirectory, "generated.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := r.prepare([]byte(`{}`)); err == nil {
		t.Fatal("expected setup failure")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(runtimeFilename)
	if err != nil || string(data) != "original" {
		t.Fatalf("original = %q, %v", data, err)
	}
}

func TestRuntimeInterruptedHelper(t *testing.T) {
	if os.Getenv("FACTORY_INTERRUPTED_HELPER") != "1" {
		return
	}
	if _, err := PrepareRuntime([]byte(`{"agent":"generated"}`)); err != nil {
		t.Fatal(err)
	}
	os.Exit(0) // Simulate termination without cleanup.
}

func TestRuntimeDetectsInterruptedLifecycle(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(runtimeFilename, []byte("original"), 0640); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestRuntimeInterruptedHelper$")
	cmd.Env = append(os.Environ(), "FACTORY_INTERRUPTED_HELPER=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helper: %v: %s", err, out)
	}
	if _, err := PrepareRuntime([]byte(`{}`)); err == nil || !strings.Contains(err.Error(), "recovery") {
		t.Fatalf("interrupted detection = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(runtimeDirectory, "original.json"))
	if err != nil || string(data) != "original" {
		t.Fatalf("backup lost: %q, %v", data, err)
	}
}

func TestRuntimeCancellationWaitsForChild(t *testing.T) {
	t.Chdir(t.TempDir())
	bin := t.TempDir()
	// The child acknowledges interruption before exiting. It checks that runtime
	// JSON remains available during its shutdown, before Factory restores it.
	script := "#!/bin/sh\ntrap 'test -f agent-sandbox.json && echo stopped > stopped; exit 0' INT\necho ready > ready\nwhile :; do :; done\n"
	if err := os.WriteFile(filepath.Join(bin, "agent-sandbox"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	runtime, err := PrepareRuntime([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
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
		t.Fatalf("child could not read configuration during shutdown: %v", err)
	}
	if err := runtime.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(runtimeFilename); !os.IsNotExist(err) {
		t.Fatalf("runtime remains: %v", err)
	}
}
