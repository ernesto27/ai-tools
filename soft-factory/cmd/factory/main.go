package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
	"soft-factory/internal/jira"
	"soft-factory/internal/sandbox"
	"soft-factory/internal/taskcontext"
)

// version is set to the release tag when building a release binary.
var version = "dev"

const factoryConfigFile = "software-factory.json"

func main() {
	if err := newRootCmd(runWorkflow).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// workflowOptions holds the parsed command-line inputs for one workflow run.
type workflowOptions struct {
	IssueURL   string
	Continue   bool
	ResumePath string
	StartStage config.Stage
	Branch     string
}

func runWorkflow(opts workflowOptions) (runErr error) {
	if err := loadEnvironment(); err != nil {
		return err
	}

	cfg, err := config.Load(factoryConfigFile)
	if err != nil {
		return err
	}

	shouldRun := func(stage config.Stage) bool {
		return opts.StartStage == "" || slices.Index(config.Stages, stage) >= slices.Index(config.Stages, opts.StartStage)
	}

	var loadedSkills sandbox.ProjectSkills

	entries := []struct {
		stage  config.Stage
		name   string
		target **sandbox.ProjectSkill
	}{
		{stage: config.StageCodeReview, name: cfg.Stages.CodeReview.CustomSkills, target: &loadedSkills.CodeReview},
		{stage: config.StageSecurityReview, name: cfg.Stages.SecurityReview.CustomSkills, target: &loadedSkills.SecurityReview},
		{stage: config.StageRiskClassification, name: cfg.Stages.RiskClassification.CustomSkills, target: &loadedSkills.RiskClassification},
		{stage: config.StageReviewChanges, name: cfg.Stages.ReviewChanges.CustomSkills, target: &loadedSkills.ReviewChanges},
	}

	for _, entry := range entries {
		// Disabled stages never read their skill files.
		if entry.name == "" || cfg.StageDisabled(entry.stage) || !shouldRun(entry.stage) {
			continue
		}

		skill, err := sandbox.LoadProjectSkill(
			filepath.Dir(factoryConfigFile),
			entry.name,
		)
		if err != nil {
			return fmt.Errorf("stages.%s.customSkills: %w", entry.stage, err)
		}

		*entry.target = skill
	}

	if err := installAgentSandbox(context.Background()); err != nil {
		return err
	}

	branch, branchSet := workflowBranch(cfg, opts)
	var runLog *executionlog.Run
	command := executionlog.CommandRun
	if opts.Continue {
		command = executionlog.CommandContinue
	}
	planned := plannedStages(cfg)
	if opts.ResumePath != "" {
		runLog, err = executionlog.OpenRun(opts.ResumePath, branch, planned)
	} else {
		runLog, err = executionlog.NewRun(branch, command, planned)
	}
	if err != nil {
		if opts.ResumePath != "" {
			return fmt.Errorf("open recovery logs: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Warning: execution logging unavailable: %v; continuing execution.\n", err)
	} else {
		fmt.Printf("Execution logs: %s\n", runLog.Path())
		defer func() { runLog.Finish(runErr) }()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var taskOverride string
	if opts.IssueURL != "" {
		taskOverride, err = loadJiraTask(ctx, opts.IssueURL)
		if err != nil {
			return err
		}
	}

	documents, err := taskcontext.Build(ctx, cfg, filepath.Dir(factoryConfigFile))
	if err != nil {
		return err
	}
	input := sandbox.TaskContext{
		TaskOverride: taskOverride,
		Documents:    documents,
		CustomSkills: loadedSkills,
		Context:      ctx,
		Log:          runLog,
		Branch:       branch,
		BranchSet:    branchSet,
		Continue:     opts.Continue,
		Config:       cfg,
	}

	if shouldRun(config.StageImplementation) {
		if err := runStage(input, cfg, config.StageImplementation, "Starting implementation...", func() error {
			return sandbox.Run(input)
		}); err != nil {
			return err
		}
	}

	// Reviews may come from this attempt or an earlier successful stage.
	var reports []string
	var disabledReviews []config.Stage
	reviews := []struct {
		stage   config.Stage
		message string
		review  func(sandbox.TaskContext) (string, error)
	}{
		{config.StageCodeReview, "\nStarting code review and corrections...", sandbox.Review},
		{config.StageSecurityReview, "\nStarting security review and corrections...", sandbox.SecurityReview},
	}
	for _, review := range reviews {
		if review.stage == config.StageSecurityReview && cfg.Stages.SecurityReview.SaveFindings {
			review.message = "\nStarting security review (save findings; no corrections)..."
		}
		if cfg.StageDisabled(review.stage) {
			disabledReviews = append(disabledReviews, review.stage)
		}
		if !shouldRun(review.stage) {
			if !cfg.StageDisabled(review.stage) && shouldRun(config.StageRiskClassification) && !cfg.StageDisabled(config.StageRiskClassification) {
				filename := ".soft-factory-stage-changes-" + rand.Text() + "-review.txt"
				worktree, err := executionlog.WorktreeForBranch(ctx, branch)
				if err != nil {
					return err
				}
				if err := runLog.RestoreReview(opts.ResumePath, review.stage, filepath.Join(worktree, filename)); err != nil {
					return fmt.Errorf("restore %s evidence: %w", review.stage, err)
				}
				reports = append(reports, filename)
			}
			continue
		}
		if err := runStage(input, cfg, review.stage, review.message, func() error {
			report, err := review.review(input)
			reports = append(reports, report)
			return err
		}); err != nil {
			return err
		}
	}

	if shouldRun(config.StageRiskClassification) {
		if err := runStage(input, cfg, config.StageRiskClassification, "\nStarting risk classification...", func() error {
			return sandbox.RiskClassification(input, reports, disabledReviews)
		}); err != nil {
			return err
		}
	}
	if cfg.StageDisabled(config.StageRiskClassification) {
		// Risk classification normally removes this run's review reports.
		sandbox.RemoveReviewReports(input, reports)
	}
	if !shouldRun(config.StageReviewChanges) {
		return nil
	}
	if err := input.Context.Err(); err != nil {
		return err
	}
	if cfg.StageDisabled(config.StageReviewChanges) {
		skipStage(input, config.StageReviewChanges)
		return nil
	}
	if err := runStage(input, cfg, config.StageReviewChanges, "\nStarting final change walkthrough...", func() error {
		return sandbox.ReviewChanges(input)
	}); err != nil {
		if input.Context.Err() != nil || opts.ResumePath != "" {
			return err
		}
		fmt.Fprintf(os.Stderr, "Warning: final change walkthrough unavailable: %v; continuing execution.\n", err)
	}

	return nil
}

// workflowBranch selects one branch for every stage and report in this run.
func workflowBranch(cfg config.Config, opts workflowOptions) (string, bool) {
	if opts.Branch != "" {
		return opts.Branch, true
	}
	settings := cfg.AgentSandbox
	section := settings.Resume
	if !opts.Continue {
		section = settings.Run
	}
	if section != nil && section.Branch != nil {
		return *section.Branch, true
	}
	return settings.Branch, settings.Branch != ""
}

// plannedStages lists enabled stage identifiers in pipeline order.
func plannedStages(cfg config.Config) []config.Stage {
	var stages []config.Stage
	for _, stage := range config.Stages {
		if !cfg.StageDisabled(stage) {
			stages = append(stages, stage)
		}
	}
	return stages
}

// runStage executes an enabled stage; disabled stages skip execute.
func runStage(input sandbox.TaskContext, cfg config.Config, stage config.Stage, message string, execute func() error) error {
	if err := input.Context.Err(); err != nil {
		return err
	}
	if cfg.StageDisabled(stage) {
		skipStage(input, stage)
		return nil
	}
	if input.Log != nil {
		input.Log.StartStage(stage)
		input.Log.Message(message)
	}
	fmt.Println(message)

	started := time.Now()
	err := execute()
	elapsed := time.Since(started)

	if input.Context.Err() != nil {
		err = errors.Join(err, input.Context.Err())
	}
	if input.Log != nil {
		input.Log.FinishStage(err)

		if logErr := input.Log.RecordStageResult(stage, elapsed, err); logErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: save stage summary: %v\n", logErr)
		}
	}
	return err
}

// skipStage reports a disabled stage without recording it as executed.
func skipStage(input sandbox.TaskContext, stage config.Stage) {
	message := "Skipping disabled stage: " + string(stage)
	fmt.Println("\n" + message)
	if input.Log != nil {
		if err := input.Log.RecordSkippedStage(message); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: save skipped stage: %v\n", err)
		}
	}
}

func loadEnvironment() error {
	if err := godotenv.Load(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Dotenv syntax errors can quote secret values. Keep diagnostics generic.
		return fmt.Errorf("load .env: check the file's syntax and permissions")
	}
	return nil
}

func loadJiraTask(ctx context.Context, issueURL string) (string, error) {
	client, err := jira.New(os.Getenv("JIRA_EMAIL"), os.Getenv("JIRA_API_TOKEN"), nil)
	if err != nil {
		return "", err
	}
	issue, err := client.GetIssue(ctx, issueURL)
	if err != nil {
		return "", err
	}
	return issue.Task(), nil
}

func installAgentSandbox(ctx context.Context) error {
	_, err := exec.LookPath("agent-sandbox")
	if err != nil {
		fmt.Fprint(os.Stderr, "agent-sandbox is missing. Download and install it? [y/N]: ")
		input := bufio.NewScanner(os.Stdin)
		input.Scan()
		if err := input.Err(); err != nil {
			return fmt.Errorf("read installation confirmation: %w", err)
		}
		answer := strings.ToLower(strings.TrimSpace(input.Text()))
		if answer != "y" && answer != "yes" {
			return fmt.Errorf("agent-sandbox installation declined")
		}

		cmd := exec.CommandContext(ctx, "bash", "-o", "pipefail", "-c",
			"curl -fsSL https://raw.githubusercontent.com/ernesto27/ai-tools/master/agent-sandbox/install.sh | bash",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install agent-sandbox: %w", err)
		}
		return nil
	}

	return nil

}
