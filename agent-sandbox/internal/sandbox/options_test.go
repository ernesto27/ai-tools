package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOptionsEffort(t *testing.T) {
	for _, agentName := range AgentNames() {
		for _, tc := range []struct {
			name   string
			effort string
			want   string
		}{
			{name: "default", want: "high"},
			{name: "whitespace only", effort: " \t\n ", want: "high"},
			{name: "override", effort: "low", want: "low"},
			{name: "trim", effort: " low ", want: "low"},
			{name: "keep case", effort: " HIGH ", want: "HIGH"},
			{name: "unknown", effort: "custom", want: "custom"},
		} {
			t.Run(agentName+"/"+tc.name, func(t *testing.T) {
				got, err := NewOptions(Options{
					AgentName: agentName,
					Branch:    "feature",
					Prompt:    "inspect",
					Effort:    tc.effort,
				})
				require.NoError(t, err)
				require.Equal(t, tc.want, got.Effort)
			})
		}
	}
}

func TestFullPromptCommitRules(t *testing.T) {
	for _, tc := range []struct {
		name     string
		pr       bool
		push     bool
		message  string
		want     []string
		unwanted []string
	}{
		{name: "ordinary run", want: []string{"Do not stage, commit, or push"}, unwanted: []string{"Before finishing, stage and commit", prContentFile}},
		{name: "push commits in container", push: true, want: []string{"Before finishing, stage and commit", "Do not push or create a pull request", "Do not change Git configuration, hooks, refs", "Inspect the staged diff", "Leave no uncommitted changes"}, unwanted: []string{"Do not stage, commit, or push", prContentFile}},
		{name: "PR chooses message", pr: true, want: []string{"Before finishing, stage and commit", "Choose one short commit message", "Do not push or create a pull request", "/workspace/" + prContentFile, "Do not stage or commit this file", "Leave no other uncommitted changes", "including earlier sessions", "abc123...HEAD", "abc123..HEAD", "status, diff, log, add, and commit"}, unwanted: []string{"Do not stage, commit, or push", "Leave no uncommitted changes"}},
		{name: "PR uses explicit message", pr: true, message: "fix login", want: []string{"Use this exact commit message: \"fix login\""}, unwanted: []string{"Choose one short commit message"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prompt := (Options{Prompt: "implement this", PR: tc.pr, Push: tc.push, CommitMessage: tc.message, prBase: "abc123"}).FullPrompt()
			for _, value := range tc.want {
				assert.Contains(t, prompt, value)
			}
			for _, value := range tc.unwanted {
				assert.NotContains(t, prompt, value)
			}
		})
	}
}

func TestPRContainerOptionsMountGitMetadata(t *testing.T) {
	repoDir := setupDeleteTestRepo(t)
	worktree := addDeleteTestWorktree(t, repoDir, "feature")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	opts, err := NewOptions(Options{AgentName: "codex", APIKey: "test-key", Branch: "feature", PR: true, Prompt: "fix this"})
	require.NoError(t, err)
	runOpts, err := containerOptions(opts, worktree.Path)
	require.NoError(t, err)
	commonDir := filepath.Join(repoDir, ".git")
	found := false
	for _, mount := range runOpts.Mounts {
		if mount.Host == commonDir && mount.Container == commonDir && !mount.ReadOnly {
			found = true
		}
	}
	require.True(t, found, "Git metadata mount missing: %#v", runOpts.Mounts)
	configProtected := false
	for _, mount := range runOpts.Mounts {
		path := filepath.Join(commonDir, "config")
		if mount.Host == path && mount.Container == path && mount.ReadOnly {
			configProtected = true
		}
	}
	require.True(t, configProtected, "read-only Git config mount missing: %#v", runOpts.Mounts)
	for _, value := range []string{"GIT_COMMON_DIR=" + commonDir, "GIT_WORK_TREE=/workspace", "GIT_AUTHOR_NAME=Test User", "GIT_AUTHOR_EMAIL=test@example.com"} {
		assert.Contains(t, runOpts.Env, value)
	}
	require.NoError(t, os.WriteFile(filepath.Join(worktree.Path, "change.txt"), []byte("committed in container\n"), 0o600))
	for _, args := range [][]string{{"add", "change.txt"}, {"commit", "-m", "agent commit"}} {
		command := exec.Command("git", append([]string{"-C", worktree.Path}, args...)...)
		command.Env = append(append(os.Environ(), runOpts.Env...), "GIT_WORK_TREE="+worktree.Path)
		{
			output, err := command.CombinedOutput()
			require.NoError(t, err, "command output: %s", output)
		}
	}
	command := exec.Command("git", "-C", worktree.Path, "log", "-1", "--format=%s")
	{
		output, err := command.Output()
		require.NoError(t, err)
		require.Equal(t, "agent commit", strings.TrimSpace(string(output)))
	}
}

