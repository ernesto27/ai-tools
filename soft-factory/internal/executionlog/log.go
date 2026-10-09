// Package executionlog stores a readable, streamed history of a factory run.
package executionlog

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	logDirectory = "logs"
	separator    = "=================================================="
	summaryFile  = "summary.log"
)

var stageNames = []string{"implementation", "code-review", "security-review", "risk-classification"}

// Run owns one directory of log files. Writes are serialized and best effort: after creation,
// logging failures warn once and never become workflow errors.
type Run struct {
	mu                sync.Mutex
	file              *os.File
	summary           *os.File
	summaryEntries    int
	changeSummaryPath string
	path              string
	header            string
	err               error
	attempts          map[string]int
	active            string
	stream            string
	newline           bool
	closed            bool
	ready             bool
}

// NewRun initializes a private invocation directory under the branch's log folder.
// command identifies the invocation as "run" or "continue".
// planned lists, in order, the stage labels this run will execute; stages
// disabled for the run are omitted from the header.
func NewRun(branch, command string, planned []string) (*Run, error) {
	if strings.TrimSpace(branch) == "" {
		return nil, fmt.Errorf("execution log requires a branch")
	}
	if command != "run" && command != "continue" {
		return nil, fmt.Errorf("unsupported execution log command %q", command)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}
	if err := IgnoreStageReports(); err != nil {
		return nil, fmt.Errorf("ignore stage change reports: %w", err)
	}
	path, err := createRunDirectory(branch, command, time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("create execution log directory: %w", err)
	}
	plannedStages := strings.Join(planned, ", ")
	if plannedStages == "" {
		plannedStages = "none"
	}
	header := fmt.Sprintf("EXECUTION\nRun: %s\nBranch: %s\nStarted: %s\nDirectory: %s\nPlanned stages: %s\n",
		filepath.Base(path), branch, timestamp(), workingDirectory, plannedStages)

	// Create summary
	pathSummary := filepath.Join(path, summaryFile)
	file, err := os.Create(pathSummary)
	if err != nil {
		return nil, fmt.Errorf("create summary: %w", err)
	}

	_, writeErr := fmt.Fprintf(file, "SOFTWARE FACTORY — RUN SUMMARY\n%s\n\n", separator)
	if writeErr != nil {
		file.Close()
		return nil, fmt.Errorf("write summary header: %w", writeErr)
	}

	return &Run{
			path:     path,
			header:   header,
			summary:  file,
			attempts: make(map[string]int),
			newline:  true,
			ready:    true},
		nil
}

// IgnoreStageReports excludes temporary reports before sandbox publication checks.
func IgnoreStageReports() error {
	output, err := exec.Command("git", "rev-parse", "--git-path", "info/exclude").Output()
	if err != nil {
		return fmt.Errorf("locate Git exclude file: %w", err)
	}
	path := strings.TrimSpace(string(output))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create Git exclude directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open Git exclude file: %w", err)
	}
	_, writeErr := fmt.Fprintln(file, "\n.soft-factory-stage-changes-*.txt\n.soft-factory-review-changes-*.md")
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return fmt.Errorf("write Git exclude rule: %w", err)
	}
	return nil
}

// createRunDirectory reserves a unique child without replacing an earlier invocation.
func createRunDirectory(branch, command string, started time.Time) (string, error) {
	branchName := sanitizeBranch(branch)
	if branchName == "." || branchName == ".." {
		return "", fmt.Errorf("create branch log directory: invalid branch directory %q", branchName)
	}
	if err := os.MkdirAll(logDirectory, 0700); err != nil {
		return "", fmt.Errorf("create branch log directory: %w", err)
	}
	root, err := os.OpenRoot(logDirectory)
	if err != nil {
		return "", fmt.Errorf("open log directory: %w", err)
	}
	defer root.Close()
	if err := root.Mkdir(branchName, 0700); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("create branch log directory: %w", err)
		}
		info, err := root.Lstat(branchName)
		if err != nil {
			return "", fmt.Errorf("inspect branch log directory: %w", err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("create branch log directory: %q is not a directory", branchName)
		}
	}
	branchDirectory := filepath.Join(logDirectory, branchName)
	name := command + "-" + started.UTC().Format("2006-01-02_15-04-05.000000000")
	for suffix := 0; ; suffix++ {
		directory := name
		if suffix > 0 {
			directory = fmt.Sprintf("%s-%d", name, suffix+1)
		}
		path := filepath.Join(branchDirectory, directory)
		err := root.Mkdir(filepath.Join(branchName, directory), 0700)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return path, err
	}
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
	r.changeSummaryPath = ""
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
	if r.summary != nil {
		if _, writeErr := fmt.Fprintln(r.summary, separator); writeErr != nil {
			r.failLocked(fmt.Errorf("write summary footer: %w", writeErr))
		}
		if closeErr := r.summary.Close(); closeErr != nil {
			r.failLocked(fmt.Errorf("close summary: %w", closeErr))
		}
		r.summary = nil
	}
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

