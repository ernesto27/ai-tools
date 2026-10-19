package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/clierror"
	"github.com/ernesto27/ai-tools/agent-sandbox/internal/sandbox"
)

// reportError leaves process exit to its caller so the real command boundary is
// testable without launching Docker or terminating the test process.
func reportError(cmd *cobra.Command, out io.Writer, err error, width int, color bool) int {
	legacy := cmd.Name() == "run-tui" || cmd.Name() == "resume-tui"
	var usage sandbox.UsageError
	if errors.As(err, &usage) {
		if usage.Error() != "" {
			if legacy {
				fmt.Fprintf(out, "error: %s\n", usage.Error())
			} else {
				renderError(out, err, clierror.Usage, width, color)
			}
		}
		fmt.Fprint(out, cmd.UsageString())
		return sandbox.ExitUsage
	}
	if errors.Is(err, context.Canceled) {
		if legacy {
			fmt.Fprintln(out, "interrupted; the container was stopped")
		} else {
			renderError(out, &clierror.Error{Category: clierror.General, Summary: "Execution interrupted.", Cause: err}, clierror.General, width, color)
		}
		return sandbox.ExitInterrupted
	}
	if legacy {
		fmt.Fprintf(out, "error: %s\n", err)
	} else {
		renderError(out, err, clierror.General, width, color)
	}
	var status sandbox.StatusError
	if errors.As(err, &status) {
		return status.Status
	}
	return sandbox.ExitFailure
}

func errorTerminal(out io.Writer) (int, bool) {
	file, ok := out.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return 80, false
	}
	width, _, err := term.GetSize(int(file.Fd()))
	if err != nil || width <= 0 {
		width = 80
	}
	_, noColor := os.LookupEnv("NO_COLOR")
	return width, !noColor
}

func renderError(out io.Writer, err error, fallback clierror.Category, width int, color bool) {
	diagnostic := &clierror.Error{Category: fallback, Summary: "The operation failed."}
	var classified *clierror.Error
	if errors.As(err, &classified) {
		diagnostic = classified
	}
	category := diagnostic.Category
	if category == "" {
		category = fallback
	}
	summary := diagnostic.Summary
	if summary == "" {
		summary = "The operation failed."
	}
	if width < 12 {
		width = 12
	}
	styles := newErrorStyles(color)
	headline := styles.headline.Render(wrapErrorProse("Error: "+summary, width))
	fmt.Fprintf(out, "\n%s\n\n", headline)
	labelWidth := ansi.StringWidth("Category:")
	for _, detail := range diagnostic.Details {
		if detail.Value != "" {
			labelWidth = max(labelWidth, ansi.StringWidth(detail.Label+":"))
		}
	}
	field := func(label, value string) {
		label += ":"
		padding := strings.Repeat(" ", labelWidth-ansi.StringWidth(label)+1)
		fmt.Fprintf(out, "%s%s%s\n", styles.label.Render(label), padding, value)
	}
	field("Category", string(category))
	for _, detail := range diagnostic.Details {
		if detail.Value != "" {
			field(detail.Label, detail.Value)
		}
	}
	// Keep the complete returned error, including outer operation context and
	// joined causes. Colon characters are ordinary diagnostic data.
	if cause := err.Error(); cause != summary && cause != "" {
		fmt.Fprintf(out, "\n%s\n%s\n", styles.label.Render("Cause:"), cause)
	}
	if len(diagnostic.Hints) > 0 {
		fmt.Fprintln(out, "\n"+styles.nextSteps.Render("Next steps:"))
		for _, hint := range diagnostic.Hints {
			fmt.Fprintf(out, "- %s\n", strings.ReplaceAll(wrapErrorProse(hint, width-2), "\n", "\n  "))
		}
	}
	fmt.Fprintln(out)
}

type errorStyles struct {
	headline, label, nextSteps lipgloss.Style
}

func newErrorStyles(color bool) errorStyles {
	if !color {
		return errorStyles{}
	}
	return errorStyles{
		headline:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(1)),
		label:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(6)),
		nextSteps: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(2)),
	}
}

// Wrap only prose at whitespace so hyphens in flags and command names remain
// intact. Diagnostic causes and detail values are printed verbatim.
func wrapErrorProse(text string, width int) string {
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(paragraph) {
			if line != "" && ansi.StringWidth(line)+1+ansi.StringWidth(word) > width {
				lines = append(lines, line)
				line = word
			} else if line == "" {
				line = word
			} else {
				line += " " + word
			}
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
