package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
					require.NoError(t, os.WriteFile(localConfigFile, []byte(config), 0o644))
					var flags runFlags
					cmd := &cobra.Command{}
					cmd.Flags().StringVarP(&flags.Branch, "branch", "b", "", "")
					flags.bind(cmd)
					require.NoError(t, cmd.Flags().Parse(tt.args))
					err := configRunArgs(section, &flags)(cmd, cmd.Flags().Args())
					if tt.wantError {
						assertUsageError(t, err)
						require.Contains(t, err.Error(), section+".hn must be a JSON boolean")
						return
					}
					require.NoError(t, err)
					opts, err := sandbox.NewOptions(flags.Options)
					require.NoError(t, err)
					assert.Equal(t, tt.want, opts.HostNetwork)
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
			require.NoError(t, os.WriteFile(localConfigFile, []byte(tt.config), 0o644))

			var flags runFlags
			cmd := &cobra.Command{}
			cmd.Flags().StringVarP(&flags.Branch, "branch", "b", "", "")
			flags.bind(cmd)
			require.NoError(t, cmd.Flags().Parse(tt.args))
			promptArgs := cmd.Flags().Args()
			err := configRunArgs(section, &flags)(cmd, promptArgs)
			if tt.wantUsage {
				assertUsageError(t, err)
				return
			}
			require.NoError(t, err)

			opts, err := sandbox.NewOptions(flags.Options)
			require.NoError(t, err)
			assert.Equal(t, tt.wantPrompt, opts.Prompt)
			if tt.wantBranch != "" {
				assert.Equal(t, tt.wantBranch, opts.Branch)
			}
			assert.Equal(t, tt.wantCommit, opts.CommitMessage)
			assert.Equal(t, tt.wantPush, opts.Push)
			assert.Equal(t, tt.wantHostNetwork, opts.HostNetwork)
			assert.Equal(t, "", opts.FilePrompt)
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
			require.NoError(t, os.WriteFile(localConfigFile, []byte(config), 0o600))

			var flags runFlags
			cmd := &cobra.Command{}
			cmd.Flags().StringVarP(&flags.Branch, "branch", "b", "", "")
			flags.bind(cmd)
			require.NoError(t, configRunArgs(tc.section, &flags)(cmd, nil))
			opts, err := sandbox.NewOptions(flags.Options)
			require.NoError(t, err)
			assert.Equal(t, tc.wantKey, opts.APIKey)
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
			require.NoError(t, os.WriteFile(localConfigFile, []byte(tt.config), 0o600))
			config, err := readLocalConfig()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			section := config.Run
			require.NotNil(t, section)
			require.NotNil(t, section.Branch)
			require.Equal(t, "fix", *section.Branch)
			require.NotNil(t, section.Agent)
			require.Equal(t, "codex", *section.Agent)
			require.NotNil(t, section.APIKey)
			require.Equal(t, "key", *section.APIKey)
			require.NotNil(t, section.Model)
			require.Equal(t, "model", *section.Model)
			require.NotNil(t, section.BaseImage)
			require.Equal(t, "alpine:latest", *section.BaseImage)
			require.NotNil(t, section.Query)
			require.Equal(t, "task", *section.Query)
			require.NotNil(t, section.Push)
			require.False(t, *section.Push, "decoded config = %+v", section)
			require.NotNil(t, section.PR)
			require.True(t, *section.PR, "decoded config = %+v", section)
			require.NotNil(t, section.HostNetwork)
			require.False(t, *section.HostNetwork, "decoded config = %+v", section)
			require.NotNil(t, section.CommitMessage)
			require.Equal(t, "commit", *section.CommitMessage)
			require.NotNil(t, section.FilePrompt)
			require.Equal(t, "prompt.md", *section.FilePrompt)
			require.NotNil(t, section.Image)
			require.Len(t, *section.Image, 2)
			require.Equal(t, "two.png", (*section.Image)[1])
		})
	}
}

