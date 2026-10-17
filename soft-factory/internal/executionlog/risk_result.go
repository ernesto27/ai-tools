package executionlog

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"soft-factory/internal/prompts"
)

// RiskResultPrompt requests a separate result only for logged workflows.
func (r *Run) RiskResultPrompt(prompt string) (string, string, error) {
	if r == nil {
		return prompt, "", nil
	}
	filename := ".soft-factory-risk-result-" + rand.Text() + ".json"
	instructions, err := prompts.Render("risk-result.txt", map[string]any{
		"ReportPath": ContainerReportPath(filename),
	})
	if err != nil {
		return "", "", err
	}
	return prompt + instructions, filename, nil
}

// RegisterRiskResult collects this attempt's file, including after interruption.
func (r *Run) RegisterRiskResult(ctx context.Context, branch, filename string) {
	path, err := StageReportPath(ctx, branch, filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: locate risk classification result: %v\n", err)
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.riskResultPath = path
}

// The boolean indicates that a regular file was read and can be cleaned up
// after saving the summary, even when its contents were invalid.
func readRiskResult(path string) (string, bool) {
	const unavailable = "Classification unavailable."
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return unavailable, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return unavailable, false
	}
	var result map[string]json.RawMessage
	if !utf8.Valid(data) || json.Unmarshal(data, &result) != nil || result == nil {
		return unavailable, true
	}
	var label string
	if json.Unmarshal(result["classification"], &label) != nil {
		return unavailable, true
	}
	for _, char := range label {
		if unicode.IsControl(char) || char == '\u2028' || char == '\u2029' {
			return unavailable, true
		}
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return unavailable, true
	}
	return label, true
}

func (r *Run) riskResultLocked() (string, func()) {
	classification, collected := readRiskResult(r.riskResultPath)
	line := "   Classification: " + classification + "\n"
	if !collected {
		return line, nil
	}
	return line, r.removeRiskResultLocked
}

func (r *Run) removeRiskResultLocked() {
	if err := os.Remove(r.riskResultPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: remove risk classification result: %v\n", err)
	}
	r.riskResultPath = ""
}
