package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeSandbox records each invocation as "stage mode branch" and keeps the
// prompt it received. It fails the stage named by FACTORY_TEST_FAIL.
const fakeSandbox = `#!/bin/sh
prompt=
branch=
previous=
for arg do
	[ "$previous" = -f ] && prompt=$arg
	[ "$previous" = --branch ] && branch=$arg
	previous=$arg
done
stage=implementation
if [ -n "$prompt" ]; then
	if grep -q 'Perform a risk classification' "$prompt"; then stage=riskClassification
	elif grep -q 'read-only walkthrough' "$prompt"; then stage=reviewChanges
	elif grep -q 'name: security-review' "$prompt"; then stage=securityReview
	elif grep -q 'name: code-review' "$prompt"; then stage=codeReview
	fi
	cp "$prompt" "$FACTORY_TEST_RECORD/$stage.prompt"
fi
if [ "$stage" = reviewChanges ]; then
	walkthrough=$(grep -o '\.soft-factory-review-changes-[0-9]*\.md' "$prompt" | head -n 1)
	echo "# Walkthrough" > "$walkthrough"
fi
echo "$stage $1 $branch" >> "$FACTORY_TEST_RECORD/calls"
[ "$stage" = "$FACTORY_TEST_FAIL" ] && exit 1
exit 0
`

type workflowFixture struct {
	record string
	stdout string
}