// StageChangesPrompt asks for a short report without changing logging-disabled runs.
func (r *Run) StageChangesPrompt(prompt string) (string, string) {
	if r == nil {
		return prompt, ""
	}
	filename := ".soft-factory-stage-changes-" + rand.Text() + ".txt"
	prompt += fmt.Sprintf(`

## Stage change summary

As your final action, write a short plain-text summary to %q in the
worktree root. Use at most two short lines, without headings or bullets.
Do not exceed two lines; do not write a full report in this file.
Describe only implementation changes made during this stage execution,
including corrections made by fixer subagents. Do not repeat earlier stages'
changes. If no code changes were made, write "No code changes made."
This temporary reporting file is explicitly allowed even for read-only
assessment stages; it is not an implementation change. Do not include it
or other factory report files in your change description. Preserve the
stage's normal final report and output; this file supplements them.
`, filename)
	return prompt, filename
}

// RegisterStageChanges locates the report after the sandbox creates its worktree.
// Collection still runs on interruption so partial change reports can be saved.
func (r *Run) RegisterStageChanges(ctx context.Context, branch, filename string) {
	path, err := StageReportPath(ctx, branch, filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: locate stage change summary: %v\n", err)
		return
	}
	r.SetChangeSummaryPath(path)
}

// StageReportPath resolves an agent report's host path, even after interruption.
func StageReportPath(ctx context.Context, branch, filename string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	collectionCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	worktree, err := WorktreeForBranch(collectionCtx, branch)
	if err != nil {
		return "", err
	}
	return filepath.Join(worktree, filename), nil
}

// WorktreeForBranch resolves a branch's host worktree for reports and Git evidence.
func WorktreeForBranch(ctx context.Context, branch string) (string, error) {
	if strings.TrimSpace(branch) == "" {
		return "", fmt.Errorf("worktree lookup requires a branch")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, "git", "-C", ".", "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("list Git worktrees: %w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("list Git worktrees: %w", err)
	}
	for _, record := range strings.Split(string(out), "\n\n") {
		var path, currentBranch string
		for _, line := range strings.Split(record, "\n") {
			if value, ok := strings.CutPrefix(line, "worktree "); ok {
				path = value
			}
			if value, ok := strings.CutPrefix(line, "branch refs/heads/"); ok {
				currentBranch = value
			}
		}
		if currentBranch == branch && path != "" {
			return path, nil
		}
	}
	return "", fmt.Errorf("sandbox worktree for branch %q not found in Git worktree list", branch)
}

// SetChangeSummaryPath registers the host path of the agent's stage report.
func (r *Run) SetChangeSummaryPath(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.changeSummaryPath = path
}

// RecordStageResult reads the stage report and appends a numbered execution result.
// The report is removed only after its text has been saved successfully.
func (r *Run) RecordStageResult(name string, elapsed time.Duration, stageErr error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed || r.summary == nil {
		return fmt.Errorf("summary file is closed or unavailable")
	}

	entry := fmt.Sprintf(
		"%d. %s\n   Status:    %s\n   Duration:  %s\n",
		r.summaryEntries+1,
		strings.ToUpper(strings.ReplaceAll(name, "-", " ")),
		outcome(stageErr), formatDuration(elapsed),
	)
	changes := "Change summary unavailable."
	readReport := false
	if r.changeSummaryPath != "" {
		info, err := os.Lstat(r.changeSummaryPath)
		if err == nil && !info.Mode().IsRegular() {
			err = fmt.Errorf("stage change summary must be a regular file")
		}
		if err == nil {
			var data []byte
			data, err = os.ReadFile(r.changeSummaryPath)
			if err == nil {
				if text := strings.TrimSpace(strings.ReplaceAll(string(data), "\r\n", "\n")); text != "" {
					changes = text
				}
				readReport = true
			}
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: read stage change summary: %v\n", err)
		}
	}
	entry += "   Changes:   " + strings.ReplaceAll(changes, "\n", "\n              ") + "\n"
	if stageErr != nil {
		message := strings.ReplaceAll(stageErr.Error(), "\r\n", "\n")
		entry += "   Error:     " + strings.ReplaceAll(message, "\n", "\n              ") + "\n"
	}
	if _, err := io.WriteString(r.summary, entry+"\n"); err != nil {
		return fmt.Errorf("write stage summary: %w", err)
	}
	r.summaryEntries++
	if readReport {
		if err := os.Remove(r.changeSummaryPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: remove stage change summary: %v\n", err)
		}
		r.changeSummaryPath = ""
	}
	return nil
}

// RecordSkippedStage notes a disabled stage in the summary without numbering
// it as an executed result or creating a stage file.
func (r *Run) RecordSkippedStage(message string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed || r.summary == nil {
		return fmt.Errorf("summary file is closed or unavailable")
	}
	if _, err := io.WriteString(r.summary, message+"\n\n"); err != nil {
		return fmt.Errorf("write skipped stage: %w", err)
	}
	return nil
}

// formatDuration rounds elapsed time to whole seconds and omits zero units.
func formatDuration(elapsed time.Duration) string {
	totalSeconds := int64(elapsed.Round(time.Second) / time.Second)
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	var parts []string
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d hr", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%d min", minutes))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d sec", seconds))
	}
	return strings.Join(parts, " ")
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
