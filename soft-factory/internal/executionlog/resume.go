package executionlog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"soft-factory/internal/config"
)

// OpenRun reopens the supplied log directory without creating a new invocation.
func OpenRun(path, branch string, planned []config.Stage) (*Run, error) {
	path, err := RunPath(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(path, summaryFile))
	if err != nil {
		return nil, fmt.Errorf("read existing summary: %w", err)
	}
	entries := 0
	for _, line := range strings.Split(string(data), "\n") {
		number, _, ok := strings.Cut(line, ". ")
		if !ok {
			continue
		}
		if n, err := strconv.Atoi(number); err == nil && n > entries {
			entries = n
		}
	}
	if err := IgnoreStageReports(); err != nil {
		return nil, err
	}
	status, err := NewStageStatus(path, false)
	if err != nil {
		return nil, err
	}
	summary, err := os.OpenFile(filepath.Join(path, summaryFile), os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		status.Close()
		return nil, fmt.Errorf("open existing summary: %w", err)
	}
	if _, err := fmt.Fprintf(summary, "\n%s\nRESUMED: %s\n\n", separator, timestamp()); err != nil {
		summary.Close()
		status.Close()
		return nil, fmt.Errorf("append resume summary: %w", err)
	}
	return &Run{
		path:           path,
		header:         fmt.Sprintf("EXECUTION\nRun: %s\nBranch: %s\nResumed: %s\nPlanned stages: %s\n", filepath.Base(path), branch, timestamp(), stageList(planned)),
		summary:        summary,
		summaryEntries: entries,
		stageStatus:    status,
		attempts:       make(map[config.Stage]int),
		newline:        true,
		ready:          true,
		resuming:       true,
	}, nil
}

// ArchiveReview saves review evidence independently of sandbox report cleanup.
func (r *Run) ArchiveReview(stage config.Stage, source string) error {
	if stage != config.StageCodeReview && stage != config.StageSecurityReview {
		return fmt.Errorf("invalid review stage %q", stage)
	}
	info, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("inspect review evidence: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("review evidence must be a regular file")
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read review evidence: %w", err)
	}
	return os.WriteFile(filepath.Join(r.path, string(stage)+".txt"), data, 0600)
}

// RestoreReview copies earlier evidence from the run logs into the worktree.
func (r *Run) RestoreReview(path string, stage config.Stage, destination string) error {
	source := filepath.Join(path, string(stage)+".txt")
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("restore %s review evidence from %q: %w", stage, source, err)
	}
	return os.WriteFile(destination, data, 0600)
}
