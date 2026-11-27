package github

import "testing"

func TestFormatPRSummaryPreservesBackticks(t *testing.T) {
	const summary = "SUMMARY\n````\n"
	const want = "`````\nSUMMARY\n````\n`````\n"
	if got := formatPRSummary(summary); got != want {
		t.Fatalf("formatPRSummary = %q, want %q", got, want)
	}
}

func TestAppendPRBody(t *testing.T) {
	const critical = "## ⚠️ Critical files changed\n- `go.mod`\n"
	tests := []struct {
		name, body, summary, critical, want string
	}{
		{"summary only", "Body", "SUMMARY", "", "Body\n\n```\nSUMMARY\n```\n"},
		{"summary and critical", "Body", "SUMMARY", critical, "Body\n\n```\nSUMMARY\n```\n\n\n" + critical},
		{"critical only", "Body", "", critical, "Body\n\n" + critical},
		{"empty body", "", "", critical, critical},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := appendPRBody(tt.body, tt.summary, tt.critical); got != tt.want {
				t.Fatalf("appendPRBody = %q, want %q", got, tt.want)
			}
		})
	}
}
