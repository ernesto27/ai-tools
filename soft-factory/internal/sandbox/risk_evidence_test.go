package sandbox

import (
	"strings"
	"testing"

	"soft-factory/internal/config"
)

func TestRiskReviewEvidence(t *testing.T) {
	for _, tc := range []struct {
		name     string
		reports  []string
		disabled []config.Stage
		want     []string
		reject   []string
	}{
		{
			name:    "both reviews",
			reports: []string{"code.txt", "security.txt"},
			want:    []string{`- "code.txt"`, `- "security.txt"`},
			reject:  []string{"No review reports", "disabled"},
		},
		{
			name:     "code review only",
			reports:  []string{"code.txt"},
			disabled: []config.Stage{config.StageSecurityReview},
			want:     []string{`- "code.txt"`, "disabled in software-factory.json and did not run:\nsecurityReview.", "verification gap"},
			reject:   []string{"No review reports", "codeReview"},
		},
		{
			name:     "security review only",
			reports:  []string{"security.txt"},
			disabled: []config.Stage{config.StageCodeReview},
			want:     []string{`- "security.txt"`, "did not run:\ncodeReview."},
			reject:   []string{"No review reports", "securityReview"},
		},
		{
			name:     "no reviews",
			disabled: []config.Stage{config.StageCodeReview, config.StageSecurityReview},
			want:     []string{"No review reports are available for this run.", "did not run:\ncodeReview, securityReview.", "from earlier runs"},
			reject:   []string{"- "},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := riskReviewEvidence(tc.reports, tc.disabled)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("evidence missing %q:\n%s", want, got)
				}
			}
			for _, reject := range tc.reject {
				if strings.Contains(got, reject) {
					t.Errorf("evidence contains %q:\n%s", reject, got)
				}
			}
		})
	}
}
