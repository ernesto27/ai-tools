package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLookup(t *testing.T) {
	tests := []struct {
		name      string
		lookup    string
		wantName  string
		wantError string
	}{
		{name: "codex", lookup: "codex", wantName: "codex"},
		{name: "claude", lookup: "claude", wantName: "claude"},
		{name: "opencode", lookup: "opencode", wantName: "opencode"},
		{name: "pi", lookup: "pi", wantName: "pi"},
		{
			name:      "unknown agent",
			lookup:    "unknown",
			wantError: "unknown agent: unknown (expected codex, claude, opencode or pi)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := Lookup(tt.lookup)
			if tt.wantError != "" {
				require.EqualError(t, err, tt.wantError)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantName, agent.Name())
		})
	}
}

func TestRegistryNames(t *testing.T) {
	want := []string{"codex", "claude", "opencode", "pi"}

	require.Equal(t, want, Names())
	require.Equal(t, "codex, claude, opencode or pi", NamesProse())

	all := All()
	got := make([]string, 0, len(all))
	for _, agent := range all {
		got = append(got, agent.Name())
	}
	require.Equal(t, want, got)
}

func TestDefaultModels(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "codex", want: "gpt-6.1-sol"},
		{name: "claude", want: "claude-opus-5-5"},
		{name: "opencode", want: ""},
		{name: "pi", want: ""},
	}

	agents := make(map[string]Agent)
	for _, agent := range All() {
		agents[agent.Name()] = agent
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, ok := agents[tt.name]
			require.True(t, ok, "All() is missing agent %q", tt.name)
			require.Equal(t, tt.want, agent.DefaultModel())
		})
	}
}

func TestRequireDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config")
	missing := filepath.Join(dir, "missing")
	require.NoError(t, os.WriteFile(file, nil, 0o600))

	tests := []struct {
		name      string
		path      string
		wantError string
	}{
		{name: "directory", path: dir},
		{name: "file", path: file, wantError: file + " does not exist; run codex on the host first"},
		{name: "missing", path: missing, wantError: missing + " does not exist; run codex on the host first"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := requireDir(tt.path, "codex")
			if tt.wantError == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantError)
			}
		})
	}
}

func TestRequireFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.json")
	missing := filepath.Join(dir, "missing")
	require.NoError(t, os.WriteFile(file, nil, 0o600))

	tests := []struct {
		name      string
		path      string
		wantError string
	}{
		{name: "file", path: file},
		{name: "directory", path: dir, wantError: dir + " does not exist; run claude on the host first"},
		{name: "missing", path: missing, wantError: missing + " does not exist; run claude on the host first"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := requireFile(tt.path, "claude")
			if tt.wantError == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantError)
			}
		})
	}
}

func TestHostConfigError(t *testing.T) {
	err := hostConfigError("/missing/config", "pi")
	want := "/missing/config does not exist; run pi on the host first"
	require.Equal(t, want, err.Error())
}
