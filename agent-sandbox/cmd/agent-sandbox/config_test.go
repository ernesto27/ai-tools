package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunAndResumeHostNetworkConfig(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		args      []string
		want      bool
		wantError bool
	}{
		{name: "omitted defaults to false"},
		{name: "JSON enables host networking", field: `,"hn":true`, want: true},
		{name: "JSON disables host networking", field: `,"hn":false`},
		{name: "explicit CLI false overrides JSON true", field: `,"hn":true`, args: []string{"--hn=false"}},
		{name: "CLI true overrides JSON false", field: `,"hn":false`, args: []string{"--hn"}, want: true},
		{name: "CLI enables without JSON default", args: []string{"--hn"}, want: true},
		{name: "string is rejected", field: `,"hn":"true"`, wantError: true},
		{name: "number is rejected", field: `,"hn":1`, wantError: true},
		{name: "null is rejected", field: `,"hn":null`, wantError: true},
		{name: "array is rejected", field: `,"hn":[]`, wantError: true},
		{name: "object is rejected", field: `,"hn":{}`, wantError: true},
		{name: "CLI override still validates JSON type", field: `,"hn":"true"`, args: []string{"--hn=false"}, wantError: true},
	}
	for _, section := range []string{"run", "resume"} {
		t.Run(section, func(t *testing.T) {
			other := "resume"
			if section == "resume" {
				other = "run"
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Chdir(t.TempDir())
					// The other section enables host networking so false and omitted
					// values also verify that defaults never leak between commands.
					config := fmt.Sprintf(`{"%s":{"branch":"network-check","agent":"codex","query":"inspect"%s},"%s":{"hn":true}}`, section, tt.field, other)
					if err := os.WriteFile(localConfigFile, []byte(config), 0o644); err != nil {
						t.Fatal(err)
					}
					var branch string
					var flags runFlags
					cmd := &cobra.Command{}
					cmd.Flags().StringVarP(&branch, "branch", "b", "", "")
					flags.bind(cmd)
					if err := cmd.Flags().Parse(tt.args); err != nil {
						t.Fatal(err)
					}
					err := configRunArgs(section, &branch, &flags)(cmd, cmd.Flags().Args())
					if tt.wantError {
						assertUsageError(t, err)
						if !strings.Contains(err.Error(), section+".hn must be a JSON boolean") {
							t.Fatalf("config error = %v, want hn boolean validation error", err)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					opts, err := flags.options(branch)
					if err != nil {
						t.Fatal(err)
					}
					if opts.HostNetwork != tt.want {
						t.Errorf("HostNetwork = %t, want %t", opts.HostNetwork, tt.want)
					}
				})
			}
		})
	}
}

