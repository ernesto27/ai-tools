package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveReviewReportPreservesAgentContent(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "agent-report.md")
	destination := filepath.Join(dir, "05-review-changes.md")
	content := "# Changes\n\n### File: `./a.go`\nThe branch has no changed files.\n" + strings.Repeat("extra output\n", 200)
	if err := os.WriteFile(source, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := archiveReviewReport(source, destination); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Fatalf("archived report content changed")
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("archived report permissions = %04o, want 0600", info.Mode().Perm())
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("archive removed source before caller cleanup: %v", err)
	}
}

func TestArchiveReviewReportRejectsInvalidSource(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "05-review-changes.md")
	for _, source := range []string{filepath.Join(dir, "missing.md"), filepath.Join(dir, "empty.md"), filepath.Join(dir, "link.md")} {
		switch filepath.Base(source) {
		case "empty.md":
			if err := os.WriteFile(source, nil, 0600); err != nil {
				t.Fatal(err)
			}
		case "link.md":
			if err := os.Symlink("empty.md", source); err != nil {
				t.Fatal(err)
			}
		}
		if err := archiveReviewReport(source, destination); err == nil {
			t.Errorf("expected invalid source %q to fail", source)
		}
		if _, err := os.Stat(destination); !os.IsNotExist(err) {
			t.Errorf("archive created destination for invalid source %q: %v", source, err)
		}
	}
}
