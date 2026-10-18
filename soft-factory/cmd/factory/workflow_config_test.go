package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

type workflowCapture struct {
	Args       []string
	Config     json.RawMessage
	ConfigPath string
	Prompt     string
	WorkingDir string
	Mode       uint32
}

// Both subprocess boundaries are controlled; no Docker, agent, Git mutation,
// credentials, or external services are needed to execute the command tree.
func TestUnifiedWorkflowHelper(t *testing.T) {
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
	if len(args) < 2 {
		t.Fatal("missing helper arguments")
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if args[0] == "git" {
		switch strings.Join(args[1:], " ") {
		case "rev-parse --git-path info/exclude":
			fmt.Println(filepath.Join(wd, "test-exclude"))
		case "-C . worktree list --porcelain":
			fmt.Printf("worktree %s\nbranch refs/heads/%s\n\n", wd, os.Getenv("FACTORY_WORKFLOW_BRANCH"))
		case "rev-parse --show-prefix":
			fmt.Println()
		default:
			t.Fatalf("unexpected Git request: %q", args[1:])
		}
		return
	}
	args = args[1:]
	got := workflowCapture{Args: args, WorkingDir: wd}
	for i, arg := range args {
		if i+1 == len(args) {
			continue
		}
		switch arg {
		case "--config":
			got.ConfigPath = args[i+1]
			got.Config, err = os.ReadFile(got.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(got.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			got.Mode = uint32(info.Mode().Perm())
		case "-f":
			data, err := os.ReadFile(args[i+1])
			if err != nil {
				t.Fatal(err)
			}
			got.Prompt = string(data)
		}
	}
	if got.ConfigPath == "" {
		t.Fatal("sandbox did not receive explicit configuration")
	}
	file, err := os.OpenFile(os.Getenv("FACTORY_WORKFLOW_CAPTURE"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(file).Encode(got); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	// Make any accidental configuration reload in a later stage fail visibly.
	if err := os.WriteFile(factoryConfigFile, []byte(`{`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range regexp.MustCompile(`/workspace/\.soft-factory-[A-Za-z0-9._-]+`).FindAllString(got.Prompt, -1) {
		content := "No code changes made.\n"
		if strings.HasSuffix(path, ".json") {
			content = `{"classification":"LOW"}`
		}
		if err := os.WriteFile(filepath.Base(path), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWorkflowEntriesUseOneUnifiedSnapshot(t *testing.T) {
	for _, entry := range []string{"default", "continue", "review", "resume"} {
		t.Run(entry, func(t *testing.T) {
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
			t.Setenv("FACTORY_WORKFLOW_EXE", exe)
			t.Setenv("FACTORY_WORKFLOW_HELPER", "1")
			branch := "resume-branch"
			if entry == "default" {
				branch = "run-branch"
			}
			t.Setenv("FACTORY_WORKFLOW_BRANCH", branch)
			capturePath := filepath.Join(root, "invocations.jsonl")
			t.Setenv("FACTORY_WORKFLOW_CAPTURE", capturePath)
			for _, name := range []string{"git", "agent-sandbox"} {
				script := "#!/bin/sh\nexec \"$FACTORY_WORKFLOW_EXE\" -test.run='^TestUnifiedWorkflowHelper$' -- " + name + " \"$@\"\n"
				if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			const sandboxJSON = `{"agent":"codex","model":"shared","branch":"shared-branch","run":{"branch":"run-branch","filePrompt":"task.txt","model":"run-model","apiKey":"run-secret"},"resume":{"branch":"resume-branch","filePrompt":"missing-resume.txt","push":false,"model":"resume-model","apiKey":"resume-secret"}}`
			data := `{"documents":["context.txt"],"disabledStages":["reviewChanges"],"stages":{"implementation":{"agent":"claude"},"codeReview":{"model":"stage-model"}},"agentSandbox":` + sandboxJSON + `}`
			for name, content := range map[string]string{factoryConfigFile: data, "agent-sandbox.json": `{`, "task.txt": "original local task", "context.txt": "supporting document"} {
				if err := os.WriteFile(name, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{}
			if entry != "default" {
				args = []string{entry}
			}
			if entry == "resume" {
				path := filepath.Join(root, "recovery")
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				for name, content := range map[string]string{"summary.log": "earlier summary\n", "stage_status.txt": "implementation,FAILED\n"} {
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
			file, err := os.Open(capturePath)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			scanner := bufio.NewScanner(file)
			count := 0
			for scanner.Scan() {
				var got workflowCapture
				if err := json.Unmarshal(scanner.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if string(got.Config) != sandboxJSON || got.WorkingDir != root || got.Mode != 0600 {
					t.Fatalf("unexpected handoff for %s", entry)
				}
				if _, err := os.Stat(got.ConfigPath); !os.IsNotExist(err) {
					t.Fatalf("transport remains: %v", err)
				}
				if !strings.Contains(got.Prompt, "original local task") || !strings.Contains(got.Prompt, "supporting document") {
					t.Fatal("task context disagrees with loaded configuration")
				}
				index := slices.Index(got.Args, "--branch")
				if index < 0 || got.Args[index+1] != branch {
					t.Fatalf("branch override = %q", got.Args)
				}
				wantMode := "resume"
				if entry == "default" && count == 0 {
					wantMode = "run"
				}
				if got.Args[0] != wantMode {
					t.Fatalf("mode = %q; want %q", got.Args[0], wantMode)
				}
				if strings.Contains(strings.Join(got.Args, " "), "secret") {
					t.Fatal("secret in invocation arguments")
				}
				count++
			}
			if err := scanner.Err(); err != nil {
				t.Fatal(err)
			}
			wantCount := 4
			if entry == "review" {
				wantCount = 3
			}
			if count != wantCount {
				t.Fatalf("invocations = %d, want %d", count, wantCount)
			}
			if got, err := os.ReadFile("agent-sandbox.json"); err != nil || string(got) != "{" {
				t.Fatal("legacy file changed")
			}
		})
	}
}
