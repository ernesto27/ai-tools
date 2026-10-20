package sandbox

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
	"soft-factory/skills"
)

type promptCapture struct {
	SecretInEnvironment bool
	Directory           string
	Args                []string
	Prompt              string
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
	directory, err := os.Getwd()
	require.NoError(t, err)
	capture := promptCapture{Args: args, Directory: directory}
	for _, value := range os.Environ() {
		if strings.Contains(value, "secret-run") || strings.Contains(value, "secret-resume") || strings.Contains(value, "secret-reviewer") {
			capture.SecretInEnvironment = true
		}
	}
	var interrupted chan os.Signal
	if os.Getenv("SOFT_FACTORY_PROMPT_WAIT") == "1" {
		interrupted = make(chan os.Signal, 1)
		signal.Notify(interrupted, os.Interrupt)
	}
	for i, arg := range args {
		if (arg == "-f" || arg == "--filePrompt") && i+1 < len(args) && os.Getenv("SOFT_FACTORY_ARGS_ONLY") != "1" {
			text, err := os.ReadFile(args[i+1])
			require.NoError(t, err)
			capture.Prompt = string(text)
		}
	}
	data, err := json.Marshal(capture)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(os.Getenv("SOFT_FACTORY_CAPTURE"), data, 0600))
	if interrupted != nil {
		<-interrupted
		os.Exit(130)
	}
	if os.Getenv("SOFT_FACTORY_PROMPT_FAIL") == "1" {
		os.Exit(17)
	}
	// Emulate agent-authored reports in the host worktree, never /workspace itself.
	for _, path := range regexp.MustCompile(`/workspace/\.soft-factory-[A-Za-z0-9._-]+`).FindAllString(capture.Prompt, -1) {
		content := "No code changes made.\n"
		if strings.HasSuffix(path, "-review.txt") && os.Getenv("SOFT_FACTORY_SECURITY_REPORT") != "" {
			content = os.Getenv("SOFT_FACTORY_SECURITY_REPORT")
		}
		if strings.HasPrefix(filepath.Base(path), ".soft-factory-risk-result-") && strings.HasSuffix(path, ".json") {
			content = `{"classification":"LOW"}`
		}
		require.NoError(t, os.WriteFile(filepath.Base(path), []byte(content), 0600))
	}
}

func preparePromptSandbox(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("TMPDIR", t.TempDir())
	for _, args := range [][]string{{"init", "-b", "main"}, {"config", "user.name", "Test"}, {"config", "user.email", "test@example.com"}} {
		out, err := exec.Command("git", args...).CombinedOutput()
		require.NoError(t, err, "%s", out)
	}
	require.NoError(t, os.WriteFile("tracked.txt", []byte("base\n"), 0600))
	for _, args := range [][]string{{"add", "tracked.txt"}, {"commit", "-m", "initial"}} {
		out, err := exec.Command("git", args...).CombinedOutput()
		require.NoError(t, err, "%s", out)
	}
	require.NoError(t, os.WriteFile("task.txt", []byte("file task {{literal}} 100%"), 0600))
	bin := filepath.Join(root, "bin")
	require.NoError(t, os.Mkdir(bin, 0700))
	exe, err := os.Executable()
	require.NoError(t, err)
	script := "#!/bin/sh\nexec \"$SOFT_FACTORY_TEST_EXE\" -test.run='^TestPromptSandboxHelper$' -- \"$@\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "agent-sandbox"), []byte(script), 0700))
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
	require.NoError(t, err)
	var capture promptCapture
	require.NoError(t, json.Unmarshal(data, &capture))
	return capture
}

