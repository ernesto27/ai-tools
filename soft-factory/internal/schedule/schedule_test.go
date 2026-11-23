package schedule

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNormalizeCron(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"zero step", "*/0 * * * *", ""},
		{"step on singleton", "5/10 * * * *", ""},
		{"reversed range", "1-0 * * * *", ""},
		{"minute out of bounds", "60 * * * *", ""},
		{"full weekday range", "* * * * 0-7", "* * * * 0-7"},
		{"descriptor", "@daily", ""},
		{"six fields", "0 0 12 * * *", ""},
		{"embedded newline", "0 9 * *\n1-5", ""},
		{"normalize whitespace", " 0\t9 * * 1-5 ", "0 9 * * 1-5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeCron(tt.input)
			if tt.want == "" {
				if err == nil {
					t.Fatalf("normalizeCron(%q) accepted invalid input", tt.input)
				}
			} else if err != nil || got != tt.want {
				t.Fatalf("normalizeCron(%q) = %q, %v; want %q", tt.input, got, err, tt.want)
			}
		})
	}
}

func TestCronJobCapturesOutput(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "agent's bin")
	toolsDir := filepath.Join(dir, "other tools")
	for _, dir := range []string{agentDir, toolsDir} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	unusedDir := filepath.Join(dir, "unused-path-entry")
	t.Setenv("PATH", strings.Repeat(unusedDir+":", 24)+strings.Join([]string{dir, agentDir, toolsDir}, ":"))
	for executable, output := range map[string]string{
		filepath.Join(agentDir, "agent-sandbox"): "startup-output",
		filepath.Join(agentDir, "docker"):        "docker-output",
		filepath.Join(toolsDir, "git"):           "git-output",
		filepath.Join(toolsDir, "gh"):            "gh-output",
	} {
		if err := os.WriteFile(executable, []byte("#!/bin/sh\necho "+output+"\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	executable := filepath.Join(dir, "software-factory")
	const script = `#!/bin/sh
command -v sh >/dev/null || exit 1
agent-sandbox && docker && git && gh || exit 1
echo startup-error >&2
exit 23
`
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	const timing = "30 8 * * 1-5"
	outputPath := filepath.Join(dir, "cron output.log")
	runJob := func(repo string) int {
		t.Helper()
		tasks := ScheduledTasks{repo: {{Name: "weekday", Cron: timing, FilePrompt: "task.txt"}}}
		block, err := renderCronBlock(tasks, outputPath, true)
		if err != nil {
			t.Fatal(err)
		}
		line := strings.Split(block, "\n")[1]
		if len(line) >= 1000 || strings.Contains(line, unusedDir) {
			t.Fatalf("expected a short cron entry without unused directories, got %q", line)
		}
		command, ok := strings.CutPrefix(line, timing+" ")
		if !ok {
			t.Fatalf("unexpected cron entry %q", line)
		}
		cmd := exec.Command("/bin/sh", "-c", command)
		cmd.Env = []string{"PATH=/usr/bin:/bin"}
		output, err := cmd.CombinedOutput()
		var exitErr *exec.ExitError
		if len(output) != 0 || !errors.As(err, &exitErr) {
			t.Fatalf("cron job returned output %q and error %v, want a quiet failure", output, err)
		}
		return exitErr.ExitCode()
	}

	if status := runJob(dir); status != 23 {
		t.Fatalf("exit status = %d, want 23", status)
	}
	const missing = "missing project"
	runJob(filepath.Join(dir, missing))
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if output := string(data); !strings.HasPrefix(output, "startup-output\ndocker-output\ngit-output\ngh-output\nstartup-error\n") || !strings.Contains(output, missing) {
		t.Fatalf("cron output log = %q, want startup output followed by the cd error", output)
	}
}

func TestQuoteCronSearchPath(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	for _, executable := range []string{
		filepath.Join(first, "software-factory"),
		filepath.Join(first, "git"),
		filepath.Join(second, "git"),
		filepath.Join(second, "gh"),
	} {
		if err := os.WriteFile(executable, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	path := strings.Join([]string{"", first, "relative", ".", second, first, ""}, ":")
	got, err := quoteCronSearchPath(path, false)
	if want := fmt.Sprintf("'%s':'%s'", first, second); err != nil || got != want {
		t.Fatalf("quoteCronSearchPath() = %q, %v; want %q", got, err, want)
	}
	for _, invalid := range []string{"/bin%name", "/bin\rname", "/bin\nname", "/bin\x00name"} {
		if _, err := quoteCronPath(invalid); err == nil {
			t.Errorf("quoteCronPath() accepted %q", invalid)
		}
	}
	if _, err := quoteCronSearchPath(path, true); err == nil {
		t.Fatal("quoteCronSearchPath() accepted a PATH without agent-sandbox")
	}
}

func TestScheduledTaskValidate(t *testing.T) {
	tests := []struct {
		name    string
		task    ScheduledTask
		wantErr bool
	}{
		{
			name: "all required fields",
			task: ScheduledTask{Name: "weekday", FilePrompt: "task.txt"},
		},
		{
			name:    "missing name",
			task:    ScheduledTask{FilePrompt: "task.txt"},
			wantErr: true,
		},
		{
			name:    "missing file prompt",
			task:    ScheduledTask{Name: "weekday"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.task.validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("validate() = %v, want error: %t", err, tt.wantErr)
			}
		})
	}
}

func TestScheduleChangesInstallCrontab(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("PATH", dir)
	const fakeCrontab = `#!/bin/sh
dir=${0%/*}
case "$1" in
  -l)
    if [ -f "$dir/table" ]; then
      /bin/cat "$dir/table"
    else
      echo "no crontab for $(/usr/bin/id -un)" >&2
      exit 1
    fi
    ;;
  -)
    if [ -f "$dir/fail-install" ]; then
      echo "installation failed" >&2
      exit 1
    fi
    /bin/cat > "$dir/table"
    ;;
  *) exit 2 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "crontab"), []byte(fakeCrontab), 0o700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(dir, "software-factory")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agent-sandbox"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	repoPath, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	schedule, err := NewSchedule()
	if err != nil {
		t.Fatal(err)
	}
	task := ScheduledTask{
		Name:       "weekday",
		FilePrompt: "task.txt",
		Cron:       "30 8 * * 1-5",
	}
	if err := schedule.Create(repoPath, task); err != nil {
		t.Fatal(err)
	}

	taskPath := filepath.Join(repoPath, task.FilePrompt)
	outputPath := filepath.Join(filepath.Dir(schedule.filePath), "cron-output.log")
	firstJob := fmt.Sprintf("30 8 * * 1-5 (cd '%s' && PATH='%s'\"${PATH:+:$PATH}\" '%s' --file '%s') >> '%s' 2>&1\n", repoPath, dir, executable, taskPath, outputPath)
	tablePath := filepath.Join(dir, "table")
	assertCrontab := func(want string) {
		t.Helper()
		data, err := os.ReadFile(tablePath)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("installed crontab = %q, want %q", data, want)
		}
	}
	firstTable := cronBegin + "\n" + firstJob + cronEnd + "\n"
	assertCrontab(firstTable)

	const unrelatedJob = "15 2 * * * /usr/local/bin/backup\n"
	if err := os.WriteFile(tablePath, []byte(firstTable+unrelatedJob), 0o600); err != nil {
		t.Fatal(err)
	}
	secondTask := task
	secondTask.Name = "weekend"
	secondTask.Cron = "0 18 * * 0"
	if err := schedule.Create(repoPath, secondTask); err != nil {
		t.Fatal(err)
	}
	secondJob := fmt.Sprintf("0 18 * * 0 (cd '%s' && PATH='%s'\"${PATH:+:$PATH}\" '%s' --file '%s') >> '%s' 2>&1\n", repoPath, dir, executable, taskPath, outputPath)
	secondTable := cronBegin + "\n" + firstJob + secondJob + cronEnd + "\n" + unrelatedJob
	assertCrontab(secondTable)

	editedTask := task
	editedTask.Cron = "0 9 * * 1-5"
	editedTask.FilePrompt = "updated task.txt"
	if err := schedule.Edit(repoPath, editedTask); err != nil {
		t.Fatal(err)
	}
	editedJob := fmt.Sprintf("0 9 * * 1-5 (cd '%s' && PATH='%s'\"${PATH:+:$PATH}\" '%s' --file '%s') >> '%s' 2>&1\n", repoPath, dir, executable, filepath.Join(repoPath, editedTask.FilePrompt), outputPath)
	editedTable := cronBegin + "\n" + editedJob + secondJob + cronEnd + "\n" + unrelatedJob
	assertCrontab(editedTable)

	if err := schedule.Delete(repoPath, secondTask.Name); err != nil {
		t.Fatal(err)
	}
	afterDelete := cronBegin + "\n" + editedJob + cronEnd + "\n" + unrelatedJob
	assertCrontab(afterDelete)

	if err := os.WriteFile(filepath.Join(dir, "fail-install"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	pendingTask := task
	pendingTask.Name = "pending"
	if err := schedule.Create(repoPath, pendingTask); err == nil || !strings.Contains(err.Error(), "saved to JSON, but crontab installation is pending") {
		t.Fatalf("Create() = %v, want a pending installation error", err)
	}
	assertCrontab(afterDelete)

	tasks, err := schedule.load()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := tasks[repoPath], []ScheduledTask{editedTask, pendingTask}; !slices.Equal(got, want) {
		t.Fatalf("tasks for %q = %+v, want %+v", repoPath, got, want)
	}
}

func TestScheduleEdit(t *testing.T) {
	repoPath := t.TempDir()
	schedule := Schedule{filePath: filepath.Join(repoPath, "cron.json")}
	original := ScheduledTask{Name: "weekday", FilePrompt: "task.txt"}
	other := ScheduledTask{Name: "weekend", FilePrompt: "task.txt"}
	if err := schedule.save(ScheduledTasks{repoPath: {original, other}}); err != nil {
		t.Fatal(err)
	}

	updated := original
	updated.FilePrompt = "updated.txt"
	if err := schedule.Edit(repoPath, updated); err != nil {
		t.Fatal(err)
	}
	invalid := updated
	invalid.FilePrompt = ""
	if err := schedule.Edit(repoPath, invalid); err == nil {
		t.Fatal("Edit accepted a task with an empty file prompt")
	}

	tasks, err := schedule.load()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := tasks[repoPath], []ScheduledTask{updated, other}; !slices.Equal(got, want) {
		t.Fatalf("tasks for %q = %+v, want %+v", repoPath, got, want)
	}
}

func TestScheduleDelete(t *testing.T) {
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")
	otherRepoPath := filepath.Join(dir, "other-repo")
	schedule := Schedule{filePath: filepath.Join(dir, "cron.json")}
	weekday := ScheduledTask{Name: "weekday", FilePrompt: "task.txt"}
	weekend := ScheduledTask{Name: "weekend", FilePrompt: "task.txt"}
	if err := schedule.save(ScheduledTasks{repoPath: {weekday, weekend}, otherRepoPath: {weekday}}); err != nil {
		t.Fatal(err)
	}

	if err := schedule.Delete(repoPath, weekday.Name); err != nil {
		t.Fatal(err)
	}
	if err := schedule.Delete(repoPath, weekday.Name); err == nil {
		t.Fatal("Delete accepted a task that was already removed")
	}
	tasks, err := schedule.load()
	if err != nil {
		t.Fatal(err)
	}
	if got := tasks[repoPath]; !slices.Equal(got, []ScheduledTask{weekend}) {
		t.Fatalf("tasks for %q = %+v, want only %+v", repoPath, got, weekend)
	}

	if err := schedule.Delete(repoPath, weekend.Name); err != nil {
		t.Fatal(err)
	}
	tasks, err = schedule.load()
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := tasks[repoPath]; exists {
		t.Fatalf("repository %q still exists after its last task was deleted", repoPath)
	}
	if got := tasks[otherRepoPath]; !slices.Equal(got, []ScheduledTask{weekday}) {
		t.Fatalf("tasks for %q = %+v, want only %+v", otherRepoPath, got, weekday)
	}
}
