package sandbox

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	sandboxconfig "github.com/ernesto27/ai-tools/agent-sandbox/config"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
	"soft-factory/internal/prompts"
	"soft-factory/skills"
)

type ExecuteOptions struct {
	Config            config.Config
	BranchSet         bool
	Stage             config.Stage
	StageConfig       config.StageConfig
	Branch            string
	Args              []string
	ReportName        string
	Prompt            string
	Context           context.Context
	Log               *executionlog.Run
	ChangeSummaryFile string
	RiskResultFile    string
}

// ProjectSkill holds a project skill loaded during workflow setup.
type ProjectSkill struct {
	Name    string
	Content string
}

type ProjectSkills struct {
	CodeReview         *ProjectSkill
	SecurityReview     *ProjectSkill
	RiskClassification *ProjectSkill
	ReviewChanges      *ProjectSkill
}

// TaskContext carries a task override and supporting documents through all stages.
// An empty override retains the configured filePrompt behavior.
type TaskContext struct {
	TaskOverride string
	Documents    string
	CustomSkills ProjectSkills
	Context      context.Context
	Log          *executionlog.Run
	Branch       string
	BranchSet    bool
	Continue     bool
	Config       config.Config
}

func Run(input TaskContext) error {
	mode := "run"
	if input.Continue {
		mode = "resume"
	}
	args := []string{mode}

	var prompt string
	var changesFilename string
	if input.TaskOverride != "" || input.Documents != "" || input.Log != nil || input.Continue {
		var err error
		if input.TaskOverride != "" || input.Documents != "" {
			prompt, err = buildTaskPrompt(input)
		} else {
			// Snapshot the source before launch so later edits cannot change the
			// executed prompt independently of the saved log.
			prompt, err = readTaskFile(input.Config.AgentSandbox)
		}
		if err != nil {
			return err
		}

		prompt, changesFilename, err = input.Log.StageChangesPrompt(prompt)
		if err != nil {
			return err
		}
		path, err := writePrompt(prompt)
		if err != nil {
			return err
		}
		defer os.Remove(path)

		args = append(args, "-f", path)
	}

	args = appendStageArgs(args, input.Config.Stages.Implementation)
	_, err := execute(ExecuteOptions{
		Config:            input.Config,
		BranchSet:         input.BranchSet,
		Stage:             config.StageImplementation,
		StageConfig:       input.Config.Stages.Implementation,
		Args:              args,
		Branch:            input.Branch,
		Prompt:            prompt,
		Context:           input.Context,
		Log:               input.Log,
		ChangeSummaryFile: changesFilename,
	})

	return err
}

func customSkillPrompt(skill *ProjectSkill, reviewerOnly bool) (string, error) {
	if skill == nil {
		return "", nil
	}
	scope := ""
	if reviewerOnly {
		var err error
		scope, err = prompts.Render("reviewer-scope.txt", nil)
		if err != nil {
			return "", err
		}
	}
	return prompts.Render("custom-skill.txt", map[string]any{
		"Name": skill.Name, "Scope": scope, "Content": skill.Content,
	})
}

func Review(input TaskContext) (string, error) {
	additionalSkill, err := customSkillPrompt(input.CustomSkills.CodeReview, true)
	if err != nil {
		return "", err
	}
	return review(input, config.StageCodeReview, "code-review/SKILL.md", additionalSkill)
}

func SecurityReview(input TaskContext) (string, error) {
	if input.Config.Stages.SecurityReview.SaveFindings {
		return securityFindings(input)
	}
	additionalSkill, err := customSkillPrompt(input.CustomSkills.SecurityReview, true)
	if err != nil {
		return "", err
	}
	return review(input, config.StageSecurityReview, "security-review/SKILL.md", additionalSkill)
}

