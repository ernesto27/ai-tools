package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
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
	IssueURL string
	// Implement runs the implementation stage before the reviews.
	Implement bool
	Continue  bool
}

func runWorkflow(opts workflowOptions) (runErr error) {
	if err := installAgentSandbox(context.Background()); err != nil {
		return err
	}

	if err := loadEnvironment(); err != nil {
		return err
	}

	cfg, err := config.Load(factoryConfigFile)
	if err != nil {
		return err
	}

	var loadedSkills sandbox.ProjectSkills

	entries := []struct {
		stage  config.Stage
		name   string
		target **sandbox.ProjectSkill
	}{
		{stage: config.StageCodeReview, name: cfg.CustomSkills.CodeReview, target: &loadedSkills.CodeReview},
		{stage: config.StageSecurityReview, name: cfg.CustomSkills.SecurityReview, target: &loadedSkills.SecurityReview},
		{stage: config.StageRiskClassification, name: cfg.CustomSkills.RiskClassification, target: &loadedSkills.RiskClassification},
		{stage: config.StageReviewChanges, name: cfg.CustomSkills.ReviewChanges, target: &loadedSkills.ReviewChanges},
	}

	for _, entry := range entries {
		// Disabled stages never read their skill files.
		if entry.name == "" || cfg.StageDisabled(entry.stage) {
			continue
		}

		skill, err := sandbox.LoadProjectSkill(
			filepath.Dir(factoryConfigFile),
			entry.name,
		)
		if err != nil {
			return fmt.Errorf("customSkills.%s: %w", entry.stage, err)
		}

		*entry.target = skill
	}

	branch, err := workflowBranch(opts)
	if err != nil {
		return err
	}
	var runLog *executionlog.Run
	if opts.Implement {
		command := executionlog.CommandRun
		if opts.Continue {
			command = executionlog.CommandContinue
		}
		var err error
		runLog, err = executionlog.NewRun(branch, command, plannedStageLabels(cfg))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: execution logging unavailable: %v; continuing execution.\n", err)
		} else {
			fmt.Printf("Execution logs: %s\n", runLog.Path())
			defer func() { runLog.Finish(runErr) }()
		}
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
		Continue:     opts.Continue,
	}

	if opts.Implement {
		if err := runStage(input, cfg, config.StageImplementation, "implementation", "Starting implementation...", func() error {
			return sandbox.Run(input)
		}); err != nil {
			return err
		}
	}

	// Only reviews that run in this invocation contribute report files.
	var reports []string
	var disabledReviews []config.Stage
	reviews := []struct {
		stage   config.Stage
		name    string
		message string
		review  func(sandbox.TaskContext) (string, error)
	}{
		{config.StageCodeReview, "code-review", "\nStarting code review and corrections...", sandbox.Review},
		{config.StageSecurityReview, "security-review", "\nStarting security review and corrections...", sandbox.SecurityReview},
	}
	for _, review := range reviews {
		if cfg.StageDisabled(review.stage) {
			disabledReviews = append(disabledReviews, review.stage)
		}
		if err := runStage(input, cfg, review.stage, review.name, review.message, func() error {
			report, err := review.review(input)
			reports = append(reports, report)
			return err
		}); err != nil {
			return err
		}
	}

	if err := runStage(input, cfg, config.StageRiskClassification, "risk-classification", "\nStarting risk classification...", func() error {
		return sandbox.RiskClassification(input, reports, disabledReviews)
	}); err != nil {
		return err
	}
	if cfg.StageDisabled(config.StageRiskClassification) {
		// Risk classification normally removes this run's review reports.
		sandbox.RemoveReviewReports(input, reports)
	}
	if !opts.Implement {
		return nil
	}
	if err := input.Context.Err(); err != nil {
		return err
	}
	if cfg.StageDisabled(config.StageReviewChanges) {
		skipStage(input, config.StageReviewChanges)
		return nil
	}
	fmt.Println("\nStarting final change walkthrough...")
	if err := sandbox.ReviewChanges(input); err != nil {
		if input.Context.Err() != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Warning: final change walkthrough unavailable: %v; continuing execution.\n", err)
	}
	return nil
}

// workflowBranch selects one branch for every stage and report in this run.
func workflowBranch(opts workflowOptions) (string, error) {
	settings, err := config.LoadSandbox()
	if err != nil {
		return "", err
	}
	mode := "resume"
	if opts.Implement && !opts.Continue {
		mode = executionlog.CommandRun
	}
	return settings.Branch(mode)
}

// plannedStageLabels lists the internal log labels of the logged stages that
// are enabled for this run, in pipeline order.
func plannedStageLabels(cfg config.Config) []string {
	var labels []string
	for _, stage := range []struct {
		stage config.Stage
		label string
	}{
		{config.StageImplementation, "implementation"},
		{config.StageCodeReview, "code-review"},
		{config.StageSecurityReview, "security-review"},
		{config.StageRiskClassification, "risk-classification"},
	} {
		if !cfg.StageDisabled(stage.stage) {
			labels = append(labels, stage.label)
		}
	}
	return labels
}

// runStage executes an enabled stage. name is the internal log label for the
// public stage identifier; a disabled stage is skipped without running execute.
func runStage(input sandbox.TaskContext, cfg config.Config, stage config.Stage, name, message string, execute func() error) error {
	if err := input.Context.Err(); err != nil {
		return err
	}
	if cfg.StageDisabled(stage) {
		skipStage(input, stage)
		return nil
	}
	if input.Log != nil {
		input.Log.StartStage(name)
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

		if logErr := input.Log.RecordStageResult(name, elapsed, err); logErr != nil {
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
