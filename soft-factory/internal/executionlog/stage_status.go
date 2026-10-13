package executionlog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"soft-factory/internal/config"
)

// StageStatusFilename is the progress file stored in each run log folder.
const StageStatusFilename = "stage_status.txt"

// add stage status
// get last stage ,  type and status

type StageStatus struct {
	file *os.File
}

// NewStageStatus opens progress, creating the file only when create is true.
func NewStageStatus(path string, create bool) (*StageStatus, error) {
	flags := os.O_APPEND | os.O_RDWR
	if create {
		flags |= os.O_CREATE
	}
	file, err := os.OpenFile(filepath.Join(path, StageStatusFilename), flags, 0600)
	if err != nil {
		return nil, fmt.Errorf("open stage status file: %w", err)
	}
	return &StageStatus{file: file}, nil
}

// Close releases the status file.
func (s *StageStatus) Close() error { return s.file.Close() }

func (s *StageStatus) append(stage config.Stage, status string) error {
	_, err := fmt.Fprintf(s.file, "%s,%s\n", stage, status)
	return err
}

// Last returns the stage and status from the last entry, or io.EOF if empty.
func (s *StageStatus) Last() (stage config.Stage, status string, err error) {
	data, err := os.ReadFile(s.file.Name())
	if err != nil {
		return "", "", fmt.Errorf("read stage status: %w", err)
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return "", "", io.EOF
	}
	line := text[strings.LastIndexByte(text, '\n')+1:]
	stageName, status, ok := strings.Cut(line, ",")
	if !ok || stageName == "" || status == "" || strings.Contains(status, ",") {
		return "", "", fmt.Errorf("invalid stage status line %q", line)
	}
	return config.Stage(stageName), status, nil
}

// RunPath validates the supplied run log directory and returns its absolute path.
func RunPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("run log directory is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve run log directory: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("inspect run log directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("run log path %q is not a directory", path)
	}
	return absolute, nil
}