func TestRunAndResumeBranchPrecedence(t *testing.T) {
	tests := []struct {
		name   string
		config string
		args   []string
		want   string
	}{
		{name: "root branch without section", config: `{"branch":"shared-branch"}`, want: "shared-branch"},
		{name: "root branch with section", config: `{"branch":"shared-branch","%s":{}}`, want: "shared-branch"},
		{name: "section overrides root", config: `{"branch":"shared-branch","%s":{"branch":"section-branch"}}`, want: "section-branch"},
		{name: "section branch without root", config: `{"%s":{"branch":"section-branch"}}`, want: "section-branch"},
		{name: "empty section overrides root", config: `{"branch":"shared-branch","%s":{"branch":""}}`},
		{name: "long CLI flag overrides both levels", config: `{"branch":"shared-branch","%s":{"branch":"section-branch"}}`, args: []string{"--branch", "cli-branch"}, want: "cli-branch"},
		{name: "short CLI flag overrides both levels", config: `{"branch":"shared-branch","%s":{"branch":"section-branch"}}`, args: []string{"-b", "cli-branch"}, want: "cli-branch"},
		{name: "CLI flag overrides root without section", config: `{"branch":"shared-branch"}`, args: []string{"-b", "cli-branch"}, want: "cli-branch"},
		{name: "empty CLI overrides both levels", config: `{"branch":"shared-branch","%s":{"branch":"section-branch"}}`, args: []string{"--branch="}},
		{name: "empty root", config: `{"branch":""}`},
		{name: "no configured branch", config: `{}`},
	}
	for _, section := range []string{"run", "resume"} {
		for _, tt := range tests {
			t.Run(section+"/"+tt.name, func(t *testing.T) {
				t.Chdir(t.TempDir())
				config := strings.ReplaceAll(tt.config, "%s", section)
				require.NoError(t, os.WriteFile(localConfigFile, []byte(config), 0o644))
				var flags runFlags
				cmd := &cobra.Command{}
				cmd.Flags().StringVarP(&flags.Branch, "branch", "b", "", "")
				flags.bind(cmd)
				args := append([]string{"--agent", "codex", "--query", "task"}, tt.args...)
				require.NoError(t, cmd.Flags().Parse(args))
				require.NoError(t, configRunArgs(section, &flags)(cmd, cmd.Flags().Args()))
				require.Equal(t, tt.want, flags.Branch)
			})
		}
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
				require.NoError(t, os.WriteFile(localConfigFile, []byte(config), 0o644))
				var flags runFlags
				cmd := &cobra.Command{}
				cmd.Flags().StringVarP(&flags.Branch, flagBranch, "b", "", "")
				flags.bind(cmd)
				require.NoError(t, cmd.Flags().Parse(tt.args))
				require.NoError(t, configRunArgs(section, &flags)(cmd, cmd.Flags().Args()))
				got := flags.Options
				assert.Equal(t, tt.want.Model, got.Model)
				assert.Equal(t, tt.want.AgentName, got.AgentName)
				assert.Equal(t, tt.want.BaseImage, got.BaseImage)
				assert.Equal(t, tt.want.Push, got.Push)
				assert.Equal(t, tt.want.PR, got.PR)
				assert.Equal(t, tt.want.HostNetwork, got.HostNetwork)
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
		{name: "branch null", field: `"branch":null`, wantErr: "branch must be a JSON string"},
		{name: "branch number", field: `"branch":1`, wantErr: "branch must be a JSON string"},
		{name: "branch boolean", field: `"branch":false`, wantErr: "branch must be a JSON string"},
		{name: "branch object", field: `"branch":{}`, wantErr: "branch must be a JSON string"},
		{name: "branch array", field: `"branch":[]`, wantErr: "branch must be a JSON string"},
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
			require.NoError(t, os.WriteFile(localConfigFile, []byte(config), 0o644))
			var flags runFlags
			cmd := &cobra.Command{}
			cmd.Flags().StringVarP(&flags.Branch, flagBranch, "b", "", "")
			flags.bind(cmd)
			err := configRunArgs("run", &flags)(cmd, nil)
			assertUsageError(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
