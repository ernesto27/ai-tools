package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
	"soft-factory/internal/jira"
	"soft-factory/internal/sandbox"
	"soft-factory/internal/taskcontext"
)

func main() {
	if err := run(os.Args[1:]); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string) (runErr error) {
	flags := flag.NewFlagSet("software-factory", flag.ContinueOnError)
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Usage: software-factory [-config config.json] [--jira <issue-url>] [review]")
		flags.PrintDefaults()
	}

	configPath := flags.String(
		"config",
		"config.json",
		"Path to the factory configuration",
	)
	issueURL := flags.String("jira", "", "Jira Cloud issue URL to use instead of run.file-prompt")
	if err := flags.Parse(args); err != nil {
		return err
	}

	if flags.NArg() > 1 || (flags.NArg() == 1 && flags.Arg(0) != "review") {
		flags.Usage()
		return fmt.Errorf("expected only the optional review command; place flags before it")
	}
	var emptyJira bool
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "jira" && strings.TrimSpace(*issueURL) == "" {
			emptyJira = true
		}
	})
	if emptyJira {
		return fmt.Errorf("--jira requires a non-empty issue URL")
	}
	var runLog *executionlog.Run
	if flags.NArg() == 0 {
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

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var taskOverride string
	if *issueURL != "" {
		taskOverride, err = loadJiraTask(ctx, *issueURL)
		if err != nil {
			return err
		}
	}

	documents, err := taskcontext.Build(ctx, cfg, filepath.Dir(*configPath))
	if err != nil {
		return err
	}
	input := sandbox.TaskContext{
		TaskOverride: taskOverride,
		Documents:    documents,
		Context:      ctx,
		Log:          runLog,
	}

	if flags.NArg() == 0 {
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

	return runStage(input, "risk-classification", "\nStarting risk classification...", func() error {
		return sandbox.RiskClassification(input, []string{codeReport, securityReport})
	})
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
	err := execute()
	if input.Context.Err() != nil {
		err = errors.Join(err, input.Context.Err())
	}
	if input.Log != nil {
		input.Log.FinishStage(err)
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
