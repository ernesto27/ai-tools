// Package executionlog stores a readable, streamed history of a factory run.
package executionlog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	logDirectory = "logs"
	separator    = "=================================================="
)

var stageNames = []string{"implementation", "code-review", "security-review", "risk-classification"}

// Run owns one log file. Writes are serialized and best effort: after creation,
// logging failures warn once and never become workflow errors.
type Run struct {
	mu       sync.Mutex
	file     *os.File
	path     string
	err      error
	statuses map[string]string
	active   string
	stream   string
	newline  bool
	closed   bool
	ready    bool
}

// NewRun initializes a private log file before any workflow work starts.
func NewRun() (*Run, error) {
	if err := os.MkdirAll(logDirectory, 0700); err != nil {
		return nil, fmt.Errorf("create execution log directory: %w", err)
	}
	file, err := createLogFile(time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("create execution log: %w", err)
	}
	r := &Run{file: file, path: file.Name(), statuses: make(map[string]string), newline: true}
	for _, name := range stageNames {
		r.statuses[name] = "PENDING"
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("resolve working directory: %w", err), file.Close())
	}
	r.writeLocked(fmt.Sprintf("EXECUTION\nRun: %s\nStarted: %s\nDirectory: %s\nPlanned stages: %s\n",
		filepath.Base(r.path), timestamp(), workingDirectory, strings.Join(stageNames, ", ")))
	if r.err != nil {
		return nil, errors.Join(r.err, file.Close())
	}
	r.ready = true
	return r, nil
}

// createLogFile creates a permanent log without replacing an existing run.
func createLogFile(started time.Time) (*os.File, error) {
	name := started.Format("2006-01-02_15-04-05.000000000")
	for suffix := 0; ; suffix++ {
		filename := name + ".log"
		if suffix > 0 {
			filename = fmt.Sprintf("%s-%d.log", name, suffix)
		}
		path := filepath.Join(logDirectory, filename)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return file, err
	}
}

func (r *Run) Path() string { return r.path }

func (r *Run) Message(message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sectionLocked(message + "\n")
}

func (r *Run) StartStage(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	index := 0
	for i, stage := range stageNames {
		if stage == name {
			index = i + 1
		}
	}
	if index == 0 || r.active != "" || r.statuses[name] != "PENDING" {
		r.failLocked(fmt.Errorf("cannot start execution log stage %q", name))
		return
	}
	r.active, r.stream = name, ""
	r.statuses[name] = "RUNNING"
	r.sectionLocked(fmt.Sprintf("%s\nSTAGE %d: %s\n%s\nStarted: %s\n",
		separator, index, strings.ToUpper(name), separator, timestamp()))
}

// Details appends known metadata and its provenance before subprocess launch.
func (r *Run) Details(agent, model, provider string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sectionLocked(fmt.Sprintf("Agent: %s\nModel: %s\nProvider: %s\nInternal commands: saved when visible in output; structured capture unavailable\n",
		agent, model, provider))
}

func (r *Run) Prompt(prompt string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sectionLocked("--- PROMPT ---\n")
	r.writeLocked(prompt)
}

func (r *Run) Command(executable string, args []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	parts := []string{quote(executable)}
	for _, arg := range args {
		parts = append(parts, quote(arg))
	}
	r.sectionLocked("Command: " + strings.Join(parts, " ") + "\n")
}

func (r *Run) ExitCode(code int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sectionLocked(fmt.Sprintf("Exit code: %d\n", code))
}

func (r *Run) Report(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sectionLocked("Report: " + path + "\n")
}

func (r *Run) FinishStage(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	status := outcome(err)
	if err != nil {
		r.sectionLocked("--- ERROR ---\n" + err.Error() + "\n")
	}
	r.sectionLocked(fmt.Sprintf("--- RESULT ---\nStatus: %s\nFinished: %s\n", status, timestamp()))
	r.statuses[r.active] = status
	r.active, r.stream = "", ""
}

// Finish writes the final summary and closes the file, even after a run failure.
func (r *Run) Finish(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	if err != nil {
		r.sectionLocked("--- ERROR ---\n" + err.Error() + "\n")
	}
	r.sectionLocked(fmt.Sprintf("%s\nEXECUTION FINISHED\nStatus: %s\nFinished: %s\n",
		separator, outcome(err), timestamp()))
	for _, name := range stageNames {
		r.writeLocked(fmt.Sprintf("%s: %s\n", name, r.statuses[name]))
	}
	r.closed = true
	if err := r.file.Close(); err != nil {
		r.failLocked(fmt.Errorf("close execution log %q: %w", r.path, err))
	}
}

// Stream preserves output chunks without scanner limits or whole-run buffering.
// A persistence failure warns once. Continue draining output so the child
// cannot block on a full pipe and terminal output survives.
func (r *Run) Stream(name string) io.Writer { return streamWriter{run: r, name: name} }

type streamWriter struct {
	run  *Run
	name string
}

func (w streamWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	r := w.run
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stream != w.name {
		heading := "--- OUTPUT ---\n"
		if w.name == "stderr" {
			heading = "--- OUTPUT (stderr) ---\n"
		} else if w.name == "combined" {
			heading = "--- OUTPUT (stdout + stderr) ---\n"
		}
		r.sectionLocked(heading)
		r.stream = w.name
	}
	r.writeLocked(string(p))
	return len(p), nil
}

func (r *Run) sectionLocked(text string) {
	if !r.newline {
		r.writeLocked("\n")
	}
	r.writeLocked("\n" + text)
	r.stream = ""
}

func (r *Run) writeLocked(text string) {
	if r.err != nil || text == "" {
		return
	}
	if r.closed {
		r.failLocked(fmt.Errorf("write execution log %q: file is closed", r.path))
		return
	}
	n, err := io.WriteString(r.file, text)
	if err == nil && n != len(text) {
		err = io.ErrShortWrite
	}
	if err != nil {
		r.failLocked(fmt.Errorf("write execution log %q: %w", r.path, err))
	}
	r.newline = strings.HasSuffix(text, "\n")
}

func (r *Run) failLocked(err error) {
	if r.err != nil {
		return
	}
	r.err = err
	if r.ready {
		fmt.Fprintf(os.Stderr, "Warning: execution logging unavailable: %v; continuing execution.\n", err)
	}
}

func timestamp() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func outcome(err error) string {
	if errors.Is(err, context.Canceled) {
		return "INTERRUPTED"
	}
	if err != nil {
		return "FAILED"
	}
	return "SUCCEEDED"
}

func quote(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\r\n'\"\\$`;&|<>()*?[]{}!#~") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
