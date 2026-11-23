package schedule

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const (
	cronBegin = "# BEGIN SOFT FACTORY"
	cronEnd   = "# END SOFT FACTORY"
)

func prepareCrontab(tasks ScheduledTasks, outputPath string, requireAgent bool) (string, string, error) {
	block, err := renderCronBlock(tasks, outputPath, requireAgent)
	if err != nil {
		return "", "", err
	}
	current, err := readCrontab()
	if err != nil {
		return "", "", err
	}
	updated, err := replaceCronBlock(current, block)
	return current, updated, err
}

func readCrontab() (string, error) {
	cmd := exec.Command("crontab", "-l")
	cmd.Env = append(cmd.Environ(), "LC_ALL=C")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err == nil {
		return string(output), nil
	}

	message := strings.TrimSpace(stderr.String())
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		account, lookupErr := user.Current()
		if lookupErr == nil && message == "no crontab for "+account.Username {
			return "", nil
		}
	}
	return "", fmt.Errorf("read user crontab: %w: %s", err, message)
}

func renderCronBlock(tasks ScheduledTasks, outputPath string, requireAgent bool) (string, error) {
	repos := make([]string, 0, len(tasks))
	for repo := range tasks {
		repos = append(repos, repo)
	}
	slices.Sort(repos)

	var jobs strings.Builder
	var executableArg, outputArg, jobPath string
	for _, repo := range repos {
		for _, task := range tasks[repo] {
			if task.Cron == "" {
				continue // Entries saved before cron support have no timing.
			}
			if executableArg == "" {
				executable, err := exec.LookPath("software-factory")
				if err != nil {
					return "", fmt.Errorf("find software-factory on PATH: %w", err)
				}
				executableArg, err = quoteCronPath(executable)
				if err != nil {
					return "", err
				}
				jobPath, err = quoteCronSearchPath(os.Getenv("PATH"), requireAgent)
				if err != nil {
					return "", err
				}
				outputArg, err = quoteCronPath(outputPath)
				if err != nil {
					return "", fmt.Errorf("cron output log: %w", err)
				}
			}
			cron, err := normalizeCron(task.Cron)
			if err != nil {
				return "", fmt.Errorf("task %q in %s: %w", task.Name, repo, err)
			}
			repoArg, err := quoteCronPath(repo)
			if err != nil {
				return "", err
			}
			command := executableArg
			if strings.TrimSpace(task.FilePrompt) != "" {
				path := task.FilePrompt
				if !filepath.IsAbs(path) {
					path = filepath.Join(repo, path)
				}
				fileArg, err := quoteCronPath(path)
				if err != nil {
					return "", fmt.Errorf("task %q file prompt: %w", task.Name, err)
				}
				command += " --file " + fileArg
			}
			fmt.Fprintf(&jobs, "%s (cd %s && PATH=%s\"${PATH:+:$PATH}\" %s) >> %s 2>&1\n", cron, repoArg, jobPath, command, outputArg)
		}
	}
	if jobs.Len() == 0 {
		return "", nil
	}
	return cronBegin + "\n" + jobs.String() + cronEnd + "\n", nil
}

func quoteCronSearchPath(path string, requireAgent bool) (string, error) {
	tools := []string{"software-factory", "agent-sandbox", "git", "docker", "gh"}
	found := make(map[string]bool, len(tools))
	var pathArgs []string

	for _, dir := range filepath.SplitList(path) {
		if !filepath.IsAbs(dir) {
			continue
		}
		needed := false
		for _, tool := range tools {
			if found[tool] {
				continue
			}
			if _, err := exec.LookPath(filepath.Join(dir, tool)); err == nil {
				found[tool] = true
				needed = true
			}
		}
		if !needed {
			continue
		}
		arg, err := quoteCronPath(dir)
		if err != nil {
			return "", fmt.Errorf("schedule PATH: %w", err)
		}
		pathArgs = append(pathArgs, arg)
		if len(found) == len(tools) {
			break
		}
	}
	if requireAgent && !found["agent-sandbox"] {
		return "", errors.New("agent-sandbox was not found on the absolute directories in PATH")
	}
	return strings.Join(pathArgs, ":"), nil
}

func quoteCronPath(path string) (string, error) {
	// Cron processes percent signs before the shell interprets quotes.
	if !filepath.IsAbs(path) || strings.ContainsAny(path, "%\r\n\x00") {
		return "", fmt.Errorf("unsupported path for cron: %q", path)
	}
	return "'" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'", nil
}

func replaceCronBlock(current, block string) (string, error) {
	lines := strings.SplitAfter(current, "\n")
	begin, end := -1, -1
	for i, line := range lines {
		switch strings.TrimSuffix(line, "\n") {
		case cronBegin:
			if begin != -1 {
				return "", errors.New("duplicate Soft Factory crontab start marker")
			}
			begin = i
		case cronEnd:
			if end != -1 {
				return "", errors.New("duplicate Soft Factory crontab end marker")
			}
			end = i
		}
	}
	if begin == -1 && end == -1 {
		if block == "" {
			return current, nil
		}
		if current != "" && !strings.HasSuffix(current, "\n") {
			current += "\n"
		}
		return current + block, nil
	}
	if begin == -1 || end <= begin {
		return "", errors.New("invalid Soft Factory crontab markers")
	}
	return strings.Join(lines[:begin], "") + block + strings.Join(lines[end+1:], ""), nil
}

func installIfCrontabUnchanged(previous, updated string) error {
	current, err := readCrontab()
	if err != nil {
		return err
	}
	if current != previous {
		return errors.New("user crontab changed during the update")
	}
	if current == updated {
		return nil
	}

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(updated)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("install user crontab: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func normalizeCron(expression string) (string, error) {
	fields := strings.Fields(expression)
	limits := [...]struct{ min, max uint64 }{
		{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 7},
	}
	if len(fields) != len(limits) || strings.ContainsAny(expression, "\r\n") {
		return "", errors.New("cron expression must contain five fields on one line")
	}
	for i, field := range fields {
		for _, item := range strings.Split(field, ",") {
			if !validCronItem(item, limits[i].min, limits[i].max) {
				return "", fmt.Errorf("invalid cron field %d: %q", i+1, field)
			}
		}
	}
	return strings.Join(fields, " "), nil
}

func validCronItem(item string, min, max uint64) bool {
	base, step, hasStep := strings.Cut(item, "/")
	first, last, hasRange := strings.Cut(base, "-")
	if hasStep {
		n, err := strconv.ParseUint(step, 10, 8)
		if err != nil || n == 0 || (base != "*" && !hasRange) {
			return false
		}
	}
	if base == "*" {
		return true
	}
	start, err := strconv.ParseUint(first, 10, 8)
	if err != nil || start < min || start > max {
		return false
	}
	if !hasRange {
		return true
	}
	end, err := strconv.ParseUint(last, 10, 8)
	return err == nil && end >= start && end <= max
}