func TestRunAndResumeQuerySources(t *testing.T) {
	tests := []struct {
		name            string
		section         string
		config          string
		args            []string
		wantBranch      string
		wantPrompt      string
		wantCommit      string
		wantPush        bool
		wantHostNetwork bool
		wantUsage       bool
	}{
		{
			name:       "JSON query and commit message stay separate",
			config:     `{"run":{"agent":"codex","query":"run go version","commit-message":"this is from json file"}}`,
			wantPrompt: "run go version",
			wantCommit: "this is from json file",
		},
		{
			name:      "commit message does not supply a query",
			config:    `{"run":{"agent":"codex","commit-message":"this is from json file"}}`,
			wantUsage: true,
		},
		{
			name:       "CLI query shorthand overrides JSON query and file prompt",
			config:     `{"run":{"agent":"codex","query":"JSON task","file-prompt":"prompt.md"}}`,
			args:       []string{"-q", "CLI task"},
			wantPrompt: "CLI task",
		},
		{
			name:       "long query flag overrides JSON query",
			config:     `{"run":{"agent":"codex","query":"JSON task"}}`,
			args:       []string{"--query", "long CLI task"},
			wantPrompt: "long CLI task",
		},
		{
			name:       "push shorthand keeps the JSON query",
			config:     `{"run":{"agent":"codex","query":"JSON task"}}`,
			args:       []string{"-p"},
			wantPrompt: "JSON task",
			wantPush:   true,
		},
		{
			name:            "host network flag survives option resolution",
			config:          `{"run":{"agent":"codex","query":"run go version"}}`,
			args:            []string{"--hn"},
			wantPrompt:      "run go version",
			wantHostNetwork: true,
		},
		{
			name:      "run rejects positional prompt even with JSON query",
			config:    `{"run":{"agent":"codex","query":"JSON task"}}`,
			args:      []string{"CLI", "task"},
			wantUsage: true,
		},
		{
			name:      "resume rejects positional prompt even with JSON query",
			section:   "resume",
			config:    `{"resume":{"branch":"existing-branch","agent":"codex","query":"JSON continuation"}}`,
			args:      []string{"CLI", "task"},
			wantUsage: true,
		},
		{
			name:       "resume query shorthand overrides its JSON query and push remains a switch",
			section:    "resume",
			config:     `{"resume":{"branch":"existing-branch","agent":"codex","query":"JSON continuation"}}`,
			args:       []string{"-p", "-q", "CLI continuation"},
			wantBranch: "existing-branch",
			wantPrompt: "CLI continuation",
			wantPush:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			section := tt.section
			if section == "" {
				section = "run"
			}
			t.Chdir(t.TempDir())
			if err := os.WriteFile(localConfigFile, []byte(tt.config), 0o644); err != nil {
				t.Fatal(err)
			}

			var branch string
			var flags runFlags
			cmd := &cobra.Command{}
			cmd.Flags().StringVarP(&branch, "branch", "b", "", "")
			flags.bind(cmd)
			if err := cmd.Flags().Parse(tt.args); err != nil {
				t.Fatal(err)
			}
			promptArgs := cmd.Flags().Args()
			err := configRunArgs(section, &branch, &flags)(cmd, promptArgs)
			if tt.wantUsage {
				assertUsageError(t, err)
				return
			}
			if err != nil {
				t.Fatalf("run args: %v", err)
			}

			opts, err := flags.options(branch)
			if err != nil {
				t.Fatalf("resolve options: %v", err)
			}
			if opts.Prompt != tt.wantPrompt {
				t.Errorf("agent prompt = %q, want %q", opts.Prompt, tt.wantPrompt)
			}
			if opts.Branch != tt.wantBranch && tt.wantBranch != "" {
				t.Errorf("branch = %q, want %q", opts.Branch, tt.wantBranch)
			}
			if opts.CommitMessage != tt.wantCommit {
				t.Errorf("commit message = %q, want %q", opts.CommitMessage, tt.wantCommit)
			}
			if opts.Push != tt.wantPush {
				t.Errorf("push = %t, want %t", opts.Push, tt.wantPush)
			}
			if opts.HostNetwork != tt.wantHostNetwork {
				t.Errorf("host network = %t, want %t", opts.HostNetwork, tt.wantHostNetwork)
			}
			if opts.FilePrompt != "" {
				t.Errorf("file prompt = %q, want empty", opts.FilePrompt)
			}
		})
	}
}

func TestRunAndResumeAPIKeysStayInTheirSections(t *testing.T) {
	for _, tc := range []struct {
		section string
		wantKey string
	}{
		{section: "run", wantKey: "run-key"},
		{section: "resume", wantKey: "resume-key"},
	} {
		t.Run(tc.section, func(t *testing.T) {
			t.Chdir(t.TempDir())
			config := `{"run":{"agent":"codex","query":"run task","api-key":"run-key"},` +
				`"resume":{"agent":"codex","query":"resume task","api-key":"resume-key"}}`
			if err := os.WriteFile(localConfigFile, []byte(config), 0o600); err != nil {
				t.Fatal(err)
			}

			var branch string
			var flags runFlags
			cmd := &cobra.Command{}
			cmd.Flags().StringVarP(&branch, "branch", "b", "", "")
			flags.bind(cmd)
			if err := configRunArgs(tc.section, &branch, &flags)(cmd, nil); err != nil {
				t.Fatal(err)
			}
			opts, err := flags.options(branch)
			if err != nil {
				t.Fatal(err)
			}
			if opts.APIKey != tc.wantKey {
				t.Errorf("APIKey = %q, want selected section key", opts.APIKey)
			}
		})
	}
}
