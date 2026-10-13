package executionlog

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"soft-factory/internal/config"
)

func TestStageStatusAppendAndReopen(t *testing.T) {
	path := t.TempDir()
	saved, err := NewStageStatus(path, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { saved.Close() })
	if err := saved.append(config.StageImplementation, StatusSucceeded); err != nil {
		t.Fatal(err)
	}
	if err := saved.Close(); err != nil {
		t.Fatal(err)
	}
	saved, err = NewStageStatus(path, false)
	if err != nil {
		t.Fatal(err)
	}
	stage, status, err := saved.Last()
	if err != nil || stage != config.StageImplementation || status != StatusSucceeded {
		t.Fatalf("Last = %q, %q, %v", stage, status, err)
	}
	if err := saved.append(config.StageCodeReview, statusFailed); err != nil {
		t.Fatal(err)
	}
	stage, status, err = saved.Last()
	if err != nil || stage != config.StageCodeReview || status != statusFailed {
		t.Fatalf("Last after append = %q, %q, %v", stage, status, err)
	}
	data, err := os.ReadFile(filepath.Join(path, StageStatusFilename))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "implementation,SUCCEEDED\ncodeReview,FAILED\n" {
		t.Fatalf("status contents = %q", data)
	}
}

func TestStageStatusLast(t *testing.T) {
	for _, tc := range []struct {
		name, content  string
		stage          config.Stage
		status         string
		empty, invalid bool
	}{
		{name: "empty", empty: true},
		{name: "blank", content: "\n \n", empty: true},
		{name: "last entry", content: "implementation,SUCCEEDED\ncodeReview,FAILED\n", stage: config.StageCodeReview, status: statusFailed},
		{name: "no trailing newline", content: "securityReview,INTERRUPTED", stage: config.StageSecurityReview, status: statusInterrupted},
		{name: "trailing blank lines", content: "codeReview,SUCCEEDED\n\n", stage: config.StageCodeReview, status: StatusSucceeded},
		{name: "missing comma", content: "codeReview", invalid: true},
		{name: "missing stage", content: ",FAILED", invalid: true},
		{name: "missing status", content: "codeReview,", invalid: true},
		{name: "extra field", content: "codeReview,FAILED,extra", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := t.TempDir()
			if err := os.WriteFile(filepath.Join(path, StageStatusFilename), []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}
			saved, err := NewStageStatus(path, false)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { saved.Close() })
			stage, status, err := saved.Last()
			if tc.empty {
				if !errors.Is(err, io.EOF) {
					t.Fatalf("Last error = %v; want EOF", err)
				}
				return
			}
			if tc.invalid {
				if err == nil || errors.Is(err, io.EOF) {
					t.Fatalf("Last error = %v; want malformed-entry error", err)
				}
				return
			}
			if err != nil || stage != tc.stage || status != tc.status {
				t.Fatalf("Last = %q, %q, %v; want %q, %q", stage, status, err, tc.stage, tc.status)
			}
		})
	}
}

func TestNewStageStatusDoesNotCreateWhenDisabled(t *testing.T) {
	path := t.TempDir()
	if saved, err := NewStageStatus(path, false); err == nil {
		saved.Close()
		t.Fatal("opening a missing status file succeeded")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("open error = %v; want not-exist error", err)
	}
	if _, err := os.Stat(filepath.Join(path, StageStatusFilename)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing status file was created: %v", err)
	}
}

func TestRunPath(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("run", 0700); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs("run")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"run", want} {
		got, err := RunPath(path)
		if err != nil || got != want {
			t.Fatalf("RunPath(%q) = %q, %v; want %q", path, got, err, want)
		}
	}
	if err := os.WriteFile("file", nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"", " ", "missing", "file"} {
		if _, err := RunPath(path); err == nil {
			t.Errorf("RunPath(%q) should fail", path)
		}
	}
}
