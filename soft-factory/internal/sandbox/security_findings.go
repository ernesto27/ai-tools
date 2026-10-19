package sandbox

import (
	"crypto/rand"
	"fmt"
	"os"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
	"soft-factory/internal/prompts"
	"soft-factory/skills"
)

func securityFindings(input TaskContext) (string, error) {
	input.Log.BeginSecurityFindings()
	skill, err := skills.ReadFile("security-review/SKILL.md")
	if err != nil {
		return "", fmt.Errorf("read security review skill: %w", err)
	}
	task, err := buildTaskPrompt(input)
	if err != nil {
		return "", err
	}
	snapshot, err := changeEvidence(input.Context, input.Branch)
	if err != nil {
		return "", fmt.Errorf("collect security review evidence: %w", err)
	}
	if input.Log == nil {
		if err := executionlog.IgnoreStageReports(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: prepare security findings report: %v\n", err)
		}
	}
	filename := ".soft-factory-stage-changes-" + rand.Text() + "-review.txt"
	additional := "(none)"
	if input.CustomSkills.SecurityReview != nil {
		additional = input.CustomSkills.SecurityReview.Content
	}
	prompt, err := prompts.Render("security-findings.txt", map[string]any{
		"Skill": string(skill), "AdditionalSkill": additional, "Task": task,
		"HostEvidence": snapshot.Text, "ReportPath": executionlog.ContainerReportPath(filename),
	})
	if err != nil {
		return "", err
	}
	path, err := writePrompt(prompt)
	if err != nil {
		return "", err
	}
	defer os.Remove(path)
	settings := input.Config.Stages.SecurityReview.StageConfig
	_, runErr := execute(ExecuteOptions{
		Config: input.Config, BranchSet: input.BranchSet, Stage: config.StageSecurityReview,
		StageConfig: settings, Args: appendStageArgs([]string{"resume", "-f", path}, settings),
		Branch: input.Branch, Prompt: prompt, Context: input.Context, Log: input.Log,
	})
	// Collect only this attempt's dedicated final report, including on failure.
	// Collection errors warn and never change execution or re-enable corrections.
	if input.Log != nil {
		source, err := executionlog.StageReportPath(input.Context, input.Branch, filename)
		input.Log.SecurityFindings(filename, source, err)
		if runErr == nil && err == nil {
			if err := input.Log.ArchiveReview(config.StageSecurityReview, source); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: archive security review evidence: %v\n", err)
			}
		}
	}
	return filename, runErr
}
