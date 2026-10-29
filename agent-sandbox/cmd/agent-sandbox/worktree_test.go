package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteWorktreeBranchesJSON(t *testing.T) {
	for _, tt := range []struct {
		name     string
		branches []string
		want     string
	}{
		{"nil", nil, "[]"},
		{"empty", []string{}, "[]"},
		{"branches", []string{"z", "a"}, `[{"branch":"z"},{"branch":"a"}]`},
		{"escaping", []string{`quote"back\slash`}, `[{"branch":"quote\"back\\slash"}]`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			require.NoError(t, writeWorktreeBranchesJSON(&out, tt.branches))
			require.JSONEq(t, tt.want, out.String())
			require.True(t, strings.HasSuffix(out.String(), "\n"))
		})
	}
	require.ErrorIs(t, writeWorktreeBranchesJSON(doctorFailingWriter{}, []string{"feature"}), io.ErrClosedPipe)
}

// The registry is isolated from the user's configuration. Real Git resolves the
// repository; worktree paths are returned from the registry without inspection.
func setupWorktreeListCLI(t *testing.T) (repo, stateFile, live, stale string) {
	t.Helper()
	repo = t.TempDir()
	output, err := exec.Command("git", "-C", repo, "init", "--quiet").CombinedOutput()
	require.NoError(t, err, "%s", output)
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	stateFile = filepath.Join(configDir, "agent-sandbox", "worktrees.jsonl")
	require.NoError(t, os.MkdirAll(filepath.Dir(stateFile), 0o755))
	live = t.TempDir()
	stale = filepath.Join(t.TempDir(), "missing")
	regular := filepath.Join(t.TempDir(), "regular")
	require.NoError(t, os.WriteFile(regular, nil, 0o600))
	var registry bytes.Buffer
	encoder := json.NewEncoder(&registry)
	for _, record := range []map[string]string{
		{"repo": repo, "branch": "z-live", "path": live},
		{"repo": repo, "branch": "stale", "path": stale},
		{"repo": t.TempDir(), "branch": "other", "path": live},
		{"repo": repo, "branch": "file", "path": regular},
		{"repo": repo, "branch": "a-live", "path": live},
	} {
		require.NoError(t, encoder.Encode(record))
	}
	registry.WriteString("malformed line\n")
	require.NoError(t, os.WriteFile(stateFile, registry.Bytes(), 0o600))
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile(localConfigFile, []byte("invalid json"), 0o600))
	return
}

func TestWorktreeListCLI(t *testing.T) {
	for _, mode := range []string{"explicit json", "cwd json", "relative nested json", "text", "explicit false"} {
		t.Run(mode, func(t *testing.T) {
			repo, stateFile, live, stale := setupWorktreeListCLI(t)
			before, err := os.ReadFile(stateFile)
			require.NoError(t, err)
			args := []string{"worktree-list", "--repo", repo}
			switch mode {
			case "explicit json":
				args = append(args, "--json")
			case "cwd json":
				t.Chdir(repo)
				args = []string{"worktree-list", "--json"}
			case "relative nested json":
				nested := filepath.Join(repo, "nested folder")
				require.NoError(t, os.Mkdir(nested, 0o755))
				cwd, err := os.Getwd()
				require.NoError(t, err)
				relative, err := filepath.Rel(cwd, nested)
				require.NoError(t, err)
				args = []string{"worktree-list", "--repo", relative, "--json"}
			case "explicit false":
				args = append(args, "--json=false")
			}
			root, state := newRootCmd()
			var out, stderr bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&stderr)
			root.SetArgs(args)
			require.NoError(t, root.Execute())
			require.Zero(t, state.status)
			require.Empty(t, stderr.String())
			after, err := os.ReadFile(stateFile)
			require.NoError(t, err)
			require.Equal(t, before, after)
			if strings.Contains(mode, "json") {
				require.Equal(t, "[{\"branch\":\"a-live\"},{\"branch\":\"file\"},{\"branch\":\"stale\"},{\"branch\":\"z-live\"}]\n", out.String())
			} else {
				lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
				require.Len(t, lines, 4)
				require.Equal(t, "a-live  "+live, lines[0])
				require.True(t, strings.HasPrefix(lines[1], "file    "))
				require.Equal(t, "stale   "+stale, lines[2])
				require.Equal(t, "z-live  "+live, lines[3])
			}
		})
	}
}

func TestWorktreeListCLIEmptyAndErrors(t *testing.T) {
	for _, kind := range []string{"empty", "missing registry", "all stale", "non Git", "missing repo", "registry error", "symlink loop"} {
		t.Run(kind, func(t *testing.T) {
			repo, stateFile, _, stale := setupWorktreeListCLI(t)
			switch kind {
			case "empty":
				require.NoError(t, os.WriteFile(stateFile, nil, 0o600))
			case "missing registry":
				require.NoError(t, os.Remove(stateFile))
			case "all stale", "symlink loop":
				if kind == "symlink loop" {
					require.NoError(t, os.Symlink(stale, stale))
				}
				data, err := json.Marshal(map[string]string{"repo": repo, "branch": "stale", "path": stale})
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(stateFile, data, 0o600))
			case "non Git":
				repo = t.TempDir()
			case "missing repo":
				repo = filepath.Join(t.TempDir(), "missing")
			case "registry error":
				require.NoError(t, os.Remove(stateFile))
				require.NoError(t, os.Mkdir(stateFile, 0o755))
			}
			root, _ := newRootCmd()
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetArgs([]string{"worktree-list", "--repo", repo, "--json"})
			cmd, err := root.ExecuteC()
			if kind == "all stale" || kind == "symlink loop" {
				require.NoError(t, err)
				require.Equal(t, "[{\"branch\":\"stale\"}]\n", out.String())
			} else if kind == "empty" || kind == "missing registry" {
				require.NoError(t, err)
				require.Equal(t, "[]\n", out.String())
			} else {
				require.Error(t, err)
				require.Empty(t, out.String())
				var stderr bytes.Buffer
				require.Equal(t, 1, reportError(cmd, &stderr, err, 80, false))
				require.Contains(t, stderr.String(), "Error:")
			}
		})
	}
}

func TestWorktreeListUsageAndHelp(t *testing.T) {
	for _, args := range [][]string{
		{"worktree-list", "extra"}, {"worktree-list", "--json=invalid"},
		{"worktree-list", "--repo"}, {"worktree-list", "--unknown"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			root, _ := newRootCmd()
			root.SetArgs(args)
			root.SetOut(io.Discard)
			cmd, err := root.ExecuteC()
			assertUsageError(t, err)
			require.Equal(t, 2, reportError(cmd, io.Discard, err, 80, false))
		})
	}
	root, _ := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"worktree-list", "--help"})
	require.NoError(t, root.Execute())
	require.Contains(t, out.String(), "--repo")
	require.Contains(t, out.String(), "--json")
	require.Contains(t, out.String(), "newest registered first")
}