// newWorkflowFixture creates a Git worktree on branch "task" with local
// configuration and puts the agent-sandbox stub first on PATH.
func newWorkflowFixture(t *testing.T, factoryConfig, sandboxConfig string) workflowFixture {
	t.Helper()
	t.Chdir(t.TempDir())
	for _, args := range [][]string{
		{"init", "-b", "task"},
		{"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "initial"},
	} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	if sandboxConfig == "" {
		sandboxConfig = `{"run":{"branch":"task","file-prompt":"task.txt"},"resume":{"branch":"task"}}`
	}
	files := map[string]string{
		factoryConfigFile:    factoryConfig,
		"agent-sandbox.json": sandboxConfig,
		"task.txt":           "Implement the task.\n",
	}
	for name, content := range files {
		if err := os.WriteFile(name, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "agent-sandbox"), []byte(fakeSandbox), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	fixture := workflowFixture{record: t.TempDir()}
	t.Setenv("FACTORY_TEST_RECORD", fixture.record)
	t.Setenv("FACTORY_TEST_FAIL", "")

	// Capture workflow and stub output in a file to avoid pipe back-pressure.
	fixture.stdout = filepath.Join(t.TempDir(), "stdout")
	output, err := os.Create(fixture.stdout)
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = output
	t.Cleanup(func() {
		os.Stdout = previous
		output.Close()
	})
	return fixture
}

func (f workflowFixture) calls(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.record, "calls"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(strings.ReplaceAll(string(data), " ", "/"))
}

func (f workflowFixture) prompt(t *testing.T, stage string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.record, stage+".prompt"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func (f workflowFixture) output(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(f.stdout)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// runLogDirectory returns the only execution log directory, if any.
func runLogDirectory(t *testing.T) string {
	t.Helper()
	entries, err := filepath.Glob(filepath.Join("logs", "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > 1 {
		t.Fatalf("expected at most one run directory, got %v", entries)
	}
	if len(entries) == 0 {
		return ""
	}
	return entries[0]
}

func disabledConfig(stages ...string) string {
	if len(stages) == 0 {
		return `{}`
	}
	return `{"disabledStages":["` + strings.Join(stages, `","`) + `"]}`
}

func TestWorkflowDisabledStages(t *testing.T) {
	const (
		impl     = "implementation/run/task"
		cont     = "implementation/resume/task"
		code     = "codeReview/resume/task"
		security = "securityReview/resume/task"
		risk     = "riskClassification/resume/task"
		walk     = "reviewChanges/resume/task"
	)
	defaultRun := workflowOptions{Implement: true}
	reviewRun := workflowOptions{}
	continueRun := workflowOptions{Implement: true, Continue: true}
	for _, tc := range []struct {
		name     string
		opts     workflowOptions
		disabled []string
		want     []string
	}{
		{"default/none", defaultRun, nil, []string{impl, code, security, risk, walk}},
		{"default/empty list", defaultRun, []string{}, []string{impl, code, security, risk, walk}},
		{"default/implementation", defaultRun, []string{"implementation"}, []string{code, security, risk, walk}},
		{"default/codeReview", defaultRun, []string{"codeReview"}, []string{impl, security, risk, walk}},
		{"default/securityReview", defaultRun, []string{"securityReview"}, []string{impl, code, risk, walk}},
		{"default/riskClassification", defaultRun, []string{"riskClassification"}, []string{impl, code, security, walk}},
		{"default/reviewChanges", defaultRun, []string{"reviewChanges"}, []string{impl, code, security, risk}},
		{"default/multiple", defaultRun, []string{"securityReview", "riskClassification"}, []string{impl, code, walk}},
		{"default/both reviews", defaultRun, []string{"codeReview", "securityReview"}, []string{impl, risk, walk}},
		{"default/duplicates", defaultRun, []string{"codeReview", "codeReview"}, []string{impl, security, risk, walk}},
		{"default/all", defaultRun, []string{"implementation", "codeReview", "securityReview", "riskClassification", "reviewChanges"}, nil},
		{"review/none", reviewRun, nil, []string{code, security, risk}},
		{"review/inapplicable stages", reviewRun, []string{"implementation", "reviewChanges"}, []string{code, security, risk}},
		{"review/codeReview", reviewRun, []string{"codeReview"}, []string{security, risk}},
		{"review/securityReview", reviewRun, []string{"securityReview"}, []string{code, risk}},
		{"review/riskClassification", reviewRun, []string{"riskClassification"}, []string{code, security}},
		{"review/all applicable", reviewRun, []string{"codeReview", "securityReview", "riskClassification"}, nil},
		{"continue/none", continueRun, nil, []string{cont, code, security, risk, walk}},
		{"continue/implementation", continueRun, []string{"implementation"}, []string{code, security, risk, walk}},
		{"continue/reviewChanges", continueRun, []string{"reviewChanges"}, []string{cont, code, security, risk}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := disabledConfig(tc.disabled...)
			if tc.disabled != nil && len(tc.disabled) == 0 {
				config = `{"disabledStages":[]}`
			}
			fixture := newWorkflowFixture(t, config, "")
			if err := runWorkflow(tc.opts); err != nil {
				t.Fatalf("runWorkflow: %v", err)
			}
			if got := fixture.calls(t); !slices.Equal(got, tc.want) {
				t.Fatalf("agent-sandbox calls = %v; want %v", got, tc.want)
			}

			output := fixture.output(t)
			for _, stage := range []string{"implementation", "codeReview", "securityReview", "riskClassification", "reviewChanges"} {
				applicable := tc.opts.Implement || (stage != "implementation" && stage != "reviewChanges")
				skipped := applicable && slices.Contains(tc.disabled, stage)
				message := "Skipping disabled stage: " + stage
				if got := strings.Count(output, message); got != map[bool]int{true: 1, false: 0}[skipped] {
					t.Errorf("output contains %q %d times; want skipped=%v\n%s", message, got, skipped, output)
				}
				if skipped {
					if _, err := os.Stat(filepath.Join(fixture.record, stage+".prompt")); !os.IsNotExist(err) {
						t.Errorf("disabled stage %s generated a prompt: %v", stage, err)
					}
				}
			}

			logDir := runLogDirectory(t)
			if !tc.opts.Implement {
				if logDir != "" {
					t.Fatalf("review command created execution logs in %s", logDir)
				}
				return
			}
			if logDir == "" {
				t.Fatal("default workflow did not create execution logs")
			}
			summary, err := os.ReadFile(filepath.Join(logDir, "summary.log"))
			if err != nil {
				t.Fatal(err)
			}
			for _, stage := range []struct{ id, file, entry string }{
				{"implementation", "01-implementation.log", "IMPLEMENTATION"},
				{"codeReview", "02-code-review.log", "CODE REVIEW"},
				{"securityReview", "03-security-review.log", "SECURITY REVIEW"},
				{"riskClassification", "04-risk-classification.log", "RISK CLASSIFICATION"},
				{"reviewChanges", "05-review-changes.md", ""},
			} {
				disabled := slices.Contains(tc.disabled, stage.id)
				_, statErr := os.Stat(filepath.Join(logDir, stage.file))
				if disabled != os.IsNotExist(statErr) {
					t.Errorf("%s exists = %v; want %v", stage.file, statErr == nil, !disabled)
				}
				if stage.entry != "" && disabled == strings.Contains(string(summary), ". "+stage.entry+"\n") {
					t.Errorf("summary entry for %s present = %v; want %v\n%s", stage.id, !disabled, !disabled, summary)
				}
				if disabled != strings.Contains(string(summary), "Skipping disabled stage: "+stage.id+"\n") {
					t.Errorf("summary skip note for %s present = %v; want %v\n%s", stage.id, !disabled, disabled, summary)
				}
			}
		})
	}
}

func TestWorkflowRiskUsesOnlyEnabledReviewReports(t *testing.T) {
	for _, tc := range []struct {
		name     string
		disabled []string
		reports  int
		want     []string
		reject   []string
	}{
		{"two reports", nil, 2, nil, []string{"No review reports", "did not run"}},
		{"code review only", []string{"securityReview"}, 1, []string{"did not run:\nsecurityReview."}, []string{"No review reports"}},
		{"security review only", []string{"codeReview"}, 1, []string{"did not run:\ncodeReview."}, []string{"No review reports"}},
		{"no reports", []string{"codeReview", "securityReview"}, 0, []string{"No review reports are available for this run.", "did not run:\ncodeReview, securityReview."}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newWorkflowFixture(t, disabledConfig(tc.disabled...), "")
			if err := runWorkflow(workflowOptions{}); err != nil {
				t.Fatalf("runWorkflow: %v", err)
			}
			prompt := fixture.prompt(t, "riskClassification")
			_, evidence, ok := strings.Cut(prompt, "## Review reports\n")
			if !ok {
				t.Fatalf("risk prompt has no review report section:\n%s", prompt)
			}
			if got := strings.Count(evidence, `- ".soft-factory-stage-changes-`); got != tc.reports {
				t.Errorf("risk prompt lists %d reports; want %d\n%s", got, tc.reports, evidence)
			}
			if strings.Contains(evidence, `- ""`) {
				t.Errorf("risk prompt lists a blank report:\n%s", evidence)
			}
			for _, want := range tc.want {
				if !strings.Contains(evidence, want) {
					t.Errorf("risk prompt missing %q:\n%s", want, evidence)
				}
			}
			for _, reject := range tc.reject {
				if strings.Contains(evidence, reject) {
					t.Errorf("risk prompt contains %q:\n%s", reject, evidence)
				}
			}
		})
	}
}

func TestWorkflowEnabledStageFailureStopsRun(t *testing.T) {
	for _, tc := range []struct {
		name     string
		opts     workflowOptions
		disabled []string
		fail     string
		want     []string
	}{
		{"default", workflowOptions{Implement: true}, []string{"implementation", "codeReview"}, "securityReview", []string{"securityReview/resume/task"}},
		{"review", workflowOptions{}, []string{"securityReview"}, "codeReview", []string{"codeReview/resume/task"}},
		{"implementation", workflowOptions{Implement: true}, []string{"codeReview"}, "implementation", []string{"implementation/run/task"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newWorkflowFixture(t, disabledConfig(tc.disabled...), "")
			t.Setenv("FACTORY_TEST_FAIL", tc.fail)
			if err := runWorkflow(tc.opts); err == nil || !strings.Contains(err.Error(), "execute agent-sandbox") {
				t.Fatalf("expected %s failure, got %v", tc.fail, err)
			}
			if got := fixture.calls(t); !slices.Equal(got, tc.want) {
				t.Fatalf("agent-sandbox calls = %v; want %v", got, tc.want)
			}
		})
	}
}

func TestWorkflowDisabledImplementationKeepsBranchSelection(t *testing.T) {
	// The default command still selects run.branch; continue selects resume.branch.
	sandboxConfig := `{"run":{"branch":"task","file-prompt":"task.txt"},"resume":{"branch":"other"}}`
	for _, tc := range []struct {
		name string
		opts workflowOptions
		want string
	}{
		{"default", workflowOptions{Implement: true}, "codeReview/resume/task"},
		{"continue", workflowOptions{Implement: true, Continue: true}, "codeReview/resume/other"},
		{"review", workflowOptions{}, "codeReview/resume/other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := disabledConfig("implementation", "securityReview", "riskClassification", "reviewChanges")
			fixture := newWorkflowFixture(t, config, sandboxConfig)
			if err := runWorkflow(tc.opts); err != nil {
				t.Fatalf("runWorkflow: %v", err)
			}
			if got := fixture.calls(t); !slices.Equal(got, []string{tc.want}) {
				t.Fatalf("agent-sandbox calls = %v; want [%s]", got, tc.want)
			}
		})
	}
}

func TestWorkflowIgnoresMissingSkillForDisabledStage(t *testing.T) {
	for _, stage := range []string{"codeReview", "securityReview", "riskClassification", "reviewChanges"} {
		t.Run(stage, func(t *testing.T) {
			config := `{"customSkills":{"` + stage + `":"missing"},"disabledStages":["` + stage + `"]}`
			fixture := newWorkflowFixture(t, config, "")
			if err := runWorkflow(workflowOptions{Implement: true}); err != nil {
				t.Fatalf("missing skill for disabled stage blocked the workflow: %v", err)
			}
			if slices.ContainsFunc(fixture.calls(t), func(call string) bool { return strings.HasPrefix(call, stage+"/") }) {
				t.Fatalf("disabled stage %s ran", stage)
			}
		})
	}
}

func TestWorkflowRejectsInvalidDisabledStagesBeforeAgents(t *testing.T) {
	for _, tc := range []struct{ config, want string }{
		{`{"disabledStages":["securityReveiw"]}`, `disabledStages[0]: unknown stage "securityReveiw"`},
		{`{"disabledStages":["security-review"]}`, `disabledStages[0]: unknown stage "security-review"`},
		{`{"disabledStages":null}`, "disabledStages must be an array"},
		{`{"disabledStages":[null]}`, "disabledStages[0] must be a string"},
	} {
		t.Run(tc.config, func(t *testing.T) {
			fixture := newWorkflowFixture(t, tc.config, "")
			err := runWorkflow(workflowOptions{Implement: true})
			if err == nil || !strings.Contains(err.Error(), "validate factory configuration: "+tc.want) {
				t.Fatalf("expected validation error %q, got %v", tc.want, err)
			}
			if calls := fixture.calls(t); len(calls) != 0 {
				t.Fatalf("agent-sandbox ran despite invalid configuration: %v", calls)
			}
			if dir := runLogDirectory(t); dir != "" {
				t.Fatalf("invalid configuration created execution logs in %s", dir)
			}
		})
	}
}

func TestWorkflowKeepsExistingReportsForDisabledStages(t *testing.T) {
	newWorkflowFixture(t, disabledConfig("codeReview", "securityReview", "riskClassification"), "")
	if err := os.Mkdir("docs", 0700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join("docs", "risk-classification-old.md")
	if err := os.WriteFile(old, []byte("old report"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runWorkflow(workflowOptions{}); err != nil {
		t.Fatalf("runWorkflow: %v", err)
	}
	if data, err := os.ReadFile(old); err != nil || string(data) != "old report" {
		t.Fatalf("existing report changed: %q, %v", data, err)
	}
}
