package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"soft-factory/internal/config"
	"soft-factory/internal/sandbox"
	"soft-factory/internal/taskcontext"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: factory [-config config.json] [review]")
		flag.PrintDefaults()
	}

	configPath := flag.String(
		"config",
		"config.json",
		"Path to the factory configuration",
	)
	flag.Parse()

	if flag.NArg() > 1 || (flag.NArg() == 1 && flag.Arg(0) != "review") {
		flag.Usage()
		os.Exit(1)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	documents, err := taskcontext.Build(ctx, cfg, filepath.Dir(*configPath))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if flag.NArg() == 0 {
		fmt.Println("Starting implementation...")

		if err := sandbox.Run(documents); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	}

	fmt.Println("\nStarting code review and corrections...")

	codeReport, err := sandbox.Review(documents)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Println("\nStarting security review and corrections...")

	securityReport, err := sandbox.SecurityReview(documents)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	fmt.Println("\nStarting risk classification...")

	if err := sandbox.RiskClassification(
		documents,
		[]string{codeReport, securityReport},
	); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