func TestCodexCredentialSelection(t *testing.T) {
	const key = "test-api-key"
	cases := []struct {
		name          string
		apiKey        string
		createHostDir bool
		wantHostMount bool
		wantKeyEnv    bool
	}{
		{name: "host login", createHostDir: true, wantHostMount: true},
		{name: "API key without host login", apiKey: key, wantKeyEnv: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repoDir := setupDeleteTestRepo(t)
			worktree := addDeleteTestWorktree(t, repoDir, "feature")
			runGit(t, repoDir, "config", "user.name", "Test User")
			runGit(t, repoDir, "config", "user.email", "test@example.com")
			home := t.TempDir()
			t.Setenv("HOME", home)
			if tc.createHostDir {
				require.NoError(t, os.Mkdir(filepath.Join(home, ".codex"), 0o700))
			}

			opts, err := NewOptions(Options{
				AgentName: "codex",
				APIKey:    tc.apiKey,
				Branch:    "test-branch",
				Prompt:    "inspect this",
			})
			require.NoError(t, err)
			got, err := containerOptions(opts, worktree.Path)
			require.NoError(t, err)

			hostMount := false
			for _, mount := range got.Mounts {
				if mount.Host == filepath.Join(home, ".codex") {
					hostMount = true
				}
			}
			assert.Equal(t, tc.wantHostMount, hostMount)
			keyEnv := slices.Contains(got.Env, "OPENAI_API_KEY="+key)
			assert.Equal(t, tc.wantKeyEnv, keyEnv)
			assert.NotContains(t, strings.Join(got.Args, " "), key)
			provider := slices.Contains(got.Args, `model_provider="agent_sandbox_api"`)
			assert.Equal(t, tc.wantKeyEnv, provider)
			inMemoryHome := false
			for _, mount := range got.Tmpfs {
				if mount.Path == "/codex-home" {
					inMemoryHome = true
				}
			}
			assert.Equal(t, tc.wantKeyEnv, inMemoryHome)
		})
	}
}

