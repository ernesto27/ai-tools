package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"soft-factory/internal/executionlog"
)

type workflowCapture struct {
	Args      []string
	Settings  json.RawMessage
	Prompt    string
	Directory string
	Mode      uint32
}

func TestUnifiedWorkflowHelper(t *testing.T) {
	if os.Getenv("FACTORY_WORKFLOW_HELPER") != "1" {
		return
	}
	args := os.Args[slices.Index(os.Args, "--")+1:]
	data, err := os.ReadFile("agent-sandbox.json")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat("agent-sandbox.json")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	capture := workflowCapture{Args: args, Settings: data, Directory: dir, Mode: uint32(info.Mode().Perm())}
	for i, arg := range args {
		if arg == "-f" {
			prompt, err := os.ReadFile(args[i+1])
			if err != nil {
				t.Fatal(err)
			}
			capture.Prompt = string(prompt)
		}
	}
	encoded, err := json.Marshal(capture)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile("captures.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(append(encoded, '\n')); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("FACTORY_WORKFLOW_REPLACE") == "1" {
		if err := os.WriteFile("replacement.json", []byte("unexpected replacement"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename("replacement.json", "agent-sandbox.json"); err != nil {
			t.Fatal(err)
		}
	}
	if os.Getenv("FACTORY_WORKFLOW_FAIL") == "1" {
		fmt.Fprintln(os.Stderr, "upstream sandbox validation failed")
		os.Exit(17)
	}
	// Mutating the maintained config during the first child must not affect the
	// branch, task path, stage overrides, logs, or transport used by later stages.
	if err := os.WriteFile(factoryConfigFile, []byte(`{"agentSandbox":{"agent":"mutated","run":{"filePrompt":"missing.txt"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, report := range regexp.MustCompile(`/workspace/\.soft-factory-[A-Za-z0-9._-]+`).FindAllString(capture.Prompt, -1) {
		content := "No code changes made.\n"
		if strings.Contains(report, "risk-result") {
			content = `{"classification":"LOW"}`
		}
		if err := os.WriteFile(filepath.Base(report), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func prepareUnifiedWorkflow(t *testing.T) string {
	t.Helper()
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
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	script := "#!/bin/sh\nexec \"$FACTORY_WORKFLOW_EXE\" -test.run='^TestUnifiedWorkflowHelper$' -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "agent-sandbox"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	// Only emulate read-only discovery. Exclusions go into a private fixture file;
	// this fixture never changes a real repository's Git administration files.
	git := `#!/bin/sh
case "$*" in
 'rev-parse --git-path info/exclude') echo "$PWD/test-exclude";;
 *'worktree list --porcelain') printf 'worktree %s\nbranch refs/heads/run-branch\n\nworktree %s\nbranch refs/heads/resume-branch\n\n' "$PWD" "$PWD";;
 *'symbolic-ref'*|*'rev-parse --verify'*) exit 1;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(git), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("task.txt", []byte("original local task"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("support.txt", []byte("supporting document"), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestUnifiedWorkflowEntryPoints(t *testing.T) {
	const embedded = `{"agent":"codex","model":"shared-model","branch":"shared-branch","push":true,"pr":true,"run":{"branch":"run-branch","filePrompt":"task.txt","apiKey":"secret-value","image":[],"model":"run-model"},"resume":{"branch":"resume-branch","push":false,"pr":false,"model":"resume-model","filePrompt":"wrong-task.txt","reviewers":["reviewer"]},"unknown":null}`
	const factory = `{"documents":["support.txt"],"stages":{"implementation":{"agent":"claude"},"codeReview":{"model":"stage-model"}},"agentSandbox":` + embedded + `}`
	for _, command := range []string{"run", "continue", "review", "resume"} {
		t.Run(command, func(t *testing.T) {
			root := prepareUnifiedWorkflow(t)
			// Malformed input is moved without parsing and restored verbatim.
			original := "{malformed conflicting legacy\n"
			if err := os.WriteFile("agent-sandbox.json", []byte(original), 0640); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(factoryConfigFile, []byte(factory), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{}
			if command != "run" {
				args = append(args, command)
			}
			if command == "resume" {
				log, err := executionlog.NewRun("resume-branch", executionlog.CommandContinue, nil)
				if err != nil {
					t.Fatal(err)
				}
				log.Finish(nil)
				if err := os.WriteFile(filepath.Join(log.Path(), executionlog.StageStatusFilename), []byte("implementation,FAILED\n"), 0600); err != nil {
					t.Fatal(err)
				}
				args = append(args, log.Path())
			}
			cmd := newRootCmd(runWorkflow)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			captures, err := os.ReadFile("captures.jsonl")
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(captures)), "\n")
			wantCount := 5
			if command == "review" {
				wantCount = 3
			}
			if len(lines) != wantCount {
				t.Fatalf("children = %d, want %d", len(lines), wantCount)
			}
			for i, line := range lines {
				var capture workflowCapture
				if err := json.Unmarshal([]byte(line), &capture); err != nil {
					t.Fatal(err)
				}
				if string(capture.Settings) != embedded {
					t.Fatalf("child %d settings changed: %s", i, capture.Settings)
				}
				if capture.Directory != root || capture.Mode != 0600 {
					t.Fatalf("child context = %q, %o", capture.Directory, capture.Mode)
				}
				mode := "resume"
				if command == "run" && i == 0 {
					mode = "run"
				}
				if capture.Args[0] != mode {
					t.Fatalf("child mode = %q", capture.Args)
				}
				branch := "resume-branch"
				if command == "run" {
					branch = "run-branch"
				}
				at := slices.Index(capture.Args, "--branch")
				if at < 0 || capture.Args[at+1] != branch {
					t.Fatalf("child branch = %q", capture.Args)
				}
				if strings.Contains(strings.Join(capture.Args, " "), "secret-value") || slices.Contains(capture.Args, "--config") {
					t.Fatalf("unexpected child arguments: %q", capture.Args)
				}
				if !strings.Contains(capture.Prompt, "original local task") || !strings.Contains(capture.Prompt, "supporting document") {
					t.Fatalf("child task context = %q", capture.Prompt)
				}
				if i == 0 && command != "review" {
					if at := slices.Index(capture.Args, "-a"); at < 0 || capture.Args[at+1] != "claude" || slices.Contains(capture.Args, "-m") {
						t.Fatalf("independent agent override lost: %q", capture.Args)
					}
				}
				codeIndex := 1
				if command == "review" {
					codeIndex = 0
				}
				if i == codeIndex {
					if at := slices.Index(capture.Args, "-m"); at < 0 || capture.Args[at+1] != "stage-model" || slices.Contains(capture.Args, "-a") {
						t.Fatalf("independent model override lost: %q", capture.Args)
					}
				}
				if i == wantCount-1 || (command != "review" && i == 3) {
					if !slices.Contains(capture.Args, "--push=false") || !slices.Contains(capture.Args, "--pr=false") {
						t.Fatalf("workflow controls lost: %q", capture.Args)
					}
				}
			}
			restored, err := os.ReadFile("agent-sandbox.json")
			if err != nil || string(restored) != original {
				t.Fatalf("restore = %q, %v", restored, err)
			}
			info, err := os.Stat("agent-sandbox.json")
			if err != nil || info.Mode().Perm() != 0640 {
				t.Fatalf("restored mode = %v, %v", info, err)
			}
			if _, err := os.Stat(".soft-factory-sandbox-runtime"); !os.IsNotExist(err) {
				t.Fatalf("runtime lock remains: %v", err)
			}
			if command != "review" {
				found := false
				if err := filepath.WalkDir("logs", func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if d.IsDir() {
						return nil
					}
					text, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					if strings.Contains(string(text), "secret-value") || strings.Contains(string(text), "mutated") {
						t.Errorf("configuration leaked or snapshot changed in %s", path)
					}
					if strings.Contains(string(text), "software-factory.json: stages.implementation.agent") {
						found = true
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				if !found {
					t.Fatal("stage selection provenance missing")
				}
			}
		})
	}
}

func TestUnifiedWorkflowFailureCleanup(t *testing.T) {
	for _, setup := range []bool{false, true} {
		t.Run(fmt.Sprintf("setup=%t", setup), func(t *testing.T) {
			prepareUnifiedWorkflow(t)
			t.Setenv("FACTORY_WORKFLOW_FAIL", "1")
			data := `{"agentSandbox":{"agent":"invalid-agent","run":{"filePrompt":"task.txt"},"resume":{"branch":"resume-branch"}}}`
			if setup {
				data = `{"customSkills":{"codeReview":"missing"},"agentSandbox":{}}`
			}
			if err := os.WriteFile(factoryConfigFile, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile("agent-sandbox.json", []byte("original"), 0640); err != nil {
				t.Fatal(err)
			}
			err := runWorkflow(workflowOptions{})
			if err == nil {
				t.Fatal("expected failure")
			}
			if !setup && !strings.Contains(err.Error(), "execute agent-sandbox") {
				t.Fatalf("downstream failure not propagated: %v", err)
			}
			restored, err := os.ReadFile("agent-sandbox.json")
			if err != nil || string(restored) != "original" {
				t.Fatalf("restore = %q, %v", restored, err)
			}
			if _, err := os.Stat(".soft-factory-sandbox-runtime"); !os.IsNotExist(err) {
				t.Fatalf("runtime remains: %v", err)
			}
		})
	}
}

func TestUnifiedWorkflowReportsExecutionAndCleanupFailure(t *testing.T) {
	prepareUnifiedWorkflow(t)
	t.Setenv("FACTORY_WORKFLOW_FAIL", "1")
	t.Setenv("FACTORY_WORKFLOW_REPLACE", "1")
	if err := os.WriteFile(factoryConfigFile, []byte(`{"agentSandbox":{"run":{"filePrompt":"task.txt"},"resume":{"branch":"resume-branch"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("agent-sandbox.json", []byte("original"), 0640); err != nil {
		t.Fatal(err)
	}
	err := runWorkflow(workflowOptions{})
	if err == nil || !strings.Contains(err.Error(), "execute agent-sandbox") || !strings.Contains(err.Error(), "clean up sandbox configuration") {
		t.Fatalf("combined failure = %v", err)
	}
	for path, want := range map[string]string{"agent-sandbox.json": "unexpected replacement", ".soft-factory-sandbox-runtime/original.json": "original"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatalf("preserved %s = %q, %v", path, data, err)
		}
	}
}

func TestUnifiedWorkflowEmptySandboxTransport(t *testing.T) {
	for _, sandboxProperty := range []string{"", `,"agentSandbox":null`} {
		t.Run(sandboxProperty, func(t *testing.T) {
			prepareUnifiedWorkflow(t)
			data := `{"disabledStages":["codeReview","securityReview","riskClassification","reviewChanges"]` + sandboxProperty + `}`
			if err := os.WriteFile(factoryConfigFile, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			// No configured branch means execution logging is unavailable. Factory must
			// still send empty settings downstream and leave command requirements there.
			if err := runWorkflow(workflowOptions{Implement: true}); err != nil {
				t.Fatal(err)
			}
			dataBytes, err := os.ReadFile("captures.jsonl")
			if err != nil {
				t.Fatal(err)
			}
			var capture workflowCapture
			if err := json.Unmarshal(dataBytes, &capture); err != nil {
				t.Fatal(err)
			}
			if string(capture.Settings) != "{}" {
				t.Fatalf("empty transport = %s", capture.Settings)
			}
			at := slices.Index(capture.Args, "--branch")
			if at < 0 || capture.Args[at+1] != "" {
				t.Fatalf("empty selected branch = %q", capture.Args)
			}
			if _, err := os.Stat("agent-sandbox.json"); !os.IsNotExist(err) {
				t.Fatalf("generated file remains: %v", err)
			}
		})
	}
}