// ReviewChanges asks the sandbox agent to write the walkthrough, then archives it.
func ReviewChanges(input TaskContext) error {
	if input.Log == nil {
		return fmt.Errorf("change walkthrough requires an execution log directory")
	}
	skill, err := skills.ReadFile("review-changes/SKILL.md")
	if err != nil {
		return fmt.Errorf("read review-changes skill: %w", err)
	}
	task, err := buildTaskPrompt(input)
	if err != nil {
		return err
	}
	snapshot, err := changeEvidence(input.Context, input.Branch)
	if err != nil {
		return fmt.Errorf("collect final change evidence: %w", err)
	}
	source := filepath.Join(snapshot.Worktree, fmt.Sprintf(".soft-factory-review-changes-%d.md", time.Now().UnixNano()))
	if _, err := os.Lstat(source); err == nil {
		return fmt.Errorf("temporary change walkthrough path already exists: %q", source)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect temporary change walkthrough path %q: %w", source, err)
	}
	additionalSkill, err := customSkillPrompt(input.CustomSkills.ReviewChanges, false)
	if err != nil {
		return err
	}
	prompt, err := prompts.Render("review-changes.txt", map[string]any{
		"ReportPath": executionlog.ContainerReportPath(filepath.Base(source)),
		"Skill":      string(skill), "AdditionalSkill": additionalSkill,
		"Task": task, "HostEvidence": snapshot.Text,
	})
	if err != nil {
		return err
	}
	path, err := writePrompt(prompt)
	if err != nil {
		return err
	}
	defer os.Remove(path)

	args := []string{"resume", "--push=false", "--pr=false", "-f", path}

	args = appendStageArgs(args, input.Config.Stages.ReviewChanges.StageConfig)
	// The walkthrough keeps subprocess output live-only, so record its
	// configured selection before executing without a stream logger.
	selection, agent, model, provider := selectionDetails(input.Config, args[0], config.StageReviewChanges, input.Config.Stages.ReviewChanges.StageConfig)
	input.Log.Selection(selection.Agent, selection.Model)
	input.Log.Details(agent, model, provider)

	_, err = execute(ExecuteOptions{
		Config:      input.Config,
		BranchSet:   input.BranchSet,
		Stage:       config.StageReviewChanges,
		StageConfig: input.Config.Stages.ReviewChanges.StageConfig,
		Args:        args,
		Branch:      input.Branch,
		Prompt:      prompt,
		Context:     input.Context,
		Log:         nil,
	})
	resolvedSource, locateErr := executionlog.StageReportPath(input.Context, input.Branch, filepath.Base(source))
	if locateErr != nil {
		return errors.Join(err, locateErr)
	}
	source = resolvedSource
	destination := filepath.Join(input.Log.Path(), "05-"+string(config.StageReviewChanges)+".md")
	archiveErr := archiveReviewReport(source, destination)
	if archiveErr != nil {
		return errors.Join(err, archiveErr)
	}
	fmt.Printf("\nReport saved: %s\n", destination)
	if removeErr := os.Remove(source); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return errors.Join(err, fmt.Errorf("remove temporary change walkthrough %q: %w", source, removeErr))
	}
	return err
}

// LoadProjectSkill resolves and reads a named skill before a workflow starts.
func LoadProjectSkill(projectDir, name string) (*ProjectSkill, error) {
	first := filepath.Join(projectDir, ".agents", "skills", name, "SKILL.md")
	second := filepath.Join(projectDir, ".claude", "skills", name, "SKILL.md")
	for _, path := range []string{first, second} {
		data, err := os.ReadFile(path)
		if err == nil {
			return &ProjectSkill{Name: name, Content: string(data)}, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read project skill %q: %w", path, err)
		}
	}
	return nil, fmt.Errorf("project skill %q not found; searched %q and %q", name, first, second)
}

func review(input TaskContext, stage config.Stage, skillPath, additionalSkill string) (string, error) {
	var settings config.StageConfig

	switch stage {
	case config.StageCodeReview:
		settings = input.Config.Stages.CodeReview.StageConfig
	case config.StageSecurityReview:
		settings = input.Config.Stages.SecurityReview.StageConfig
	}

	skill, err := skills.ReadFile(skillPath)
	if err != nil {
		return "", fmt.Errorf("read review skill %q: %w", skillPath, err)
	}

	task, err := buildTaskPrompt(input)
	if err != nil {
		return "", err
	}

	prompt, err := prompts.Render("review.txt", map[string]any{
		"Skill": string(skill), "AdditionalSkill": additionalSkill, "Task": task,
	})
	if err != nil {
		return "", err
	}

	// A dedicated final report carries review evidence without execution transcripts.
	if input.Log == nil {
		if err := executionlog.IgnoreStageReports(); err != nil {
			return "", fmt.Errorf("prepare final review report: %w", err)
		}
	}
	reviewFilename := ".soft-factory-stage-changes-" + rand.Text() + "-review.txt"
	reportInstructions, err := prompts.Render("review-report.txt", map[string]any{
		"ReportPath": executionlog.ContainerReportPath(reviewFilename),
	})
	if err != nil {
		return "", err
	}
	prompt += reportInstructions

	prompt, changesFilename, err := input.Log.StageChangesPrompt(prompt)
	if err != nil {
		return "", err
	}
	path, err := writePrompt(prompt)
	if err != nil {
		return "", err
	}
	defer os.Remove(path)

	args := []string{"resume", "-f", path}
	args = appendStageArgs(args, settings)

	options := ExecuteOptions{
		Config:            input.Config,
		BranchSet:         input.BranchSet,
		Stage:             stage,
		StageConfig:       settings,
		Args:              args,
		Branch:            input.Branch,
		Prompt:            prompt,
		Context:           input.Context,
		Log:               input.Log,
		ChangeSummaryFile: changesFilename,
	}
	_, runErr := execute(options)
	if runErr == nil && input.Log != nil {
		source, archiveErr := executionlog.StageReportPath(input.Context, input.Branch, reviewFilename)
		if archiveErr == nil {
			archiveErr = input.Log.ArchiveReview(stage, source)
		}
		if archiveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: archive %s review evidence: %v\n", stage, archiveErr)
		}
	}
	return reviewFilename, runErr
}

