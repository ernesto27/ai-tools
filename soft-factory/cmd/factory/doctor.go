package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"soft-factory/internal/doctor"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check whether agent-sandbox, git, and docker are installed on PATH.",
		Long:  "Check all required executables on PATH and report the results together.\nDoes not check configuration, agent CLIs, or Docker daemon access.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			results := doctor.Check(exec.LookPath)
			return writeDoctorReport(out, results, os.Getenv("NO_COLOR") == "")
		},
	}
}

func writeDoctorReport(out io.Writer, results []doctor.Result, color bool) error {
	var report strings.Builder
	report.WriteString("Dependency checks:\n\n")
	nameWidth := len("Dependency")
	for _, result := range results {
		if len(result.Name) > nameWidth {
			nameWidth = len(result.Name)
		}
	}
	fmt.Fprintf(&report, "%-*s  %-7s  Path\n", nameWidth, "Dependency", "Status")
	fmt.Fprintf(&report, "%s  -------  ----\n", strings.Repeat("-", nameWidth))
	var missing []string
	for _, result := range results {
		status := "FOUND"
		if result.Path == "" {
			status = "MISSING"
			missing = append(missing, result.Name)
		}
		padding := strings.Repeat(" ", 7-len(status))
		if color {
			code := "\x1b[32m"
			if result.Path == "" {
				code = "\x1b[31m"
			}
			status = code + status + "\x1b[0m"
		}
		path := result.Path
		if path == "" {
			path = "-"
		}
		fmt.Fprintf(&report, "%-*s  %s%s  %s\n", nameWidth, result.Name, status, padding, path)
	}
	fmt.Fprintf(&report, "\nResult: %d/%d dependencies found.\n", len(results)-len(missing), len(results))
	if _, err := io.WriteString(out, report.String()); err != nil {
		return fmt.Errorf("write doctor report: %w", err)
	}
	if len(missing) != 0 {
		return fmt.Errorf("missing dependencies: %s", strings.Join(missing, ", "))
	}
	return nil
}
