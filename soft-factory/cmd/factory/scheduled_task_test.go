package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"soft-factory/internal/schedule"
)

func TestScheduledTaskCreateEditAndDelete(t *testing.T) {
	const cfg = `{"agentSandbox":{"run":{"filePrompt":["default.txt"]}}}`
	const prompt = "cron task.txt"
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(dir, "project")
	if err := os.Mkdir(project, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, factoryConfigFile), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Chdir(project)
	t.Setenv("HOME", dir)
	t.Setenv("PATH", dir)
	const fakeCrontab = `#!/bin/sh
table=${0%/*}/table
case "$1" in
  -l)
    if [ -f "$table" ]; then
      /bin/cat "$table"
    else
      printf '%s\n' '15 2 * * * /usr/local/bin/backup'
    fi
    ;;
  -) /bin/cat > "$table" ;;
  *) exit 2 ;;
esac
`
	for name, contents := range map[string]string{
		"crontab":          fakeCrontab,
		"software-factory": "#!/bin/sh\nexit 0\n",
		"agent-sandbox":    "#!/bin/sh\nexit 0\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o700); err != nil {
			t.Fatal(err)
		}
	}

	var output bytes.Buffer
	cmd := newRootCmd(func(workflowOptions) error {
		t.Fatal("create invoked the workflow")
		return nil
	})
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"scheduled-task", "create", "--name", "weekday-morning", "--cron", "30 8 * * 1-5", "--file", prompt})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `Created schedule "weekday-morning"`) {
		t.Fatalf("unexpected output: %s", &output)
	}

	table, err := os.ReadFile(filepath.Join(dir, "table"))
	if err != nil {
		t.Fatal(err)
	}
	scheduleDir := filepath.Join(dir, ".config", "software-factory", "scheduled-tasks")
	wantJob := fmt.Sprintf("30 8 * * 1-5 (cd '%s' && PATH='%s'\"${PATH:+:$PATH}\" '%s' --file '%s') >> '%s' 2>&1\n",
		project, dir, filepath.Join(dir, "software-factory"), filepath.Join(project, prompt), filepath.Join(scheduleDir, "cron-output.log"))
	wantTable := "15 2 * * * /usr/local/bin/backup\n# BEGIN SOFT FACTORY\n" + wantJob + "# END SOFT FACTORY\n"
	if string(table) != wantTable {
		t.Fatalf("installed crontab = %q, want %q", table, wantTable)
	}

	const updatedPrompt = "updated cron task.txt"
	output.Reset()
	edit := newRootCmd(func(workflowOptions) error {
		t.Fatal("edit invoked the workflow")
		return nil
	})
	edit.SetOut(&output)
	edit.SetArgs([]string{"scheduled-task", "edit", "--name", "weekday-morning", "--file", updatedPrompt})
	if err := edit.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `Updated schedule "weekday-morning"`) {
		t.Fatalf("unexpected output: %s", &output)
	}
	table, err = os.ReadFile(filepath.Join(dir, "table"))
	if err != nil {
		t.Fatal(err)
	}
	wantTable = strings.Replace(wantTable, filepath.Join(project, prompt), filepath.Join(project, updatedPrompt), 1)
	if string(table) != wantTable {
		t.Fatalf("edited crontab = %q, want %q", table, wantTable)
	}
	data, err := os.ReadFile(filepath.Join(scheduleDir, "cron.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tasks schedule.ScheduledTasks
	if err := json.Unmarshal(data, &tasks); err != nil {
		t.Fatal(err)
	}
	wantTask := schedule.ScheduledTask{Name: "weekday-morning", Cron: "30 8 * * 1-5", FilePrompt: updatedPrompt}
	if got := tasks[project]; len(got) != 1 || got[0] != wantTask {
		t.Fatalf("saved tasks = %+v, want [%+v]", got, wantTask)
	}

	output.Reset()
	deleteCmd := newRootCmd(func(workflowOptions) error {
		t.Fatal("delete invoked the workflow")
		return nil
	})
	deleteCmd.SetOut(&output)
	deleteCmd.SetArgs([]string{"scheduled-task", "delete", "--name", "weekday-morning"})
	if err := deleteCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `Deleted schedule "weekday-morning"`) {
		t.Fatalf("unexpected output: %s", &output)
	}
	table, err = os.ReadFile(filepath.Join(dir, "table"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "15 2 * * * /usr/local/bin/backup\n"; string(table) != want {
		t.Fatalf("crontab after delete = %q, want %q", table, want)
	}
}

func TestScheduledTaskCreateRequiresFlags(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"missing name", []string{"--cron", "30 8 * * 1-5", "--file", "task.txt"}, `required flag(s) "name" not set`},
		{"missing cron", []string{"--name", "weekday-morning", "--file", "task.txt"}, `required flag(s) "cron" not set`},
		{"missing file", []string{"--name", "weekday-morning", "--cron", "30 8 * * 1-5"}, `required flag(s) "file" not set`},
		{"blank name", []string{"--name", " ", "--cron", "30 8 * * 1-5", "--file", "task.txt"}, "require non-empty values"},
		{"blank file", []string{"--name", "weekday-morning", "--cron", "30 8 * * 1-5", "--file", " "}, "--file requires a non-empty file path"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := newRootCmd(func(workflowOptions) error {
				t.Fatal("create invoked the workflow")
				return nil
			})
			cmd.SetArgs(append([]string{"scheduled-task", "create"}, test.args...))
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Execute() = %v, want a message containing %q", err, test.want)
			}
		})
	}
}

func TestScheduledTaskList(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("PATH", dir)
	t.Chdir(dir)

	registryDir := filepath.Join(dir, ".config", "software-factory", "scheduled-tasks")
	if err := os.MkdirAll(registryDir, 0o700); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "project with spaces")
	tasks := schedule.ScheduledTasks{
		repo: {
			{Name: "pending", FilePrompt: "task.txt", Cron: "30 8 * * 1-5"},
			{Name: "legacy", FilePrompt: "old.txt"},
		},
		filepath.Join(dir, "other"): {{Name: "other", FilePrompt: "other.txt", Cron: "0 9 * * *"}},
	}
	data, err := json.Marshal(tasks)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(registryDir, "cron.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	const wantSchedules = `[{"name":"pending","filePrompt":"task.txt","cron":"30 8 * * 1-5"},{"name":"legacy","filePrompt":"old.txt"}]`

	for _, test := range []struct {
		name       string
		args       []string
		wantOutput string
		wantErr    string
	}{
		{name: "absolute", args: []string{"--path", repo}, wantOutput: wantSchedules},
		{name: "relative", args: []string{"--path", "project with spaces/"}, wantOutput: wantSchedules},
		{name: "no schedules", args: []string{"--path", "unknown"}, wantOutput: "[]"},
		{name: "missing path", wantErr: `required flag(s) "path" not set`},
		{name: "blank path", args: []string{"--path", " "}, wantErr: "--path requires a non-empty repository path"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			cmd := newRootCmd(func(workflowOptions) error {
				t.Fatal("list invoked the workflow")
				return nil
			})
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs(append([]string{"scheduled-task", "list"}, test.args...))
			err := cmd.Execute()
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("Execute() = %v, want a message containing %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := output.String(); got != test.wantOutput+"\n" {
				t.Fatalf("list output = %q, want %q", got, test.wantOutput+"\n")
			}
		})
	}
}