func buildTaskPrompt(input TaskContext) (string, error) {
	task := input.TaskOverride
	if task == "" {
		var err error
		task, err = readTaskFile(input.Config.AgentSandbox)
		if err != nil {
			return "", err
		}
	}
	prompt, err := prompts.Render("task.txt", map[string]any{"Task": task})
	if err != nil {
		return "", err
	}
	if input.Documents != "" {
		contextPrompt, err := prompts.Render("supporting-context.txt", map[string]any{"Documents": input.Documents})
		if err != nil {
			return "", err
		}
		prompt += contextPrompt
	}
	return prompt, nil
}

func readTaskFile(settings sandboxconfig.Config) (string, error) {
	var taskPath string
	if settings.Run != nil && settings.Run.FilePrompt != nil {
		taskPath = *settings.Run.FilePrompt
	}

	task, err := os.ReadFile(taskPath)
	if err != nil {
		return "", fmt.Errorf("read software-factory.json: agentSandbox.run.filePrompt %q: %w", taskPath, err)
	}

	return string(task), nil
}

func writePrompt(prompt string) (string, error) {
	file, err := os.CreateTemp("", "factory-prompt-*.txt")
	if err != nil {
		return "", fmt.Errorf("create prompt file: %w", err)
	}

	_, writeErr := file.WriteString(prompt)
	closeErr := file.Close()

	if writeErr != nil {
		os.Remove(file.Name())
		return "", fmt.Errorf("write prompt file: %w", writeErr)
	}

	if closeErr != nil {
		os.Remove(file.Name())
		return "", fmt.Errorf("close prompt file: %w", closeErr)
	}

	return file.Name(), nil
}

func appendStageArgs(args []string, settings config.StageConfig) []string {
	if settings.Agent != "" {
		args = append(args, "-a", settings.Agent)
	}
	if settings.Model != "" {
		args = append(args, "-m", settings.Model)
	}
	return args
}

