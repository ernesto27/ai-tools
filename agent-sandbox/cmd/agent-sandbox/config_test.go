package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/sandbox"
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
					var flags runFlags
					cmd := &cobra.Command{}
					cmd.Flags().StringVarP(&flags.Branch, "branch", "b", "", "")
					flags.bind(cmd)
					if err := cmd.Flags().Parse(tt.args); err != nil {
						t.Fatal(err)
					}
					err := configRunArgs(section, &flags)(cmd, cmd.Flags().Args())
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
					opts, err := sandbox.NewOptions(flags.Options)
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
			config:     `{"run":{"agent":"codex","query":"run go version","commitMessage":"this is from json file"}}`,
			wantPrompt: "run go version",
			wantCommit: "this is from json file",
		},
		{
			name:      "commit message does not supply a query",
			config:    `{"run":{"agent":"codex","commitMessage":"this is from json file"}}`,
			wantUsage: true,
		},
		{
			name:       "CLI query shorthand overrides JSON query and file prompt",
			config:     `{"run":{"agent":"codex","query":"JSON task","filePrompt":"prompt.md"}}`,
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

			var flags runFlags
			cmd := &cobra.Command{}
			cmd.Flags().StringVarP(&flags.Branch, "branch", "b", "", "")
			flags.bind(cmd)
			if err := cmd.Flags().Parse(tt.args); err != nil {
				t.Fatal(err)
			}
			promptArgs := cmd.Flags().Args()
			err := configRunArgs(section, &flags)(cmd, promptArgs)
			if tt.wantUsage {
				assertUsageError(t, err)
				return
			}
			if err != nil {
				t.Fatalf("run args: %v", err)
			}

			opts, err := sandbox.NewOptions(flags.Options)
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
			config := `{"run":{"agent":"codex","query":"run task","apiKey":"run-key"},` +
				`"resume":{"agent":"codex","query":"resume task","apiKey":"resume-key"}}`
			if err := os.WriteFile(localConfigFile, []byte(config), 0o600); err != nil {
				t.Fatal(err)
			}

			var flags runFlags
			cmd := &cobra.Command{}
			cmd.Flags().StringVarP(&flags.Branch, "branch", "b", "", "")
			flags.bind(cmd)
			if err := configRunArgs(tc.section, &flags)(cmd, nil); err != nil {
				t.Fatal(err)
			}
			opts, err := sandbox.NewOptions(flags.Options)
			if err != nil {
				t.Fatal(err)
			}
			if opts.APIKey != tc.wantKey {
				t.Errorf("APIKey = %q, want selected section key", opts.APIKey)
			}
		})
	}
}

func TestLocalConfigFieldsAndValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{name: "all typed fields", config: `{"run":{"branch":"fix","agent":"codex","apiKey":"key","model":"model","baseImage":"alpine:latest","query":"task","push":false,"pr":true,"hn":false,"commitMessage":"commit","filePrompt":"prompt.md","image":["one.png","two.png"]}}`},
		{name: "unknown section", config: `{"other":{}}`, wantErr: `unknown section "other"`},
		{name: "unknown field", config: `{"resume":{"typo":true}}`, wantErr: `unknown field resume.typo`},
		{name: "null section", config: `{"run":null}`, wantErr: `run must be a JSON object`},
		{name: "null string", config: `{"run":{"agent":null}}`, wantErr: `run.agent must be a JSON string`},
		{name: "null boolean", config: `{"resume":{"push":null}}`, wantErr: `resume.push must be a JSON boolean`},
		{name: "null image", config: `{"run":{"image":null}}`, wantErr: `run.image must be an array of JSON strings`},
		{name: "non-string image", config: `{"run":{"image":["one.png",false]}}`, wantErr: `run.image[1] must be a JSON string`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile(localConfigFile, []byte(tt.config), 0o600); err != nil {
				t.Fatal(err)
			}
			config, err := readLocalConfig()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("readLocalConfig error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			section := config.Run
			if section == nil || section.Branch == nil || *section.Branch != "fix" ||
				section.Agent == nil || *section.Agent != "codex" ||
				section.APIKey == nil || *section.APIKey != "key" ||
				section.Model == nil || *section.Model != "model" ||
				section.BaseImage == nil || *section.BaseImage != "alpine:latest" ||
				section.Query == nil || *section.Query != "task" ||
				section.Push == nil || *section.Push ||
				section.PR == nil || !*section.PR ||
				section.HostNetwork == nil || *section.HostNetwork ||
				section.CommitMessage == nil || *section.CommitMessage != "commit" ||
				section.FilePrompt == nil || *section.FilePrompt != "prompt.md" ||
				section.Image == nil || len(*section.Image) != 2 || (*section.Image)[1] != "two.png" {
				t.Fatalf("decoded config = %+v", section)
			}
		})
	}
}

