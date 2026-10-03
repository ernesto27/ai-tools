package agent

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
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
				if err == nil || err.Error() != tt.wantError {
					t.Fatalf("Lookup(%q) error = %v, want %q", tt.lookup, err, tt.wantError)
				}
				return
			}

			if err != nil {
				t.Fatal(err)
			}
			if got := agent.Name(); got != tt.wantName {
				t.Fatalf("Lookup(%q).Name() = %q, want %q", tt.lookup, got, tt.wantName)
			}
		})
	}
}

func TestRegistryNames(t *testing.T) {
	want := []string{"codex", "claude", "opencode", "pi"}

	if got := Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %q, want %q", got, want)
	}
	if got := NamesProse(); got != "codex, claude, opencode or pi" {
		t.Fatalf("NamesProse() = %q", got)
	}

	all := All()
	got := make([]string, 0, len(all))
	for _, agent := range all {
		got = append(got, agent.Name())
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("All() names = %q, want %q", got, want)
	}
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
			if !ok {
				t.Fatalf("All() is missing agent %q", tt.name)
			}
			if got := agent.DefaultModel(); got != tt.want {
				t.Fatalf("%s.DefaultModel() = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestRequireDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config")
	missing := filepath.Join(dir, "missing")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}

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
			if tt.wantError == "" && err != nil {
				t.Fatalf("requireDir(%q) error = %v, want nil", tt.path, err)
			}
			if tt.wantError != "" && (err == nil || err.Error() != tt.wantError) {
				t.Fatalf("requireDir(%q) error = %v, want %q", tt.path, err, tt.wantError)
			}
		})
	}
}

func TestRequireFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.json")
	missing := filepath.Join(dir, "missing")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}

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
			if tt.wantError == "" && err != nil {
				t.Fatalf("requireFile(%q) error = %v, want nil", tt.path, err)
			}
			if tt.wantError != "" && (err == nil || err.Error() != tt.wantError) {
				t.Fatalf("requireFile(%q) error = %v, want %q", tt.path, err, tt.wantError)
			}
		})
	}
}

func TestHostConfigError(t *testing.T) {
	err := hostConfigError("/missing/config", "pi")
	want := "/missing/config does not exist; run pi on the host first"
	if got := err.Error(); got != want {
		t.Fatalf("hostConfigError() = %q, want %q", got, want)
	}
}
