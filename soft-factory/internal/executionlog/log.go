// Package executionlog stores a readable, streamed history of a factory run.
package executionlog

import (
	"context"
	"encoding/json"
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

// Run owns one directory of log files. Writes are serialized and best effort: after creation,
// logging failures warn once and never become workflow errors.
type Run struct {
	mu       sync.Mutex
	file     *os.File
	path     string
	header   string
	err      error
	attempts map[string]int
	active   string
	stream   string
	newline  bool
	closed   bool
	ready    bool
}

// NewRun initializes a private run directory before any workflow work starts.
func NewRun() (*Run, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}
	if err := os.MkdirAll(logDirectory, 0700); err != nil {
		return nil, fmt.Errorf("create execution log directory: %w", err)
	}
	branch, branchNote := sandboxBranch()
	path, err := createRunDirectory(branch, time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("create execution log directory: %w", err)
	}
	header := fmt.Sprintf("EXECUTION\nRun: %s\nBranch: %s\nStarted: %s\nDirectory: %s\nPlanned stages: %s\n",
		filepath.Base(path), branch, timestamp(), workingDirectory, strings.Join(stageNames, ", "))
	if branchNote == "agent-sandbox.json: run.branch" {
		header += "Branch source: " + branchNote + "\n"
	} else {
		header += "Branch note: " + branchNote + "\n"
	}
	return &Run{path: path, header: header, attempts: make(map[string]int), newline: true, ready: true}, nil
}

// createRunDirectory reserves a unique name without replacing an existing run.
func createRunDirectory(branch string, started time.Time) (string, error) {
	name := sanitizeBranch(branch) + "-" + started.Format("2006-01-02_15-04-05.000000000")
	for suffix := 0; ; suffix++ {
		directory := name
		if suffix > 0 {
			directory = fmt.Sprintf("%s-%d", name, suffix+1)
		}
		path := filepath.Join(logDirectory, directory)
		err := os.Mkdir(path, 0700)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return path, err
	}
}

func sandboxBranch() (string, string) {
	data, err := os.ReadFile("agent-sandbox.json")
	if err != nil {
		return "no-branch", "agent-sandbox.json unavailable"
	}
	var settings struct {
		Run struct {
			Branch string `json:"branch"`
		} `json:"run"`
	}
	if json.Unmarshal(data, &settings) != nil || strings.TrimSpace(settings.Run.Branch) == "" {
		return "no-branch", "run.branch unavailable in agent-sandbox.json"
	}
	return settings.Run.Branch, "agent-sandbox.json: run.branch"
}

func sanitizeBranch(branch string) string {
	var name strings.Builder
	for _, char := range branch {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '.' || char == '_' || char == '-' {
			name.WriteRune(char)
		} else {
			name.WriteByte('-')
		}
	}
	return name.String()
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
	if index == 0 || r.active != "" || r.closed {
		r.failLocked(fmt.Errorf("cannot start execution log stage %q", name))
		return
	}
	r.attempts[name]++
	filename := fmt.Sprintf("%02d-%s.log", index, name)
	if r.attempts[name] > 1 {
		filename = fmt.Sprintf("%02d-%s-%d.log", index, name, r.attempts[name])
	}
	var file *os.File
	if r.err == nil {
		var err error
		file, err = os.OpenFile(filepath.Join(r.path, filename), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			r.failLocked(fmt.Errorf("create stage log %q: %w", filename, err))
		}
	}
	r.active, r.stream = name, ""
	r.file = file
	r.newline = true
	r.writeLocked(r.header)
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
	r.finishStageLocked(err)
}

func (r *Run) finishStageLocked(err error) {
	if r.active == "" {
		return
	}
	status := outcome(err)
	if err != nil {
		r.sectionLocked("--- ERROR ---\n" + err.Error() + "\n")
	}
	r.sectionLocked(fmt.Sprintf("--- RESULT ---\nStatus: %s\nFinished: %s\n", status, timestamp()))
	r.closeStageLocked()
}

// closeStageLocked releases the active stage file. The caller holds r.mu.
func (r *Run) closeStageLocked() {
	if r.active != "" && r.file != nil {
		if closeErr := r.file.Close(); closeErr != nil {
			r.failLocked(fmt.Errorf("close stage log %q: %w", r.file.Name(), closeErr))
		}
	}
	r.file = nil
	r.newline = true
	r.active, r.stream = "", ""
}

// Finish closes an active stage, if any, even after a run failure.
func (r *Run) Finish(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.finishStageLocked(err)
	r.closed = true
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
		r.failLocked(fmt.Errorf("write execution log: run is closed"))
		return
	}
	if r.file == nil {
		return
	}
	n, err := io.WriteString(r.file, text)
	if err == nil && n != len(text) {
		err = io.ErrShortWrite
	}
	if err != nil {
		r.failLocked(fmt.Errorf("write execution log %q: %w", r.file.Name(), err))
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
