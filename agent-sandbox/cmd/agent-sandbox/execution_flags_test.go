package main

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/sandbox"
)

var executionCommands = []string{"run", "resume", "run-tui", "resume-tui"}

func TestExecutionAuthenticationAndReviewerFlagHelp(t *testing.T) {
	for _, command := range executionCommands {
		t.Run(command, func(t *testing.T) {
			root, _ := newRootCmd()
			cmd, _, err := root.Find([]string{command})
			if err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct{ name, kind string }{
				{flagAPIKey, "string"}, {flagReviewers, "stringArray"},
			} {
				flag := cmd.Flags().Lookup(tc.name)
				if flag == nil || flag.Shorthand != "" || flag.Value.Type() != tc.kind {
					t.Fatalf("unexpected registration for --%s: %+v", tc.name, flag)
				}
			}
			if err := cmd.ParseFlags([]string{"--apiKey", "dummy-cli-key", "--reviewers", "alice", "--reviewers", "bob"}); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			cmd.SetOut(&out)
			if err := cmd.Help(); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"--apiKey key", "--reviewers username", "repeatable", "no effect on resume"} {
				if !strings.Contains(out.String(), want) {
					t.Errorf("help missing %q", want)
				}
			}
			if strings.Contains(out.String(), "dummy-cli-key") {
				t.Fatal("help includes supplied API key")
			}
		})
	}
}

