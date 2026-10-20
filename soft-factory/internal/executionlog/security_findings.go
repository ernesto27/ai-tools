package executionlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

type securityFinding struct {
	Priority     string `json:"priority"`
	Location     string `json:"location"`
	Trigger      string `json:"trigger"`
	Impact       string `json:"impact"`
	SuggestedFix string `json:"suggestedFix"`
}

type securityReport struct {
	Verdict  string            `json:"verdict"`
	Findings []securityFinding `json:"findings"`
	Checks   string            `json:"checks"`
	Gaps     string            `json:"gaps"`
}

// BeginSecurityFindings also marks attempts that fail before agent launch.
func (r *Run) BeginSecurityFindings() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.securityReporting = true
	// A new attempt must not leave an older report available as its latest
	// successful evidence if this attempt produces no readable report.
	if err := os.Remove(filepath.Join(r.path, "securityReview.txt")); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: clear previous security evidence: %v\n", err)
	}
}

// SecurityFindings registers a unique final report and retains an attempt archive
// before any later stage can remove worktree evidence. Failed summary writes
// retain this archive; if archiving fails, retain the worktree report instead.
func (r *Run) SecurityFindings(filename, source string, locateErr error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.securityReporting = true
	r.securityFindingsPath = source
	r.securityReportName = filename
	r.retainSecurityReport = true
	data, err := readSecurityFile(source)
	if locateErr != nil {
		err = locateErr
	}
	if err == nil {
		destination := filepath.Join(r.path, filename)
		err = os.WriteFile(destination, data, 0600)
		if err == nil {
			r.securityFindingsPath = destination
			r.retainSecurityReport = false
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: collect security findings: %v\n", err)
	}
}

// RetainReviewReport protects evidence when both archiving and summary saving fail.
func (r *Run) RetainReviewReport(filename string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return filename == r.securityReportName && r.retainSecurityReport
}

func readSecurityFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("security report must be a regular file")
	}
	return os.ReadFile(path)
}

func securitySummary(path string) (string, error) {
	data, err := readSecurityFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("security report is not UTF-8")
	}
	var report securityReport
	if err := json.Unmarshal(data, &report); err != nil {
		return "", fmt.Errorf("invalid security report JSON: %w", err)
	}
	validText := func(value string) bool {
		if strings.TrimSpace(value) == "" {
			return false
		}
		for _, c := range value {
			if unicode.IsControl(c) && c != '\n' && c != '\r' && c != '\t' {
				return false
			}
		}
		return true
	}
	if (report.Verdict != "PASS" && report.Verdict != "UNRESOLVED" && report.Verdict != "BLOCKED") ||
		report.Findings == nil || !validText(report.Checks) || !validText(report.Gaps) ||
		(report.Verdict == "PASS" && len(report.Findings) != 0) ||
		(report.Verdict == "UNRESOLVED" && len(report.Findings) == 0) {
		return "", fmt.Errorf("incomplete or inconsistent security report")
	}
	field := func(label, value string) string {
		value = strings.ReplaceAll(value, "\r\n", "\n")
		value = strings.ReplaceAll(value, "\r", "\n")
		return "   " + label + ": " + strings.ReplaceAll(value, "\n", "\n      ") + "\n"
	}
	text := field("Verdict", report.Verdict)
	if len(report.Findings) == 0 {
		if report.Verdict == "PASS" {
			text += field("Findings", "No actionable findings.")
		} else {
			text += field("Findings", "No actionable findings established; review evidence is insufficient.")
		}
	} else {
		text += "   Findings:\n"
		for _, f := range report.Findings {
			if (f.Priority != "P0" && f.Priority != "P1" && f.Priority != "P2" && f.Priority != "P3") ||
				!validText(f.Trigger) || !validText(f.Impact) || !validText(f.SuggestedFix) ||
				(f.Location != "" && !validText(f.Location)) {
				return "", fmt.Errorf("incomplete security finding")
			}
			location := f.Location
			if strings.TrimSpace(location) == "" {
				location = "Location unavailable"
			}
			text += field(f.Priority, location) + field("Trigger", f.Trigger) + field("Impact", f.Impact) + field("Suggested fix", f.SuggestedFix)
		}
	}
	return text + field("Checks", report.Checks) + field("Gaps", report.Gaps), nil
}

func (r *Run) securitySummaryLocked() string {
	text, err := securitySummary(r.securityFindingsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: read security findings: %v\n", err)
		return "   Verdict: BLOCKED\n   Findings: Security findings unavailable.\n   Checks: Unavailable.\n   Gaps: Final security report missing, unreadable, or invalid.\n"
	}
	return text
}
