package sandbox

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
	"soft-factory/skills"
)

type promptCapture struct {
	Config     json.RawMessage
	ConfigPath string
	ConfigMode uint32
	WorkingDir string
	Args       []string
	Prompt     string
}

// A controlled subprocess receives the same -f file as a real sandbox.
func TestPromptSandboxHelper(t *testing.T) {
	if os.Getenv("SOFT_FACTORY_PROMPT_HELPER") != "1" {
		return
	}
	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	capture := promptCapture{Args: args}
	capture.WorkingDir, _ = os.Getwd()
	for i, arg := range args {
		if arg == "--config" && i+1 < len(args) {
			capture.ConfigPath = args[i+1]
			data, err := os.ReadFile(capture.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			capture.Config = data
			info, err := os.Stat(capture.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			capture.ConfigMode = uint32(info.Mode().Perm())
		}
	}
	if capture.ConfigPath == "" {
		t.Fatal("no explicit configuration handoff")
	}
	for i, arg := range args {
		if arg == "-f" && i+1 < len(args) {
			text, err := os.ReadFile(args[i+1])
			if err != nil {
				t.Fatal(err)
			}
			capture.Prompt = string(text)
		}
	}
	data, err := json.Marshal(capture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("SOFT_FACTORY_CAPTURE"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("SOFT_FACTORY_PROMPT_WAIT") == "1" {
		time.Sleep(30 * time.Second)
	}
	if os.Getenv("SOFT_FACTORY_PROMPT_FAIL") == "1" {
		os.Exit(17)
	}
	// Emulate agent-authored reports in the host worktree, never /workspace itself.
	for _, path := range regexp.MustCompile(`/workspace/\.soft-factory-[A-Za-z0-9._-]+`).FindAllString(capture.Prompt, -1) {
		content := "No code changes made.\n"
		if strings.HasPrefix(filepath.Base(path), ".soft-factory-risk-result-") && strings.HasSuffix(path, ".json") {
			content = `{"classification":"LOW"}`
		}
		if err := os.WriteFile(filepath.Base(path), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func preparePromptSandbox(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)
	for _, args := range [][]string{{"init", "-b", "main"}, {"config", "user.name", "Test"}, {"config", "user.email", "test@example.com"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v\n%s", err, out)
		}
	}
	if err := os.WriteFile("tracked.txt", []byte("base\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "tracked.txt"}, {"commit", "-m", "initial"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v\n%s", err, out)
		}
	}
	if err := os.WriteFile("task.txt", []byte("file task {{literal}} 100%"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("software-factory.json", []byte(`{"agentSandbox":{"run":{"branch":"main","filePrompt":"task.txt"},"resume":{"branch":"main"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nexec \"$SOFT_FACTORY_TEST_EXE\" -test.run='^TestPromptSandboxHelper$' -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "agent-sandbox"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SOFT_FACTORY_TEST_EXE", exe)
	t.Setenv("SOFT_FACTORY_PROMPT_HELPER", "1")
	capture := filepath.Join(root, "capture.json")
	t.Setenv("SOFT_FACTORY_CAPTURE", capture)
	return capture
}

func readPromptCapture(t *testing.T, path string) promptCapture {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var capture promptCapture
	if err := json.Unmarshal(data, &capture); err != nil {
		t.Fatal(err)
	}
	if capture.ConfigMode != 0600 {
		t.Fatalf("configuration permissions = %o", capture.ConfigMode)
	}
	if _, err := os.Stat(capture.ConfigPath); !os.IsNotExist(err) {
		t.Fatalf("configuration transport remains: %v", err)
	}
	return capture
}

func TestStagePromptIntegration(t *testing.T) {
	// Read frozen expectations before entering temporary invocation repositories.
	baseline := map[string]string{}
	for _, name := range []string{"task.txt", "supporting-context.txt", "custom-skill.txt", "reviewer-scope.txt", "review.txt", "review-report.txt", "stage-changes.txt", "risk-classification.txt", "review-changes.txt", "no-review-reports.txt", "disabled-reviews.txt"} {
		data, err := os.ReadFile(filepath.Join("..", "prompts", "testdata", name+".golden"))
		if err != nil {
			t.Fatal(err)
		}
		baseline[name] = string(data)
	}
	frozen := func(name string, fields map[string]string) string {
		pairs := []string{}
		for key, value := range fields {
			marker := "VALUE_" + key + " {{literal}} 100%"
			pairs = append(pairs, strconv.Quote(marker), strconv.Quote(value), marker, value)
		}
		return strings.NewReplacer(pairs...).Replace(baseline[name])
	}
	bundledSkills := map[string]string{}
	for _, name := range []string{"code-review", "security-review", "risk-classification", "review-changes"} {
		data, err := skills.ReadFile(name + "/SKILL.md")
		if err != nil {
			t.Fatal(err)
		}
		bundledSkills[name] = string(data)
	}
	for _, stage := range []string{"implementation", "code-review", "security-review", "risk-classification", "review-changes"} {
		for _, logged := range []bool{false, true} {
			if stage == "review-changes" && !logged {
				continue
			} // Existing contract requires a log directory.
			for _, custom := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/log=%t/custom=%t", stage, logged, custom), func(t *testing.T) {
					capturePath := preparePromptSandbox(t)
					input := TaskContext{Config: loadPromptConfig(t), TaskOverride: "task {{literal}} <code> 100%", Documents: "document body", Branch: "main"}
					if logged {
						run, err := executionlog.NewRun("main", executionlog.CommandRun, nil)
						if err != nil {
							t.Fatal(err)
						}
						input.Log = run
						t.Cleanup(func() { run.Finish(nil) })
						if stage != "review-changes" {
							run.StartStage(map[string]config.Stage{
								"implementation":      config.StageImplementation,
								"code-review":         config.StageCodeReview,
								"security-review":     config.StageSecurityReview,
								"risk-classification": config.StageRiskClassification,
							}[stage])
						}
					}
					var project *ProjectSkill
					if custom {
						project = &ProjectSkill{Name: "project", Content: "project skill {{literal}} <code> 100%"}
					}
					input.CustomSkills = ProjectSkills{CodeReview: project, SecurityReview: project, RiskClassification: project, ReviewChanges: project}
					hostEvidence := ""
					if stage == "review-changes" {
						snapshot, err := changeEvidence(nil, "main")
						if err != nil {
							t.Fatal(err)
						}
						hostEvidence = snapshot.Text
					}
					var err error
					switch stage {
					case "implementation":
						err = Run(input)
					case "code-review":
						_, err = Review(input)
					case "security-review":
						_, err = SecurityReview(input)
					case "risk-classification":
						err = RiskClassification(input, nil, []config.Stage{config.StageCodeReview, config.StageSecurityReview})
					case "review-changes":
						err = ReviewChanges(input)
					}
					if err != nil {
						t.Fatal(err)
					}
					capture := readPromptCapture(t, capturePath)
					task := frozen("task.txt", map[string]string{"Task": input.TaskOverride}) + frozen("supporting-context.txt", map[string]string{"Documents": input.Documents})
					additional := ""
					if custom {
						scope := ""
						if stage == "code-review" || stage == "security-review" {
							scope = baseline["reviewer-scope.txt"]
						}
						additional = frozen("custom-skill.txt", map[string]string{"Name": project.Name, "Scope": scope, "Content": project.Content})
					}
					want := task
					got := capture.Prompt
					switch stage {
					case "code-review", "security-review":
						want = frozen("review.txt", map[string]string{"Skill": bundledSkills[stage], "AdditionalSkill": additional, "Task": task})
						path := regexp.MustCompile(`/workspace/\.soft-factory-stage-changes-[A-Za-z0-9]+-review\.txt`).FindString(capture.Prompt)
						if path == "" {
							t.Fatal("missing review report path")
						}
						want += frozen("review-report.txt", map[string]string{"ReportPath": path})
					case "risk-classification":
						evidence := baseline["no-review-reports.txt"] + frozen("disabled-reviews.txt", map[string]string{"Stages": "codeReview, securityReview"})
						want = frozen("risk-classification.txt", map[string]string{"ReportDirectory": "/workspace", "Skill": bundledSkills[stage], "AdditionalSkill": additional, "Task": task, "ReviewEvidence": evidence})
					case "review-changes":
						path := regexp.MustCompile(`/workspace/\.soft-factory-review-changes-[0-9]+\.md`).FindString(capture.Prompt)
						if path == "" {
							t.Fatal("missing walkthrough path")
						}
						want = frozen("review-changes.txt", map[string]string{"ReportPath": path, "Skill": bundledSkills[stage], "AdditionalSkill": additional, "Task": task, "HostEvidence": hostEvidence})
						report, err := os.ReadFile(filepath.Join(input.Log.Path(), "05-reviewChanges.md"))
						if err != nil || string(report) != "No code changes made.\n" {
							t.Fatalf("archive = %q, %v", report, err)
						}
					}
					if logged && stage == "risk-classification" {
						path := regexp.MustCompile(`/workspace/\.soft-factory-risk-result-[A-Za-z0-9]+\.json`).FindString(capture.Prompt)
						if path == "" {
							t.Fatal("missing structured risk result path")
						}
						if !strings.HasPrefix(capture.Prompt, want) {
							t.Fatal("risk prompt missing assessment instructions")
						}
						resultInstructions := strings.TrimPrefix(capture.Prompt, want)
						if !strings.Contains(resultInstructions, path) || !strings.Contains(resultInstructions, `{"classification":"LOW"}`) {
							t.Fatal("risk prompt missing structured result contract")
						}
						got = strings.TrimSuffix(capture.Prompt, resultInstructions)
					} else if logged && stage != "review-changes" {
						path := regexp.MustCompile(`/workspace/\.soft-factory-stage-changes-[A-Za-z0-9]+\.txt`).FindString(capture.Prompt)
						if path == "" {
							t.Fatal("missing stage summary path")
						}
						want += frozen("stage-changes.txt", map[string]string{"ReportPath": path})
					}
					if got != want {
						t.Fatalf("assembled prompt differs:\ngot %q\nwant %q", got, want)
					}
					mode := "resume"
					if stage == "implementation" {
						mode = "run"
					}
					wantArgs := []string{mode}
					if stage == "risk-classification" || stage == "review-changes" {
						wantArgs = append(wantArgs, "--push=false", "--pr=false")
					}
					promptPath := ""
					for i, arg := range capture.Args {
						if arg == "-f" {
							promptPath = capture.Args[i+1]
						}
					}
					wantArgs = append(wantArgs, "-f", promptPath, "--branch", "main", "--config", capture.ConfigPath)
					if strings.Join(capture.Args, "\x00") != strings.Join(wantArgs, "\x00") {
						t.Fatalf("args %q, want %q", capture.Args, wantArgs)
					}
					if _, err := os.Stat(promptPath); !os.IsNotExist(err) {
						t.Fatalf("temporary prompt was not removed: %v", err)
					}
					if logged && stage != "review-changes" {
						input.Log.FinishStage(nil)
						files, err := filepath.Glob(filepath.Join(input.Log.Path(), "*.log"))
						if err != nil {
							t.Fatal(err)
						}
						found := false
						for _, path := range files {
							text, err := os.ReadFile(path)
							if err != nil {
								t.Fatal(err)
							}
							if strings.Contains(string(text), capture.Prompt) {
								found = true
							}
						}
						if !found {
							t.Fatal("execution log did not contain the exact launched prompt")
						}
					}
				})
			}
		}
	}
}

func TestImplementationPromptModes(t *testing.T) {
	for _, continuing := range []bool{false, true} {
		t.Run(fmt.Sprintf("continue=%t", continuing), func(t *testing.T) {
			path := preparePromptSandbox(t)
			if err := Run(TaskContext{Config: loadPromptConfig(t), Branch: "main", Continue: continuing}); err != nil {
				t.Fatal(err)
			}
			got := readPromptCapture(t, path)
			want := ""
			if continuing {
				want = "file task {{literal}} 100%"
			}
			if got.Prompt != want {
				t.Fatalf("prompt %q, want %q", got.Prompt, want)
			}
			wantArgs := []string{"run", "--branch", "main"}
			if continuing {
				if len(got.Args) != 7 {
					t.Fatalf("args %q", got.Args)
				}
				wantArgs = []string{"resume", "-f", got.Args[2], "--branch", "main"}
			}
			wantArgs = append(wantArgs, "--config", got.ConfigPath)
			if strings.Join(got.Args, "\x00") != strings.Join(wantArgs, "\x00") {
				t.Fatalf("args %q, want %q", got.Args, wantArgs)
			}
		})
	}
}

func TestPromptCleanupAfterSandboxFailure(t *testing.T) {
	path := preparePromptSandbox(t)
	t.Setenv("SOFT_FACTORY_PROMPT_FAIL", "1")
	if _, err := Review(TaskContext{TaskOverride: "task", Branch: "main"}); err == nil {
		t.Fatal("expected sandbox error")
	}
	got := readPromptCapture(t, path)
	for i, arg := range got.Args {
		if arg == "-f" {
			if _, err := os.Stat(got.Args[i+1]); !os.IsNotExist(err) {
				t.Fatalf("prompt remains: %v", err)
			}
		}
	}
}

func loadPromptConfig(t *testing.T) config.Config {
	t.Helper()
	cfg, err := config.Load("software-factory.json")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
