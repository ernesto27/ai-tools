package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
	"soft-factory/internal/prompts"
)

func Audit(logPath string, cfg config.Config) error {
	if !filepath.IsAbs(logPath) {
		return fmt.Errorf("audit requires an absolute log path")
	}
	logPath = filepath.Clean(logPath)
	info, err := os.Stat(logPath)
	if err != nil {
		return fmt.Errorf("inspect audit log: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("audit log must be a regular file")
	}
	branch := cfg.AgentSandbox.Branch
	if cfg.AgentSandbox.Resume != nil && cfg.AgentSandbox.Resume.Branch != nil {
		branch = *cfg.AgentSandbox.Resume.Branch
	}
	worktree, err := executionlog.WorktreeForBranch(context.Background(), branch)
	if err != nil {
		return err
	}
	reportName := fmt.Sprintf(".soft-factory-audit-%d.html", time.Now().UnixNano())
	if _, err := os.Lstat(filepath.Join(worktree, reportName)); err == nil {
		return fmt.Errorf("temporary audit report already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect temporary audit report: %w", err)
	}
	filename := filepath.Base(logPath)
	stem := strings.TrimSuffix(filename, filepath.Ext(filename))
	outputPath := filepath.Join(filepath.Dir(logPath), stem+"-audit.html")
	prompt, err := prompts.Render("audit.txt", map[string]any{
		"LogPath":           filepath.ToSlash(filepath.Join("/agent-sandbox-files/1", filename)),
		"OutputHTMLPath":    executionlog.ContainerReportPath(reportName),
		"AuditHTMLTemplate": prompts.AuditHTMLTemplate(),
	})

	if err != nil {
		return err
	}

	path, err := writePrompt(prompt)
	if err != nil {
		return err
	}
	defer os.Remove(path)

	args := []string{"resume", "--push=false", "--pr=false", "-f", path, "--file", logPath}
	args = appendStageArgs(args, cfg.Audit)

	_, err = execute(ExecuteOptions{
		Config:      cfg,
		StageConfig: cfg.Audit,
		Args:        args,
		Prompt:      prompt,
		Branch:      branch,
		BranchSet:   true,
	})
	if err != nil {
		return err
	}
	source, err := executionlog.StageReportPath(context.Background(), branch, reportName)
	if err != nil {
		return err
	}
	if err := archiveReport(source, outputPath, true); err != nil {
		return err
	}
	if err := os.Remove(source); err != nil {
		return fmt.Errorf("remove temporary audit report: %w", err)
	}
	fmt.Printf("\nReport saved: %s\n", outputPath)
	return nil
}
