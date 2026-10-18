package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"soft-factory/internal/executionlog"
)

type workflowCapture struct {
	Args              []string
	Prompt, Directory string
}

func TestWorkflowSandboxHelper(t *testing.T) {
	if os.Getenv("FACTORY_WORKFLOW_HELPER") != "1" {
		return
	}
	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	capture := workflowCapture{Args: args, Directory: cwd}
	for i, arg := range args {
		if arg == "-f" || arg == "--filePrompt" {
			data, err := os.ReadFile(args[i+1])
			if err != nil {
				t.Fatal(err)
			}
			capture.Prompt = string(data)
		}
	}
	file, err := os.OpenFile("capture.jsonl", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(file).Encode(capture); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range regexp.MustCompile(`/workspace/\.soft-factory-[A-Za-z0-9._-]+`).FindAllString(capture.Prompt, -1) {
		content := "Test stage changes.\n"
		if strings.HasSuffix(path, ".json") {
			content = `{"classification":"LOW"}`
		}
		if err := os.WriteFile(filepath.Base(path), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Later stages must keep using the original loaded configuration.
	if err := os.WriteFile(factoryConfigFile, []byte(`{invalid after launch`), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestUnifiedWorkflowEntryPoints(t *testing.T) {
	for _, command := range []string{"default", "continue", "review", "resume", "disabled"} {
		t.Run(command, func(t *testing.T) {
			// "disabled" runs the default workflow with reviewChanges disabled.
			disabled := command == "disabled"
			if disabled {
				command = "default"
			}
			root := t.TempDir()
			t.Chdir(root)
			bin := filepath.Join(root, "bin")
			if err := os.Mkdir(bin, 0700); err != nil {
				t.Fatal(err)
			}
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			scripts := map[string]string{
				"agent-sandbox": "#!/bin/sh\nexec \"$FACTORY_WORKFLOW_EXE\" -test.run='^TestWorkflowSandboxHelper$' -- \"$@\"\n",
				"git":           "#!/bin/sh\ncase \"$*\" in\n *--git-path*) printf '%s/exclude\\n' \"$FACTORY_WORKFLOW_ROOT\";;\n *worktree*) printf 'worktree %s\\nbranch refs/heads/new-task\\n\\nworktree %s\\nbranch refs/heads/old-task\\n\\n' \"$FACTORY_WORKFLOW_ROOT\" \"$FACTORY_WORKFLOW_ROOT\";;\nesac\n",
			}
			for name, script := range scripts {
				if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin)
			t.Setenv("FACTORY_WORKFLOW_EXE", exe)
			t.Setenv("FACTORY_WORKFLOW_ROOT", root)
			t.Setenv("FACTORY_WORKFLOW_HELPER", "1")
			cfg := `{"documents":["support.txt"],"customSkills":{"codeReview":"legacy-missing","securityReview":"legacy-missing","riskClassification":"legacy-missing","reviewChanges":"legacy-missing"},"stages":{"implementation":{"agent":"stage-agent","customSkill":42},"codeReview":{"model":"stage-model","customSkill":"project-code"},"securityReview":{"customSkill":"project-security"},"riskClassification":{"customSkill":"project-risk"},"reviewChanges":{"customSkill":"project-changes"}},"agentSandbox":{"agent":"shared-agent","model":"shared-model","baseImage":"go image","file":["shared mount"],"push":true,"pr":true,"run":{"agent":"run-agent","model":"run-model","branch":"new-task","filePrompt":"task path.txt","apiKey":"secret-run","reviewers":["secret-reviewer"]},"resume":{"agent":"resume-agent","model":"resume-model","branch":"old-task","filePrompt":"wrong-task.txt","query":"wrong task","push":false,"file":[],"apiKey":"secret-resume","reviewers":["secret-reviewer"]}}}`
			if disabled {
				cfg = strings.Replace(cfg, `{"documents"`, `{"disabledStages":["reviewChanges"],"documents"`, 1)
			}
			for name, content := range map[string]string{factoryConfigFile: cfg, "task path.txt": "original local task", "support.txt": "supporting context"} {
				if err := os.WriteFile(name, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			skillNames := []string{"project-code", "project-security", "project-risk", "project-changes"}
			for _, name := range skillNames {
				if disabled && name == "project-changes" {
					continue // a disabled stage must not load its skill file
				}
				path := filepath.Join(".agents", "skills", name, "SKILL.md")
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("Instructions for "+name), 0600); err != nil {
					t.Fatal(err)
				}
			}
			var args []string
			if command != "default" {
				args = []string{command}
			}
			if command == "resume" {
				path := filepath.Join(root, "recovery")
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				for name, content := range map[string]string{"summary.log": "SOFTWARE FACTORY\n", executionlog.StageStatusFilename: "implementation,FAILED\n"} {
					if err := os.WriteFile(filepath.Join(path, name), []byte(content), 0600); err != nil {
						t.Fatal(err)
					}
				}
				args = append(args, path)
			}
			cmd := newRootCmd(runWorkflow)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open("capture.jsonl")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			decoder := json.NewDecoder(file)
			count := 5
			if command == "review" {
				count = 3
			}
			if disabled {
				count = 4
			}
			for i := 0; i < count; i++ {
				var got workflowCapture
				if err := decoder.Decode(&got); err != nil {
					t.Fatal(err)
				}
				mode, branch, agent, model := "resume", "old-task", "resume-agent", "resume-model"
				stage := i
				if command == "review" {
					stage++
				}
				if command == "default" {
					branch = "new-task"
					if stage == 0 {
						mode, agent, model = "run", "run-agent", "run-model"
					}
				}
				if stage == 0 {
					agent = "stage-agent"
				}
				if stage == 1 {
					model = "stage-model"
				}
				if got.Args[0] != mode || got.Directory != root {
					t.Fatalf("invocation = %+v", got)
				}
				flag := func(long, short string) string {
					for i, arg := range got.Args {
						if arg == long || arg == short {
							return got.Args[i+1]
						}
					}
					return "<absent>"
				}
				if flag("--branch", "") != branch || flag("--agent", "-a") != agent || flag("--model", "-m") != model || flag("--baseImage", "") != "go image" {
					t.Fatalf("selection args = %q", got.Args)
				}
				if !strings.Contains(got.Prompt, "original local task") || !strings.Contains(got.Prompt, "supporting context") || strings.Contains(got.Prompt, "wrong task") {
					t.Fatal("task/context mismatch")
				}
				for skillStage, name := range skillNames {
					if got, want := strings.Contains(got.Prompt, "Instructions for "+name), stage == skillStage+1; got != want {
						t.Fatalf("stage %d: skill %q present = %v; want %v", stage, name, got, want)
					}
				}
				joined := strings.Join(got.Args, "\n")
				if strings.Contains(joined, "secret-") {
					t.Fatal("secret forwarded")
				}
				if mode == "resume" && flag("--file", "") != "<absent>" {
					t.Fatal("empty resume mount list did not clear shared mounts")
				}
				if stage == 3 && (!strings.Contains(joined, "--push=false") || !strings.Contains(joined, "--pr=false")) {
					t.Fatal("risk workflow controls lost")
				}
			}
			if decoder.More() {
				t.Fatal("unexpected extra invocation")
			}
			if _, err := os.Lstat("agent-sandbox.json"); !os.IsNotExist(err) {
				t.Fatal("sandbox config artifact created")
			}
			if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !entry.IsDir() && strings.HasSuffix(path, ".log") {
					data, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					if strings.Contains(string(data), "secret-") {
						t.Error("secret disclosed in log")
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
