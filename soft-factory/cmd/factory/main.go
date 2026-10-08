package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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
}

func runWorkflow(opts workflowOptions) (runErr error) {
	var runLog *executionlog.Run
	if opts.Implement {
		var err error
		runLog, err = executionlog.NewRun()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: execution logging unavailable: %v; continuing execution.\n", err)
		} else {
			fmt.Printf("Execution logs: %s\n", runLog.Path())
			defer func() { runLog.Finish(runErr) }()
		}
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
		property string
		name     string
		target   **sandbox.ProjectSkill
	}{
		{property: "codeReview", name: cfg.CustomSkills.CodeReview, target: &loadedSkills.CodeReview},
		{property: "securityReview", name: cfg.CustomSkills.SecurityReview, target: &loadedSkills.SecurityReview},
		{property: "riskClassification", name: cfg.CustomSkills.RiskClassification, target: &loadedSkills.RiskClassification},
		{property: "reviewChanges", name: cfg.CustomSkills.ReviewChanges, target: &loadedSkills.ReviewChanges},
	}

	for _, entry := range entries {
		if entry.name == "" {
			continue
		}

		skill, err := sandbox.LoadProjectSkill(
			filepath.Dir(factoryConfigFile),
			entry.name,
		)
		if err != nil {
			return fmt.Errorf("customSkills.%s: %w", entry.property, err)
		}

		*entry.target = skill
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
	}

	if opts.Implement {
		if err := runStage(input, "implementation", "Starting implementation...", func() error {
			return sandbox.Run(input)
		}); err != nil {
			return err
		}
	}

	var codeReport, securityReport string
	if err := runStage(input, "code-review", "\nStarting code review and corrections...", func() error {
		var err error
		codeReport, err = sandbox.Review(input)
		return err
	}); err != nil {
		return err
	}

	if err := runStage(input, "security-review", "\nStarting security review and corrections...", func() error {
		var err error
		securityReport, err = sandbox.SecurityReview(input)
		return err
	}); err != nil {
		return err
	}

	if err := runStage(input, "risk-classification", "\nStarting risk classification...", func() error {
		return sandbox.RiskClassification(input, []string{codeReport, securityReport})
	}); err != nil {
		return err
	}
	if !opts.Implement {
		return nil
	}
	if err := input.Context.Err(); err != nil {
		return err
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

func runStage(input sandbox.TaskContext, name, message string, execute func() error) error {
	if err := input.Context.Err(); err != nil {
		return err
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