// executionFlagOptions exercises the shared bindings and argument validation
// without invoking Docker, host authentication, or a terminal UI.
func executionFlagOptions(t *testing.T, command, config string, args []string) (runFlags, error) {
	t.Helper()
	t.Chdir(t.TempDir())
	if config != "" {
		if err := os.WriteFile(localConfigFile, []byte(config), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var flags runFlags
	cmd := &cobra.Command{}
	cmd.Flags().StringVarP(&flags.Branch, flagBranch, "b", "", "")
	flags.bind(cmd)
	args = append([]string{"--agent", "codex", "--query", "task"}, args...)
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatal(err)
	}
	err := configRunArgs(strings.TrimSuffix(command, "-tui"), &flags)(cmd, cmd.Flags().Args())
	return flags, err
}

func TestExecutionAPIKeyPrecedence(t *testing.T) {
	for _, command := range executionCommands {
		for _, tc := range []struct {
			name, config, want string
			args               []string
		}{
			{name: "absent JSON"},
			{name: "CLI without JSON", args: []string{"--apiKey", "dummy-cli-key"}, want: "dummy-cli-key"},
			{name: "section fallback", config: `{"run":{"apiKey":"dummy-run-key"},"resume":{"apiKey":"dummy-resume-key"}}`, want: "section"},
			{name: "CLI overrides section", config: `{"run":{"apiKey":"dummy-run-key"},"resume":{"apiKey":"dummy-resume-key"}}`, args: []string{"--apiKey", "dummy-cli-key"}, want: "dummy-cli-key"},
			{name: "empty CLI overrides section", config: `{"run":{"apiKey":"dummy-run-key"},"resume":{"apiKey":"dummy-resume-key"}}`, args: []string{"--apiKey="}},
			{name: "empty section", config: `{"run":{"apiKey":""},"resume":{"apiKey":""}}`},
			{name: "other section only", config: "other"},
		} {
			t.Run(command+"/"+tc.name, func(t *testing.T) {
				section := strings.TrimSuffix(command, "-tui")
				if tc.want == "section" {
					tc.want = "dummy-" + section + "-key"
				}
				if tc.config == "other" {
					other := "run"
					if section == "run" {
						other = "resume"
					}
					tc.config = `{"` + other + `":{"apiKey":"dummy-other-key"}}`
				}
				flags, err := executionFlagOptions(t, command, tc.config, tc.args)
				if err != nil {
					t.Fatal(err)
				}
				opts, err := sandbox.NewOptions(flags.Options)
				if err != nil {
					t.Fatal(err)
				}
				if opts.APIKey != tc.want {
					t.Fatal("API key did not follow CLI and section precedence")
				}
				if strings.Contains(opts.FullPrompt(), "dummy-") {
					t.Fatal("prompt includes supplied API key")
				}
			})
		}
	}
}

func TestExecutionAPIKeyAgentSupport(t *testing.T) {
	for _, command := range executionCommands {
		for _, agent := range []string{"codex", "claude", "opencode", "pi"} {
			for _, key := range []string{"dummy-cli-key", ""} {
				t.Run(command+"/"+agent+"/"+key, func(t *testing.T) {
					flags, err := executionFlagOptions(t, command, "", []string{"--agent", agent, "--apiKey=" + key})
					if err != nil {
						t.Fatal(err)
					}
					_, err = sandbox.NewOptions(flags.Options)
					if key != "" && (agent == "opencode" || agent == "pi") {
						assertUsageError(t, err)
						if !strings.Contains(err.Error(), "apiKey is not supported for agent "+agent) || strings.Contains(err.Error(), key) {
							t.Fatalf("unexpected unsupported-agent error: %v", err)
						}
					} else if err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestExecutionReviewerFlags(t *testing.T) {
	for _, command := range executionCommands {
		for _, tc := range []struct {
			name, config string
			args, want   []string
		}{
			{name: "absent JSON"},
			{name: "CLI without JSON", args: []string{"--reviewers", "alice", "--reviewers", "bob"}, want: []string{"alice", "bob"}},
			{name: "JSON fallback", config: `{"run":{"reviewers":[" Json-User ","other","JSON-user"]}}`, want: []string{"Json-User", "other"}},
			{name: "CLI replaces JSON", config: `{"run":{"reviewers":["json-user"]}}`, args: []string{"--reviewers", " Alice ", "--reviewers", "bob-2", "--reviewers", "ALICE", "--reviewers", " BOB-2 ", "--reviewers", "7user"}, want: []string{"Alice", "bob-2", "7user"}},
			{name: "clear JSON", config: `{"run":{"reviewers":["json-user"]}}`, args: []string{"--reviewers="}},
			{name: "clear without JSON", args: []string{"--reviewers="}},
		} {
			t.Run(command+"/"+tc.name, func(t *testing.T) {
				flags, err := executionFlagOptions(t, command, tc.config, tc.args)
				if err != nil {
					t.Fatal(err)
				}
				if strings.HasPrefix(command, "resume") {
					tc.want = nil
				}
				if !reflect.DeepEqual(flags.Reviewers, tc.want) {
					t.Fatalf("reviewers = %v, want %v", flags.Reviewers, tc.want)
				}
			})
		}
	}
}

func TestExecutionInvalidReviewerFlagsAreUsageErrors(t *testing.T) {
	for _, command := range executionCommands {
		for _, values := range [][]string{
			{" "}, {"\t"}, {"@alice"}, {"alice@example.com"}, {"org/team"},
			{"alice,bob"}, {"alice bob"}, {"--draft"}, {"-alice"}, {"alice_bob"},
			{"", "alice"}, {"alice", ""}, {"", ""}, {"alice", " "},
		} {
			t.Run(command+"/"+strings.Join(values, "+"), func(t *testing.T) {
				t.Chdir(t.TempDir())
				root, _ := newRootCmd()
				args := []string{command, "--agent", "codex", "--query", "task", "--branch", "feature"}
				for _, value := range values {
					args = append(args, "--reviewers="+value)
				}
				root.SetArgs(args)
				err := root.Execute()
				assertUsageError(t, err)
				if !strings.Contains(err.Error(), "--reviewers[") {
					t.Fatalf("unexpected reviewer error: %v", err)
				}
			})
		}
	}
}

func TestExecutionFlagOverridesStillValidateCompleteJSON(t *testing.T) {
	for _, command := range executionCommands {
		for _, config := range []string{
			`{"run":{"apiKey":null}}`, `{"resume":{"apiKey":12}}`,
			`{"run":{"reviewers":["@invalid"]}}`, `{"resume":{"reviewers":[]}}`,
			`{"reviewers":[]}`, `{"apiKey":"dummy-root-key"}`,
		} {
			t.Run(command+"/"+config, func(t *testing.T) {
				_, err := executionFlagOptions(t, command, config, []string{"--apiKey=", "--reviewers="})
				assertUsageError(t, err)
				if !strings.Contains(err.Error(), localConfigFile) {
					t.Fatalf("unexpected JSON validation error: %v", err)
				}
			})
		}
	}
}

func TestExecutionFlagSyntaxErrorsAreUsageErrors(t *testing.T) {
	for _, command := range executionCommands {
		for _, args := range [][]string{{"--api-key=dummy-key"}, {"--apiKey"}, {"--reviewers"}} {
			t.Run(command+"/"+strings.Join(args, " "), func(t *testing.T) {
				t.Chdir(t.TempDir())
				root, _ := newRootCmd()
				root.SetArgs(append([]string{command}, args...))
				assertUsageError(t, root.Execute())
			})
		}
	}
}