func execute(options ExecuteOptions) (string, error) {
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	options.Args = sandboxArgs(options)
	cmd := exec.CommandContext(ctx, "agent-sandbox", options.Args...)
	// Let the sandbox stop its container on interruption before forcing exit.
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 20 * time.Second
	cmd.Stdin = os.Stdin

	if options.Log != nil {
		selection, agent, model, provider := selectionDetails(options.Config, options.Args[0], options.Stage, options.StageConfig)
		options.Log.Selection(selection.Agent, selection.Model)
		options.Log.Details(agent, model, provider)
		options.Log.Command(cmd.Path, options.Args)
		options.Log.Prompt(options.Prompt)
	}

	var output bytes.Buffer
	var stdout io.Writer = os.Stdout
	var stderr io.Writer = os.Stderr
	if options.ReportName != "" {
		// Assign the exact same writer to both streams so os/exec shares one
		// child pipe and copy goroutine, preserving combined report ordering.
		stdout = io.MultiWriter(os.Stdout, &output)
		if options.Log != nil {
			stdout = io.MultiWriter(options.Log.Stream("combined"), stdout)
		}
		stderr = stdout
	} else if options.Log != nil {
		stdout = io.MultiWriter(options.Log.Stream("stdout"), stdout)
		stderr = io.MultiWriter(options.Log.Stream("stderr"), stderr)
	}
	cmd.Stdout, cmd.Stderr = stdout, stderr

	runErr := cmd.Run()
	if err := options.Log.ArchiveVerification(ctx, options.Branch, options.Stage); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: archive %s verification: %v\n", options.Stage, err)
	}
	if options.Log != nil && options.ChangeSummaryFile != "" {
		options.Log.RegisterStageChanges(ctx, options.Branch, options.ChangeSummaryFile)
	}
	if options.Log != nil && options.RiskResultFile != "" {
		options.Log.RegisterRiskResult(ctx, options.Branch, options.RiskResultFile)
	}
	if ctx.Err() != nil {
		runErr = errors.Join(runErr, ctx.Err())
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) && exitErr.ExitCode() == 130 {
		runErr = errors.Join(runErr, context.Canceled)
	}
	if options.Log != nil {
		if cmd.ProcessState != nil {
			options.Log.ExitCode(cmd.ProcessState.ExitCode())
		}
	}
	if runErr != nil {
		if options.ReportName != "" {
			fmt.Fprintf(&output, "\nCommand error: %v\n", runErr)
		}
		runErr = fmt.Errorf("execute agent-sandbox: %w", runErr)
	}
	if options.ReportName == "" {
		return "", runErr
	}
	reportPath, reportErr := saveReport(options.ReportName, output.Bytes())
	if options.Log != nil && reportPath != "" {
		options.Log.Report(reportPath)
	}

	return reportPath, errors.Join(runErr, reportErr)
}

// selectionDetails reads only supported selection fields. Provider means the
// selected sandbox agent, not its underlying API backend. Do not copy the
// configuration into logs: it may also contain API keys. Selection contains
// plain names: stage overrides, then run/resume, then shared fields and defaults.
func selectionDetails(cfg config.Config, section string, stage config.Stage, override config.StageConfig) (config.StageConfig, string, string, string) {
	agent := "unavailable (no configured selection)"
	model := "unavailable (no configured model)"
	provider := "unavailable (no configured agent)"
	var agentValue, modelValue string
	agentSource, modelSource := "software-factory.json: agentSandbox.agent", "software-factory.json: agentSandbox.model"
	settings := cfg.AgentSandbox
	agentValue, modelValue = settings.Agent, settings.Model
	selected := settings.Run
	if section == "resume" {
		selected = settings.Resume
	}
	if selected != nil {
		if selected.Agent != nil {
			agentValue, agentSource = *selected.Agent, "software-factory.json: agentSandbox."+section+".agent"
		}
		if selected.Model != nil {
			modelValue, modelSource = *selected.Model, "software-factory.json: agentSandbox."+section+".model"
		}
	}
	stageSource := "software-factory.json: stages." + string(stage)
	if override.Agent != "" {
		agentValue, agentSource = override.Agent, stageSource+".agent"
	}
	if override.Model != "" {
		modelValue, modelSource = override.Model, stageSource+".model"
	}
	if agentValue != "" {
		agent = agentValue + " (configured: " + agentSource + ")"
		provider = agentValue + " (" + agentSource + ")"
	}
	if modelValue != "" {
		model = modelValue + " (configured: " + modelSource + ")"
	} else if defaultModel, err := sandboxconfig.DefaultModel(agentValue); err == nil {
		modelValue = defaultModel
		model = defaultModel
	}
	selection := config.StageConfig{Agent: agentValue, Model: modelValue}
	return selection, agent, model, provider
}

func saveReport(name string, content []byte) (string, error) {
	if err := os.MkdirAll("docs", 0755); err != nil {
		return "", fmt.Errorf("create report directory: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05.000000000")
	filename := fmt.Sprintf("%s-%s.md", name, timestamp)
	path := filepath.Join("docs", filename)
	if err := os.WriteFile(path, content, 0600); err != nil {
		return "", fmt.Errorf("save report %q: %w", path, err)
	}

	fmt.Printf("\nReport saved: %s\n", path)

	return path, nil
}

// archiveReviewReport copies the agent-authored file without rewriting its content.
func archiveReviewReport(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("inspect change walkthrough %q: %w", source, err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("change walkthrough %q must be a nonempty regular file", source)
	}
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open change walkthrough %q: %w", source, err)
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create archived change walkthrough %q: %w", destination, err)
	}
	n, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if err := errors.Join(copyErr, closeErr); err != nil || n == 0 {
		os.Remove(destination)
		if err == nil {
			err = fmt.Errorf("source file became empty")
		}
		return fmt.Errorf("copy change walkthrough to %q: %w", destination, err)
	}
	return nil
}