func TestClaudeCredentialSelection(t *testing.T) {
	const key = "test-claude-key"
	for _, tc := range []struct {
		name          string
		apiKey        string
		createHostDir bool
		wantMounts    int
		wantKeyEnv    bool
	}{
		{name: "host login", createHostDir: true, wantMounts: 5},
		{name: "API key without host login", apiKey: key, wantMounts: 3, wantKeyEnv: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoDir := setupDeleteTestRepo(t)
			worktree := addDeleteTestWorktree(t, repoDir, "feature")
			runGit(t, repoDir, "config", "user.name", "Test User")
			runGit(t, repoDir, "config", "user.email", "test@example.com")
			home := t.TempDir()
			t.Setenv("HOME", home)
			if tc.createHostDir {
				require.NoError(t, os.Mkdir(filepath.Join(home, ".claude"), 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o600))
			}

			opts, err := NewOptions(Options{AgentName: "claude", APIKey: tc.apiKey, Branch: "test-branch", Prompt: "inspect this"})
			require.NoError(t, err)
			got, err := containerOptions(opts, worktree.Path)
			require.NoError(t, err)

			assert.Equal(t, tc.wantMounts, len(got.Mounts))
			hostCredentialMounts := 0
			for _, mount := range got.Mounts {
				if mount.Host == filepath.Join(home, ".claude") || mount.Host == filepath.Join(home, ".claude.json") {
					hostCredentialMounts++
				}
			}
			wantHostCredentialMounts := 0
			if tc.createHostDir {
				wantHostCredentialMounts = 2
			}
			assert.Equal(t, wantHostCredentialMounts, hostCredentialMounts)
			keyEnv := slices.Contains(got.Env, "ANTHROPIC_API_KEY="+key)
			assert.Equal(t, tc.wantKeyEnv, keyEnv)
			assert.NotContains(t, strings.Join(got.Args, " "), key)
			assert.Contains(t, got.Args, "--print", "Claude key path must run in non-interactive print mode")
			for _, arg := range []string{"--output-format", "stream-json", "--verbose", "--include-partial-messages"} {
				count := 0
				for _, actual := range got.Args {
					if actual == arg {
						count++
					}
				}
				assert.Equal(t, 1, count)
			}
			assert.True(t, got.Interactive, "Claude stdin settings must remain unchanged")
			assert.False(t, got.TTY, "Claude TTY settings must remain unchanged")
			assert.Equal(t, tc.wantKeyEnv, (len(got.Tmpfs) == 1 && got.Tmpfs[0].Path == "/claude-home"))
			if tc.wantKeyEnv {
				for _, env := range []string{"CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST=1", "CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=0"} {
					assert.Contains(t, got.Env, env)
				}
			}
		})
	}
}

func TestNewOptionsRejectsAPIKeyForOtherAgents(t *testing.T) {
	for _, name := range []string{"opencode", "pi"} {
		t.Run(name, func(t *testing.T) {
			_, err := NewOptions(Options{AgentName: name, APIKey: "test-api-key", Branch: "test-branch"})
			var usageErr UsageError
			require.ErrorAs(t, err, &usageErr)
		})
	}
}

func TestNewOptionsGeneratesBranchWhenOmitted(t *testing.T) {
	opts, err := NewOptions(Options{
		AgentName: "codex",
		Prompt:    "create a file",
	})
	require.NoError(t, err)

	require.Regexp(t, `^[a-z]{8}-[0-9]{6}$`, opts.Branch)
}

func TestNewOptionsResolvesSupportedAgentImagePaths(t *testing.T) {
	image := filepath.Join(t.TempDir(), "mockup.png")
	require.NoError(t, os.WriteFile(image, []byte("not a real image"), 0o600))

	for _, name := range []string{"codex", "claude"} {
		t.Run(name, func(t *testing.T) {
			opts, err := NewOptions(Options{AgentName: name, Prompt: "inspect it", Images: []string{image}})
			require.NoError(t, err)
			{
				got, want := opts.Images, []string{image}
				require.Equal(t, len(want), len(got))
				require.Equal(t, want[0], got[0])
			}
		})
	}
}

func TestNewOptionsRejectsNonRegularSupportedAgentImages(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"codex", "claude"} {
		for _, image := range []string{filepath.Join(dir, "missing.png"), dir} {
			t.Run(name+"/"+filepath.Base(image), func(t *testing.T) {
				_, err := NewOptions(Options{AgentName: name, Prompt: "inspect it", Images: []string{image}})
				var usageErr UsageError
				require.ErrorAs(t, err, &usageErr)
			})
		}
	}
}

func TestNewOptionsIgnoresImagesForUnsupportedAgents(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.png")
	for _, name := range []string{"opencode", "pi"} {
		t.Run(name, func(t *testing.T) {
			opts, err := NewOptions(Options{AgentName: name, Prompt: "inspect it", Images: []string{missing}})
			require.NoError(t, err)
			{
				got, want := opts.Images, []string{missing}
				require.Equal(t, len(want), len(got))
				require.Equal(t, want[0], got[0])
			}
		})
	}
}