func TestStagePromptIntegration(t *testing.T) {
	// Read frozen expectations before entering temporary invocation repositories.
	baseline := map[string]string{}
	for _, name := range []string{"task.txt", "supporting-context.txt", "custom-skill.txt", "reviewer-scope.txt", "review.txt", "review-report.txt", "stage-changes.txt", "risk-classification.txt", "review-changes.txt", "no-review-reports.txt", "disabled-reviews.txt"} {
		data, err := os.ReadFile(filepath.Join("..", "prompts", "testdata", name+".golden"))
		require.NoError(t, err)
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
		require.NoError(t, err)
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
					input := TaskContext{TaskOverride: "task {{literal}} <code> 100%", Documents: "document body", Branch: "main"}
					if logged {
						run, err := executionlog.NewRun("main", executionlog.CommandRun, nil)
						require.NoError(t, err)
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
						require.NoError(t, err)
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
					require.NoError(t, err)
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
						require.NotEmpty(t, path)
						want += frozen("review-report.txt", map[string]string{"ReportPath": path})
					case "risk-classification":
						evidence := baseline["no-review-reports.txt"] + frozen("disabled-reviews.txt", map[string]string{"Stages": "codeReview, securityReview"})
						want = frozen("risk-classification.txt", map[string]string{"ReportDirectory": "/workspace", "Skill": bundledSkills[stage], "AdditionalSkill": additional, "Task": task, "ReviewEvidence": evidence})
					case "review-changes":
						path := regexp.MustCompile(`/workspace/\.soft-factory-review-changes-[0-9]+\.md`).FindString(capture.Prompt)
						require.NotEmpty(t, path)
						want = frozen("review-changes.txt", map[string]string{"ReportPath": path, "Skill": bundledSkills[stage], "AdditionalSkill": additional, "Task": task, "HostEvidence": hostEvidence})
						report, err := os.ReadFile(filepath.Join(input.Log.Path(), "05-reviewChanges.md"))
						require.NoError(t, err)
						require.Equal(t, "No code changes made.\n", string(report))
					}
					if logged && stage == "risk-classification" {
						path := regexp.MustCompile(`/workspace/\.soft-factory-risk-result-[A-Za-z0-9]+\.json`).FindString(capture.Prompt)
						require.NotEmpty(t, path)
						require.True(t, strings.HasPrefix(capture.Prompt, want))
						resultInstructions := strings.TrimPrefix(capture.Prompt, want)
						require.Contains(t, resultInstructions, path)
						require.Contains(t, resultInstructions, `{"classification":"LOW"}`)
						got = strings.TrimSuffix(capture.Prompt, resultInstructions)
					} else if logged && stage != "review-changes" {
						path := regexp.MustCompile(`/workspace/\.soft-factory-stage-changes-[A-Za-z0-9]+\.txt`).FindString(capture.Prompt)
						require.NotEmpty(t, path)
						want += frozen("stage-changes.txt", map[string]string{"ReportPath": path})
					}
					require.Equal(t, want, got)
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
					wantArgs = append(wantArgs, "-f", promptPath, "--branch", "main")
					require.Equal(t, wantArgs, capture.Args)

					_, err = os.Stat(promptPath)
					require.ErrorIs(t, err, os.ErrNotExist)

					if logged && stage != "review-changes" {
						input.Log.FinishStage(nil)
						files, err := filepath.Glob(filepath.Join(input.Log.Path(), "*.log"))
						require.NoError(t, err)
						found := false
						for _, path := range files {
							text, err := os.ReadFile(path)
							require.NoError(t, err)
							if strings.Contains(string(text), capture.Prompt) {
								found = true
							}
						}
						require.True(t, found)
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
			require.NoError(t, Run(TaskContext{Branch: "main", Continue: continuing, Config: taskFileConfig(t)}))
			got := readPromptCapture(t, path)
			want := "file task {{literal}} 100%"
			require.Equal(t, want, got.Prompt)
			wantArgs := []string{"run", "--branch", "main", "--filePrompt", "task.txt"}
			if continuing {
				require.Len(t, got.Args, 5)
				wantArgs = []string{"resume", "-f", got.Args[2], "--branch", "main"}
			}
			require.Equal(t, wantArgs, got.Args)
		})
	}
}

func TestPromptCleanupAfterSandboxFailure(t *testing.T) {
	path := preparePromptSandbox(t)
	t.Setenv("SOFT_FACTORY_PROMPT_FAIL", "1")

	_, err := Review(TaskContext{TaskOverride: "task", Branch: "main"})
	require.Error(t, err)

	got := readPromptCapture(t, path)
	for i, arg := range got.Args {
		if arg == "-f" {
			_, err := os.Stat(got.Args[i+1])
			require.ErrorIs(t, err, os.ErrNotExist)
		}
	}
}
