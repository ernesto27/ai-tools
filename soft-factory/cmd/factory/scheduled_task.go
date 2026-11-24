package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"soft-factory/internal/config"
	"soft-factory/internal/schedule"
)

func newScheduledTaskCmd() *cobra.Command {
	parent := &cobra.Command{
		Use:   "scheduled-task",
		Short: "Manage scheduled Soft Factory runs",
	}
	var name, cron, file string
	create := &cobra.Command{
		Use:   "create",
		Short: "Schedule the default workflow in the current project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return createScheduledTask(cmd, strings.TrimSpace(name), cron, file)
		},
	}
	create.Flags().StringVar(&name, "name", "", "Schedule name within the project")
	create.Flags().StringVar(&cron, "cron", "", "Five-field cron expression")
	create.Flags().StringVar(&file, "file", "", "Task prompt file path, relative to the project or absolute")
	for _, flag := range []string{"name", "cron", "file"} {
		if err := create.MarkFlagRequired(flag); err != nil {
			panic(err)
		}
	}
	parent.AddCommand(create, newScheduledTaskEditCmd(), newScheduledTaskDeleteCmd(), newScheduledTaskListCmd())
	return parent
}

func newScheduledTaskListCmd() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved schedules for a repository as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(path) == "" {
				return errors.New("--path requires a non-empty repository path")
			}
			schedules, err := schedule.NewSchedule()
			if err != nil {
				return err
			}
			tasks, err := schedules.List(path)
			if err != nil {
				return err
			}
			if err := json.NewEncoder(cmd.OutOrStdout()).Encode(tasks); err != nil {
				return fmt.Errorf("write scheduled tasks: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "Repository directory, relative to the current directory or absolute")
	if err := cmd.MarkFlagRequired("path"); err != nil {
		panic(err)
	}
	return cmd
}

func newScheduledTaskEditCmd() *cobra.Command {
	var name, file, cron string
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Update a schedule in the current project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			projectDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve current directory: %w", err)
			}
			schedules, err := schedule.NewSchedule()
			if err != nil {
				return err
			}
			task := schedule.ScheduledTask{Name: strings.TrimSpace(name), FilePrompt: file, Cron: cron}
			if err := schedules.Edit(projectDir, task); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Updated schedule %q.\n", task.Name)
			return err
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Existing schedule name")
	cmd.Flags().StringVar(&file, "file", "", "Task prompt file path")
	cmd.Flags().StringVar(&cron, "cron", "", "New cron expression (omit to keep the current time)")
	for _, flag := range []string{"name", "file"} {
		if err := cmd.MarkFlagRequired(flag); err != nil {
			panic(err)
		}
	}
	return cmd
}

func newScheduledTaskDeleteCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a schedule in the current project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			taskName := strings.TrimSpace(name)
			if taskName == "" {
				return errors.New("--name requires a non-empty schedule name")
			}
			projectDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve current directory: %w", err)
			}
			schedules, err := schedule.NewSchedule()
			if err != nil {
				return err
			}
			if err := schedules.Delete(projectDir, taskName); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Deleted schedule %q.\n", taskName)
			return err
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Existing schedule name")
	if err := cmd.MarkFlagRequired("name"); err != nil {
		panic(err)
	}
	return cmd
}

func createScheduledTask(cmd *cobra.Command, name, cron, file string) error {
	if name == "" || strings.TrimSpace(cron) == "" {
		return errors.New("--name and --cron require non-empty values")
	}
	if strings.TrimSpace(file) == "" {
		return errors.New("--file requires a non-empty file path")
	}
	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve current directory: %w", err)
	}
	if _, err := config.Load(factoryConfigFile); err != nil {
		return err
	}
	schedules, err := schedule.NewSchedule()
	if err != nil {
		return err
	}
	task := schedule.ScheduledTask{Name: name, Cron: cron, FilePrompt: file}
	if err := schedules.Create(projectDir, task); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Created schedule %q for %s.\n", name, projectDir)
	return err
}
