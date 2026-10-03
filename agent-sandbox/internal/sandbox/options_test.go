package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

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
			home := t.TempDir()
			t.Setenv("HOME", home)
			if tc.createHostDir {
				if err := os.Mkdir(filepath.Join(home, ".codex"), 0o700); err != nil {
					t.Fatal(err)
				}
			}

			opts, err := NewOptions(Options{
				AgentName: "codex",
				APIKey:    tc.apiKey,
				Branch:    "test-branch",
				Prompt:    "inspect this",
			})
			if err != nil {
				t.Fatal(err)
			}
			got, err := containerOptions(opts, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}

			hostMount := false
			for _, mount := range got.Mounts {
				if mount.Host == filepath.Join(home, ".codex") {
					hostMount = true
				}
			}
			if hostMount != tc.wantHostMount {
				t.Errorf("host Codex mount = %t, want %t", hostMount, tc.wantHostMount)
			}
			keyEnv := slices.Contains(got.Env, "OPENAI_API_KEY="+key)
			if keyEnv != tc.wantKeyEnv {
				t.Errorf("API key environment = %t, want %t", keyEnv, tc.wantKeyEnv)
			}
			if strings.Contains(strings.Join(got.Args, " "), key) {
				t.Error("API key appeared in Codex command arguments")
			}
			provider := slices.Contains(got.Args, `model_provider="agent_sandbox_api"`)
			if provider != tc.wantKeyEnv {
				t.Errorf("API key provider = %t, want %t", provider, tc.wantKeyEnv)
			}
			inMemoryHome := false
			for _, mount := range got.Tmpfs {
				if mount.Path == "/codex-home" {
					inMemoryHome = true
				}
			}
			if inMemoryHome != tc.wantKeyEnv {
				t.Errorf("in-memory Codex home = %t, want %t", inMemoryHome, tc.wantKeyEnv)
			}
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
		{name: "host login", createHostDir: true, wantMounts: 3},
		{name: "API key without host login", apiKey: key, wantMounts: 1, wantKeyEnv: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			if tc.createHostDir {
				if err := os.Mkdir(filepath.Join(home, ".claude"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			opts, err := NewOptions(Options{AgentName: "claude", APIKey: tc.apiKey, Branch: "test-branch", Prompt: "inspect this"})
			if err != nil {
				t.Fatal(err)
			}
			got, err := containerOptions(opts, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}

			if len(got.Mounts) != tc.wantMounts {
				t.Errorf("mount count = %d, want %d", len(got.Mounts), tc.wantMounts)
			}
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
			if hostCredentialMounts != wantHostCredentialMounts {
				t.Errorf("host Claude credential mounts = %d, want %d", hostCredentialMounts, wantHostCredentialMounts)
			}
			keyEnv := slices.Contains(got.Env, "ANTHROPIC_API_KEY="+key)
			if keyEnv != tc.wantKeyEnv {
				t.Errorf("Claude API key environment present = %t, want %t", keyEnv, tc.wantKeyEnv)
			}
			if strings.Contains(strings.Join(got.Args, " "), key) {
				t.Error("API key appeared in Claude command arguments")
			}
			if !slices.Contains(got.Args, "--print") {
				t.Error("Claude key path must run in non-interactive print mode")
			}
			if (len(got.Tmpfs) == 1 && got.Tmpfs[0].Path == "/claude-home") != tc.wantKeyEnv {
				t.Errorf("disposable Claude home = %v, want %t", got.Tmpfs, tc.wantKeyEnv)
			}
			if tc.wantKeyEnv {
				for _, env := range []string{"CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST=1", "CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=0"} {
					if !slices.Contains(got.Env, env) {
						t.Errorf("missing Claude key-mode environment %q", env)
					}
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
			if !errors.As(err, &usageErr) {
				t.Fatalf("error = %v, want UsageError", err)
			}
		})
	}
}

func TestNewOptionsGeneratesBranchWhenOmitted(t *testing.T) {
	opts, err := NewOptions(Options{
		AgentName: "codex",
		Prompt:    "create a file",
	})
	if err != nil {
		t.Fatalf("NewOptions() error = %v", err)
	}

	if !regexp.MustCompile(`^[a-z]{8}-[0-9]{6}$`).MatchString(opts.Branch) {
		t.Fatalf("NewOptions() branch = %q, want eight lowercase letters and six digits", opts.Branch)
	}
}

func TestNewOptionsResolvesSupportedAgentImagePaths(t *testing.T) {
	image := filepath.Join(t.TempDir(), "mockup.png")
	if err := os.WriteFile(image, []byte("not a real image"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"codex", "claude"} {
		t.Run(name, func(t *testing.T) {
			opts, err := NewOptions(Options{AgentName: name, Prompt: "inspect it", Images: []string{image}})
			if err != nil {
				t.Fatal(err)
			}
			if got, want := opts.Images, []string{image}; len(got) != len(want) || got[0] != want[0] {
				t.Fatalf("Images = %q, want %q", got, want)
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
				if !errors.As(err, &usageErr) {
					t.Fatalf("error = %v, want UsageError", err)
				}
			})
		}
	}
}

func TestNewOptionsIgnoresImagesForUnsupportedAgents(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.png")
	for _, name := range []string{"opencode", "pi"} {
		t.Run(name, func(t *testing.T) {
			opts, err := NewOptions(Options{AgentName: name, Prompt: "inspect it", Images: []string{missing}})
			if err != nil {
				t.Fatal(err)
			}
			if got, want := opts.Images, []string{missing}; len(got) != len(want) || got[0] != want[0] {
				t.Fatalf("Images = %q, want ignored input %q", got, want)
			}
		})
	}
}
