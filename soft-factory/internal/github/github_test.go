package github

import "testing"

func TestFormatPRSummaryPreservesBackticks(t *testing.T) {
	const summary = "SUMMARY\n````\n"
	const want = "`````\nSUMMARY\n````\n`````\n"
	if got := formatPRSummary(summary); got != want {
		t.Fatalf("formatPRSummary = %q, want %q", got, want)
	}
}