func TestRunAndResumeBranchPrecedence(t *testing.T) {
	for _, tt := range []struct {
		name    string
		section string
		args    []string
		want    string
	}{
		{name: "run uses JSON branch", section: "run", want: "json-branch"},
		{name: "run CLI branch wins", section: "run", args: []string{"--branch", "cli-branch"}, want: "cli-branch"},
		{name: "resume uses JSON branch", section: "resume", want: "json-branch"},
		{name: "resume CLI branch wins", section: "resume", args: []string{"-b", "cli-branch"}, want: "cli-branch"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			config := `{"run":{"branch":"json-branch","agent":"codex","query":"task"},` +
				`"resume":{"branch":"json-branch","agent":"codex","query":"task"}}`
			if err := os.WriteFile(localConfigFile, []byte(config), 0o644); err != nil {
				t.Fatal(err)
			}
			var flags runFlags
			cmd := &cobra.Command{}
			cmd.Flags().StringVarP(&flags.Branch, "branch", "b", "", "")
			flags.bind(cmd)
			if err := cmd.Flags().Parse(tt.args); err != nil {
				t.Fatal(err)
			}
			if err := configRunArgs(tt.section, &flags)(cmd, cmd.Flags().Args()); err != nil {
				t.Fatal(err)
			}
			opts, err := sandbox.NewOptions(flags.Options)
			if err != nil {
				t.Fatal(err)
			}
			if opts.Branch != tt.want {
				t.Fatalf("branch = %q, want %q", opts.Branch, tt.want)
			}
		})
	}
}

func TestRunAndResumeSharedConfigPrecedence(t *testing.T) {
	sectionOverrides := `,"model":"section-model","agent":"claude","baseImage":"section-image","push":false,"pr":false,"hn":false`
	tests := []struct {
		name          string
		sectionFields string
		args          []string
		want          sandbox.Options
	}{
		{
			name: "top-level defaults",
			want: sandbox.Options{Model: "shared-model", AgentName: "codex", BaseImage: "shared-image", Push: true, PR: true, HostNetwork: true},
		},
		{
			name:          "section overrides top-level defaults",
			sectionFields: sectionOverrides,
			want:          sandbox.Options{Model: "section-model", AgentName: "claude", BaseImage: "section-image"},
		},
		{
			name:          "CLI overrides both JSON levels",
			sectionFields: sectionOverrides,
			args:          []string{"--model", "cli-model", "--agent", "pi", "--baseImage", "cli-image", "--push=true", "--pr=true", "--hn=true"},
			want:          sandbox.Options{Model: "cli-model", AgentName: "pi", BaseImage: "cli-image", Push: true, PR: true, HostNetwork: true},
		},
		{
			name:          "explicit empty and false section values override top-level defaults",
			sectionFields: `,"model":"","baseImage":"","push":false,"pr":false,"hn":false`,
			want:          sandbox.Options{AgentName: "codex"},
		},
		{
			name: "explicit false CLI flags override top-level true",
			args: []string{"--push=false", "--pr=false", "--hn=false"},
			want: sandbox.Options{Model: "shared-model", AgentName: "codex", BaseImage: "shared-image"},
		},
	}

	for _, section := range []string{"run", "resume"} {
		for _, tt := range tests {
			t.Run(section+"/"+tt.name, func(t *testing.T) {
				t.Chdir(t.TempDir())
				config := fmt.Sprintf(`{"model":"shared-model","agent":"codex","baseImage":"shared-image","push":true,"pr":true,"hn":true,"%s":{"branch":"existing","query":"task"%s}}`, section, tt.sectionFields)
				if err := os.WriteFile(localConfigFile, []byte(config), 0o644); err != nil {
					t.Fatal(err)
				}
				var flags runFlags
				cmd := &cobra.Command{}
				cmd.Flags().StringVarP(&flags.Branch, flagBranch, "b", "", "")
				flags.bind(cmd)
				if err := cmd.Flags().Parse(tt.args); err != nil {
					t.Fatal(err)
				}
				if err := configRunArgs(section, &flags)(cmd, cmd.Flags().Args()); err != nil {
					t.Fatal(err)
				}
				got := flags.Options
				if got.Model != tt.want.Model || got.AgentName != tt.want.AgentName ||
					got.BaseImage != tt.want.BaseImage || got.Push != tt.want.Push ||
					got.PR != tt.want.PR || got.HostNetwork != tt.want.HostNetwork {
					t.Errorf("merged options = %+v, want model=%q agent=%q baseImage=%q push=%t pr=%t hn=%t",
						got, tt.want.Model, tt.want.AgentName, tt.want.BaseImage, tt.want.Push, tt.want.PR, tt.want.HostNetwork)
				}
			})
		}
	}
}

func TestTopLevelConfigTypes(t *testing.T) {
	for _, tt := range []struct {
		name    string
		field   string
		wantErr string
	}{
		{name: "model null", field: `"model":null`, wantErr: "model must be a JSON string"},
		{name: "agent number", field: `"agent":1`, wantErr: "agent must be a JSON string"},
		{name: "baseImage boolean", field: `"baseImage":false`, wantErr: "baseImage must be a JSON string"},
		{name: "push string", field: `"push":"yes"`, wantErr: "push must be a JSON boolean"},
		{name: "pr null", field: `"pr":null`, wantErr: "pr must be a JSON boolean"},
		{name: "hn number", field: `"hn":1`, wantErr: "hn must be a JSON boolean"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			config := `{` + tt.field + `,"run":{"agent":"codex","query":"task"}}`
			if err := os.WriteFile(localConfigFile, []byte(config), 0o644); err != nil {
				t.Fatal(err)
			}
			var flags runFlags
			cmd := &cobra.Command{}
			cmd.Flags().StringVarP(&flags.Branch, flagBranch, "b", "", "")
			flags.bind(cmd)
			err := configRunArgs("run", &flags)(cmd, nil)
			assertUsageError(t, err)
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("config error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
