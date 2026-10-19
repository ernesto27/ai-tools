package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"soft-factory/internal/executionlog"
)

const workflowSecurityReport = `{"verdict":"UNRESOLVED","findings":[{"priority":"P1","location":"handler.go:42","trigger":"Untrusted input","impact":"Command execution","suggestedFix":"Avoid shell interpretation"}],"checks":"Inspected handler","gaps":"No live service"}`

func prepareSecurityWorkflow(t *testing.T) string {
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
	for name, script := range map[string]string{
		"agent-sandbox": "#!/bin/sh\nexec \"$FACTORY_WORKFLOW_EXE\" -test.run='^TestWorkflowSandboxHelper$' -- \"$@\"\n",
		"git":           "#!/bin/sh\ncase \"$*\" in\n *--git-path*) printf '%s/exclude\\n' \"$FACTORY_WORKFLOW_ROOT\";;\n *worktree*) printf 'worktree %s\\nbranch refs/heads/task\\n\\n' \"$FACTORY_WORKFLOW_ROOT\";;\nesac\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("FACTORY_WORKFLOW_EXE", exe)
	t.Setenv("FACTORY_WORKFLOW_ROOT", root)
	t.Setenv("FACTORY_WORKFLOW_HELPER", "1")
	t.Setenv("FACTORY_KEEP_TEST_CONFIG", "1")
	if err := os.WriteFile("task.txt", []byte("Original task"), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestSecurityReportingWorkflows(t *testing.T) {
	for _, tc := range []struct {
		name, command, save, disabled, status string
		wantCalls, wantEntries                int
		wantLog                               bool
	}{
		{"default", "", "true", "", "", 4, 1, true},
		{"continue", "continue", "true", "", "", 4, 1, true},
		{"review", "review", "true", "", "", 3, 1, true},
		{"review false", "review", "false", "", "", 3, 0, false},
		{"review disabled", "review", "true", `,"securityReview"`, "", 2, 0, false},
		{"default disabled", "", "true", `,"securityReview"`, "", 3, 0, true},
		{"resume failed security", "resume", "true", "", "securityReview,FAILED\n", 2, 1, true},
		{"resume successful security", "resume", "true", "", "securityReview,SUCCEEDED\n", 1, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := prepareSecurityWorkflow(t)
			cfg := `{"disabledStages":["reviewChanges"` + tc.disabled + `],"stages":{"securityReview":{"saveFindings":` + tc.save + `}},"agentSandbox":{"agent":"codex","branch":"task","push":true,"pr":true,"run":{"filePrompt":"task.txt"},"resume":{"push":true,"pr":true}}}`
			if err := os.WriteFile(factoryConfigFile, []byte(cfg), 0600); err != nil {
				t.Fatal(err)
			}
			var args []string
			if tc.command != "" {
				args = append(args, tc.command)
			}
			summaryPath := ""
			historical := "SOFTWARE FACTORY\n1. HISTORICAL ENTRY\n"
			if tc.command == "resume" {
				recovery := filepath.Join(root, "recovery")
				if err := os.Mkdir(recovery, 0700); err != nil {
					t.Fatal(err)
				}
				for name, content := range map[string]string{"summary.log": historical, executionlog.StageStatusFilename: tc.status, "codeReview.txt": "Prior code review evidence", "securityReview.txt": "Prior security evidence"} {
					if err := os.WriteFile(filepath.Join(recovery, name), []byte(content), 0600); err != nil {
						t.Fatal(err)
					}
				}
				summaryPath = filepath.Join(recovery, "summary.log")
				args = append(args, recovery)
			}
			cmd := newRootCmd(runWorkflow)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			f, err := os.Open("capture.jsonl")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			decoder := json.NewDecoder(f)
			var captures []workflowCapture
			for {
				var capture workflowCapture
				err := decoder.Decode(&capture)
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				captures = append(captures, capture)
			}
			if len(captures) != tc.wantCalls {
				t.Fatalf("calls=%d, want %d", len(captures), tc.wantCalls)
			}
			securityCalls := 0
			for _, capture := range captures {
				if strings.Contains(capture.Prompt, "This stage has saveFindings enabled.") {
					securityCalls++
					joined := strings.Join(capture.Args, "\n")
					if !strings.HasSuffix(joined, "--push=false\n--pr=false") {
						t.Fatalf("security controls: %s", joined)
					}
				}
			}
			if securityCalls != tc.wantEntries {
				t.Fatalf("security reporting attempts = %d, want %d", securityCalls, tc.wantEntries)
			}
			if !tc.wantLog {
				if _, err := os.Stat("logs"); !os.IsNotExist(err) {
					t.Fatalf("unexpected standalone execution logs: %v", err)
				}
				return
			}
			if summaryPath == "" {
				matches, err := filepath.Glob("logs/task/*/summary.log")
				if err != nil || len(matches) != 1 {
					t.Fatalf("summary paths = %v: %v", matches, err)
				}
				summaryPath = matches[0]
			}
			summary, err := os.ReadFile(summaryPath)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(summary), "SECURITY REVIEW") != tc.wantEntries {
				t.Fatalf("duplicate or missing security summary: %s", summary)
			}
			if tc.wantEntries > 0 {
				for _, want := range []string{"UNRESOLVED", "Avoid shell interpretation", "No code changes made (saveFindings enabled).", "RISK CLASSIFICATION", "SUCCEEDED"} {
					if !strings.Contains(string(summary), want) {
						t.Errorf("summary missing %q: %s", want, summary)
					}
				}
			}
			if tc.command == "review" && (strings.Contains(string(summary), "IMPLEMENTATION") || strings.Contains(string(summary), "REVIEW CHANGES")) {
				t.Fatalf("standalone review ran extra stages: %s", summary)
			}
			if tc.command == "resume" && !strings.HasPrefix(string(summary), historical) {
				t.Fatal("resume overwrote historical summary")
			}
		})
	}
}

func TestResumeStandaloneSecurityReview(t *testing.T) {
	root := prepareSecurityWorkflow(t)
	cfg := `{"stages":{"securityReview":{"saveFindings":true}},"agentSandbox":{"agent":"codex","branch":"task","run":{"filePrompt":"task.txt"}}}`
	if err := os.WriteFile(factoryConfigFile, []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
	// Use the real log constructor to persist the standalone workflow identity.
	run, err := executionlog.NewRun("task", executionlog.CommandReview, nil)
	if err != nil {
		t.Fatal(err)
	}
	path := run.Path()
	run.Finish(nil) // Empty status models interruption before the first result.
	cmd := newRootCmd(runWorkflow)
	cmd.SetArgs([]string{"resume", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(root, "capture.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	calls := 0
	for {
		var capture workflowCapture
		err := decoder.Decode(&capture)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		calls++
		if capture.Args[0] != "resume" || strings.Contains(capture.Prompt, "write the walkthrough") {
			t.Fatalf("unexpected standalone recovery stage: %+v", capture)
		}
	}
	if calls != 3 {
		t.Fatalf("standalone recovery ran %d stages, want only code review, security review, and risk classification", calls)
	}
	summary, err := os.ReadFile(filepath.Join(path, "summary.log"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(summary), "IMPLEMENTATION") || strings.Contains(string(summary), "REVIEW CHANGES") || !strings.Contains(string(summary), "UNRESOLVED") {
		t.Fatalf("standalone recovery summary: %s", summary)
	}
}
