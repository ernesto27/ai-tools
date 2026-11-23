package schedule

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type ScheduledTask struct {
	Name       string `json:"name"`
	FilePrompt string `json:"filePrompt"`
	Cron       string `json:"cron,omitempty"`
}

func (task ScheduledTask) validate() error {
	if task.Name == "" || strings.TrimSpace(task.FilePrompt) == "" {
		return errors.New("name and filePrompt are required")
	}
	return nil
}

type ScheduledTasks map[string][]ScheduledTask

type Schedule struct {
	filePath string
}

func NewSchedule() (*Schedule, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	filePath := filepath.Join(homeDir, ".config", "software-factory", "scheduled-tasks", "cron.json")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
		return nil, fmt.Errorf("create scheduled tasks directory: %w", err)
	}
	s := &Schedule{filePath: filePath}
	lock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	if err := ensureCronFile(filePath); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Schedule) Create(repoPath string, task ScheduledTask) error {
	if !filepath.IsAbs(repoPath) {
		return errors.New("repository path must be absolute")
	}
	if err := task.validate(); err != nil {
		return err
	}
	cron, err := normalizeCron(task.Cron)
	if err != nil {
		return err
	}
	task.Cron = cron

	lock, err := s.lock()
	if err != nil {
		return err
	}
	defer lock.Close()

	tasks, err := s.load()
	if err != nil {
		return err
	}

	repoPath = filepath.Clean(repoPath)
	for _, existing := range tasks[repoPath] {
		if existing.Name == task.Name {
			return fmt.Errorf("task %q already exists for %s", task.Name, repoPath)
		}
	}
	tasks[repoPath] = append(tasks[repoPath], task)
	return s.saveAndInstallCrontab(tasks, task.Name)
}

func (s *Schedule) Edit(repoPath string, task ScheduledTask) error {
	if !filepath.IsAbs(repoPath) {
		return errors.New("repository path must be absolute")
	}
	if err := task.validate(); err != nil {
		return err
	}
	if task.Cron != "" {
		cron, err := normalizeCron(task.Cron)
		if err != nil {
			return err
		}
		task.Cron = cron
	}

	lock, err := s.lock()
	if err != nil {
		return err
	}
	defer lock.Close()

	tasks, err := s.load()
	if err != nil {
		return err
	}

	repoPath = filepath.Clean(repoPath)
	for i, existing := range tasks[repoPath] {
		if existing.Name != task.Name {
			continue
		}
		if task.Cron == "" {
			task.Cron = existing.Cron
		}
		tasks[repoPath][i] = task
		if task.Cron == "" {
			return s.save(tasks) // Older entries may have no cron expression.
		}
		return s.saveAndInstallCrontab(tasks, task.Name)
	}
	return fmt.Errorf("task %q not found for %s", task.Name, repoPath)
}

func (s *Schedule) Delete(repoPath, name string) error {
	if !filepath.IsAbs(repoPath) {
		return errors.New("repository path must be absolute")
	}
	if name == "" {
		return errors.New("task name is required")
	}

	lock, err := s.lock()
	if err != nil {
		return err
	}
	defer lock.Close()

	tasks, err := s.load()
	if err != nil {
		return err
	}

	repoPath = filepath.Clean(repoPath)
	repoTasks := tasks[repoPath]
	for i, task := range repoTasks {
		if task.Name != name {
			continue
		}
		if len(repoTasks) == 1 {
			delete(tasks, repoPath)
		} else {
			tasks[repoPath] = append(repoTasks[:i], repoTasks[i+1:]...)
		}
		if task.Cron == "" {
			return s.save(tasks)
		}
		return s.saveAndInstallCrontab(tasks, name)
	}
	return fmt.Errorf("task %q not found for %s", name, repoPath)
}

func (s *Schedule) load() (ScheduledTasks, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return nil, fmt.Errorf("read cron file: %w", err)
	}

	var tasks ScheduledTasks
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("decode cron file: %w", err)
	}
	if tasks == nil {
		return nil, errors.New("cron file must contain a JSON object")
	}
	return tasks, nil
}

func (s *Schedule) saveAndInstallCrontab(tasks ScheduledTasks, name string) error {
	outputPath := filepath.Join(filepath.Dir(s.filePath), "cron-output.log")
	current, updated, err := prepareCrontab(tasks, outputPath)
	if err != nil {
		return err
	}
	if err := s.save(tasks); err != nil {
		return err
	}
	if err := installIfCrontabUnchanged(current, updated); err != nil {
		return fmt.Errorf("schedule change for %q saved to JSON, but crontab installation is pending: %w", name, err)
	}
	return nil
}

func (s *Schedule) save(tasks ScheduledTasks) error {
	file, err := os.CreateTemp(filepath.Dir(s.filePath), "cron-*.json")
	if err != nil {
		return fmt.Errorf("create temporary cron file: %w", err)
	}
	defer os.Remove(file.Name())

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	writeErr := encoder.Encode(tasks)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return fmt.Errorf("write cron file: %w", err)
	}
	if err := os.Rename(file.Name(), s.filePath); err != nil {
		return fmt.Errorf("replace cron file: %w", err)
	}
	return nil
}

func (s *Schedule) lock() (*os.File, error) {
	// Keep the lock on a separate file because save replaces cron.json.
	file, err := os.OpenFile(s.filePath+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open schedule lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, fmt.Errorf("lock schedules: %w", err)
	}
	return file, nil
}

func ensureCronFile(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("create cron file: %w", err)
	}

	_, writeErr := file.WriteString("{}\n")
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return fmt.Errorf("initialize cron file %q: %w", path, err)
	}
	return nil
}