// riskReviewEvidence lists this run's review reports and names disabled
// reviews so the classifier neither expects nor trusts older reports.
func riskReviewEvidence(reportFiles []string, disabledReviews []config.Stage) (string, error) {
	var evidence strings.Builder
	for _, filename := range reportFiles {
		fmt.Fprintf(&evidence, "- %q\n", filename)
	}
	if len(reportFiles) == 0 {
		text, err := prompts.Render("no-review-reports.txt", nil)
		if err != nil {
			return "", err
		}
		evidence.WriteString(text)
	}
	if len(disabledReviews) > 0 {
		names := make([]string, len(disabledReviews))
		for i, stage := range disabledReviews {
			names[i] = string(stage)
		}
		text, err := prompts.Render("disabled-reviews.txt", map[string]any{"Stages": strings.Join(names, ", ")})
		if err != nil {
			return "", err
		}
		evidence.WriteString(text)
	}
	return evidence.String(), nil
}

// RemoveReviewReports deletes the named review reports from this run's
// worktree, as risk classification does after reading them. Empty names are
// ignored, and failures only warn.
func RemoveReviewReports(input TaskContext, reportFiles []string) {
	for _, filename := range reportFiles {
		if filename == "" {
			continue
		}
		if input.Log.RetainReviewReport(filename) {
			continue
		}
		reportPath, err := executionlog.StageReportPath(input.Context, input.Branch, filename)
		if err == nil {
			err = os.Remove(reportPath)
		}
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: remove review report: %v\n", err)
		}
	}
}

// RiskClassification assesses the final changes using only the review reports
// produced by enabled reviews in this run. Empty report names are ignored.
func RiskClassification(input TaskContext, reportFiles []string, disabledReviews []config.Stage) error {
	reportFiles = slices.DeleteFunc(slices.Clone(reportFiles), func(filename string) bool { return filename == "" })
	for _, filename := range reportFiles {
		reportPath, err := executionlog.StageReportPath(input.Context, input.Branch, filename)
		if err != nil {
			return fmt.Errorf("locate review report: %w", err)
		}
		defer func(path string) {
			if input.Log.RetainReviewReport(filepath.Base(path)) {
				return
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Warning: remove review report: %v\n", err)
			}
		}(reportPath)
	}

	skill, err := skills.ReadFile("risk-classification/SKILL.md")
	if err != nil {
		return fmt.Errorf("read risk-classification skill: %w", err)
	}

	task, err := buildTaskPrompt(input)
	if err != nil {
		return err
	}

	additionalSkill, err := customSkillPrompt(input.CustomSkills.RiskClassification, false)
	if err != nil {
		return err
	}
	reviewEvidence, err := riskReviewEvidence(reportFiles, disabledReviews)
	if err != nil {
		return err
	}
	if input.Config.Stages.SecurityReview.SaveFindings && !input.Config.StageDisabled(config.StageSecurityReview) {
		reviewEvidence += "\nSecurity review uses saveFindings: findings and suggested fixes were intentionally left unapplied. Do not describe them as fixed.\n"
	}
	prompt, err := prompts.Render("risk-classification.txt", map[string]any{
		"ReportDirectory": executionlog.ContainerReportPath(""),
		"Skill":           string(skill), "AdditionalSkill": additionalSkill,
		"Task": task, "ReviewEvidence": reviewEvidence,
	})
	if err != nil {
		return err
	}

	prompt, resultFilename, err := input.Log.RiskResultPrompt(prompt)
	if err != nil {
		return err
	}
	path, err := writePrompt(prompt)
	if err != nil {
		return err
	}
	defer os.Remove(path)

	args := []string{"resume", "--push=false", "--pr=false", "-f", path}
	args = appendStageArgs(args, input.Config.Stages.RiskClassification.StageConfig)

	options := ExecuteOptions{
		Config:         input.Config,
		BranchSet:      input.BranchSet,
		Stage:          config.StageRiskClassification,
		StageConfig:    input.Config.Stages.RiskClassification.StageConfig,
		Args:           args,
		Branch:         input.Branch,
		Prompt:         prompt,
		Context:        input.Context,
		Log:            input.Log,
		RiskResultFile: resultFilename,
	}
	if input.Log == nil {
		options.ReportName = string(config.StageRiskClassification)
	}
	_, err = execute(options)

	return err
}
