package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingSkillStopsBeforeImplementation(t *testing.T) {
	for _, stage := range []string{"codeReview", "securityReview", "riskClassification", "reviewChanges"} {
		t.Run(stage, func(t *testing.T) {
			t.Chdir(t.TempDir())
			data := `{"customSkills":{"` + stage + `":"missing"}}`
			if err := os.WriteFile(factoryConfigFile, []byte(data), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile("config.json", []byte(`{`), 0644); err != nil {
				t.Fatal(err)
			}

			err := runWorkflow(workflowOptions{Implement: true})
			want := "customSkills." + stage + `: project skill "missing" not found`
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("expected missing-skill error %q first, got %v", want, err)
			}
		})
	}
}

func prepareLoggedWorkflow(t *testing.T, settings string) {
	t.Helper()
	t.Chdir(t.TempDir())
	if output, err := exec.Command("git", "init", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("initialize test repository: %v\n%s", err, output)
	}
	for path, content := range map[string]string{
		factoryConfigFile:    `{"disabledStages":["implementation","codeReview","securityReview","riskClassification","reviewChanges"]}`,
		"agent-sandbox.json": settings,
	} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func captureWorkflow(t *testing.T, opts workflowOptions) (string, string) {
	t.Helper()
	stdout, stderr := os.Stdout, os.Stderr
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	warnings, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer warnings.Close()
	os.Stdout, os.Stderr = out, warnings
	defer func() { os.Stdout, os.Stderr = stdout, stderr }()
	if err := runWorkflow(opts); err != nil {
		t.Fatalf("workflow: %v", err)
	}
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	return read(out.Name()), read(warnings.Name())
}

func TestWorkflowExecutionLogLayout(t *testing.T) {
	for _, tc := range []struct {
		name     string
		settings string
		options  []workflowOptions
		parents  []string
	}{
		{
			"shared branch",
			`{"run":{"branch":"feature/logs"},"resume":{"branch":"feature/logs"}}`,
			[]workflowOptions{{Implement: true}, {Implement: true, Continue: true}, {Implement: true, Continue: true}},
			[]string{"feature-logs", "feature-logs", "feature-logs"},
		},
		{
			"continue first and distinct selected branches",
			`{"run":{"branch":"new/task"},"resume":{"branch":"old/task"}}`,
			[]workflowOptions{{Implement: true, Continue: true}, {Implement: true}},
			[]string{"old-task", "new-task"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepareLoggedWorkflow(t, tc.settings)
			paths := make(map[string]bool)
			for i, opts := range tc.options {
				output, warnings := captureWorkflow(t, opts)
				if warnings != "" {
					t.Fatalf("unexpected warnings: %s", warnings)
				}
				line := strings.SplitN(output, "\n", 2)[0]
				path := strings.TrimPrefix(line, "Execution logs: ")
				command := "run"
				if opts.Continue {
					command = "continue"
				}
				if filepath.Dir(path) != filepath.Join("logs", tc.parents[i]) || !strings.HasPrefix(filepath.Base(path), command+"-") {
					t.Fatalf("unexpected printed invocation path: %q", line)
				}
				if paths[path] {
					t.Fatalf("reused invocation path: %s", path)
				}
				paths[path] = true
				data, err := os.ReadFile(filepath.Join(path, "summary.log"))
				if err != nil || !strings.Contains(string(data), "Skipping disabled stage: implementation") {
					t.Fatalf("summary = %q, error = %v", data, err)
				}
			}
		})
	}
}

func TestWorkflowLoggingUnavailableAndReviewOnly(t *testing.T) {
	for _, opts := range []workflowOptions{{Implement: true}, {Implement: true, Continue: true}, {}} {
		name := "review"
		if opts.Implement {
			name = "run"
			if opts.Continue {
				name = "continue"
			}
		}
		t.Run(name, func(t *testing.T) {
			prepareLoggedWorkflow(t, `{"run":{"branch":"feature"},"resume":{"branch":"feature"}}`)
			if opts.Implement {
				if err := os.WriteFile("logs", []byte("logging blocked"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			output, warnings := captureWorkflow(t, opts)
			if strings.Contains(output, "Execution logs:") {
				t.Fatalf("unexpected execution log path: %s", output)
			}
			if opts.Implement {
				if !strings.Contains(warnings, "Warning: execution logging unavailable:") || !strings.Contains(warnings, "; continuing execution.") {
					t.Fatalf("expected logging warning, got %q", warnings)
				}
				data, err := os.ReadFile("logs")
				if err != nil || string(data) != "logging blocked" {
					t.Fatalf("existing logs file changed: %q, error = %v", data, err)
				}
			} else {
				if warnings != "" {
					t.Fatalf("unexpected review warning: %s", warnings)
				}
				if _, err := os.Stat("logs"); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("review created execution logs: %v", err)
				}
			}
		})
	}
}

func TestWorkflowDoesNotFallBackToLegacyConfig(t *testing.T) {
	for _, implement := range []bool{false, true} {
		for _, invalid := range []bool{false, true} {
			name := "review"
			if implement {
				name = "implementation"
			}
			if invalid {
				name += "/invalid"
			} else {
				name += "/missing"
			}
			t.Run(name, func(t *testing.T) {
				t.Chdir(t.TempDir())
				if err := os.WriteFile("config.json", []byte(`{"customSkills":{"codeReview":"missing"}}`), 0644); err != nil {
					t.Fatal(err)
				}
				if invalid {
					if err := os.WriteFile(factoryConfigFile, []byte(`{`), 0644); err != nil {
						t.Fatal(err)
					}
				}
				err := runWorkflow(workflowOptions{Implement: implement})
				if invalid {
					if err == nil || !strings.Contains(err.Error(), "parse factory configuration") {
						t.Fatalf("expected invalid factory configuration error, got %v", err)
					}
				} else if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), factoryConfigFile) {
					t.Fatalf("expected missing %s error, got %v", factoryConfigFile, err)
				}
			})
		}
	}
}
