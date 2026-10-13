package executionlog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"soft-factory/internal/config"
)

func TestResumeAppendsInSameFolder(t *testing.T) {
	prepareLogTestDirectory(t)
	run, err := NewRun("test1", CommandRun, []config.Stage{config.StageCodeReview})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { run.Finish(nil) })
	path := run.Path()
	run.StartStage(config.StageCodeReview)
	run.Message("Original output")
	failure := errors.New("interrupted")
	run.FinishStage(failure)
	if err := run.RecordStageResult(config.StageCodeReview, time.Second, failure); err != nil {
		t.Fatal(err)
	}
	run.Finish(failure)

	resumed, err := OpenRun(path, "test1", []config.Stage{config.StageCodeReview})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resumed.Finish(nil) })
	wantPath, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Path() != wantPath {
		t.Fatalf("resumed path = %q; want %q", resumed.Path(), wantPath)
	}
	resumed.StartStage(config.StageCodeReview)
	resumed.Message("Resumed output")
	resumed.FinishStage(nil)
	if err := resumed.RecordStageResult(config.StageCodeReview, time.Second, nil); err != nil {
		t.Fatal(err)
	}
	resumed.Finish(nil)

	for file, fragments := range map[string][]string{
		"02-codeReview.log": {"Original output", "Resumed output"},
		"summary.log":       {"1. CODE REVIEW", "2. CODE REVIEW"},
		StageStatusFilename: {"codeReview,FAILED\ncodeReview,SUCCEEDED\n"},
	} {
		data, err := os.ReadFile(filepath.Join(path, file))
		if err != nil {
			t.Fatal(err)
		}
		for _, fragment := range fragments {
			if !strings.Contains(string(data), fragment) {
				t.Errorf("%s missing %q", file, fragment)
			}
		}
	}
	folders, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 {
		t.Fatal("resume created another folder")
	}
}

func TestReviewArchiveUsesStageIdentifier(t *testing.T) {
	prepareLogTestDirectory(t)
	run, err := NewRun("test1", CommandRun, config.Stages)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { run.Finish(nil) })
	source := filepath.Join(t.TempDir(), "review.txt")
	const evidence = "PASS\nReview findings\n"
	if err := os.WriteFile(source, []byte(evidence), 0600); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []config.Stage{config.StageCodeReview, config.StageSecurityReview} {
		if err := run.ArchiveReview(stage, source); err != nil {
			t.Fatal(err)
		}
		archive := filepath.Join(run.Path(), string(stage)+".txt")
		data, err := os.ReadFile(archive)
		if err != nil || string(data) != evidence {
			t.Fatalf("archive %s = %q, %v", archive, data, err)
		}
		destination := filepath.Join(t.TempDir(), "restored.txt")
		if err := run.RestoreReview(run.Path(), stage, destination); err != nil {
			t.Fatal(err)
		}
		data, err = os.ReadFile(destination)
		if err != nil || string(data) != evidence {
			t.Fatalf("restored %s = %q, %v", stage, data, err)
		}
	}
}
