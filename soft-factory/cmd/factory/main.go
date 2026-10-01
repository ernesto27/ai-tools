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

func run(args []string) error {
	flags := flag.NewFlagSet("factory", flag.ContinueOnError)
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Usage: factory [-config config.json] [--jira <issue-url>] [review]")
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
	input := sandbox.TaskContext{TaskOverride: taskOverride, Documents: documents}

	if flags.NArg() == 0 {
		fmt.Println("Starting implementation...")

		if err := sandbox.Run(input); err != nil {
			return err
		}
	}

	fmt.Println("\nStarting code review and corrections...")

	codeReport, err := sandbox.Review(input)
	if err != nil {
		return err
	}

	fmt.Println("\nStarting security review and corrections...")

	securityReport, err := sandbox.SecurityReview(input)
	if err != nil {
		return err
	}
	fmt.Println("\nStarting risk classification...")

	if err := sandbox.RiskClassification(
		input,
		[]string{codeReport, securityReport},
	); err != nil {
		return err
	}
	return nil
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
