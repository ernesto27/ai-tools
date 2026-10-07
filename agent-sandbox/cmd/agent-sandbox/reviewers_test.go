package main

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestReviewerConfigValidation(t *testing.T) {
	for _, tc := range []struct {
		name, config, wantError string
	}{
		{"omitted", `{"run":{}}`, ""},
		{"empty", `{"run":{"reviewers":[]}}`, ""},
		{"usernames", `{"run":{"reviewers":[" alice ","bob-2","ALICE"]}}`, ""},
		{"root", `{"reviewers":[]}`, `unknown section "reviewers"`},
		{"resume", `{"resume":{"reviewers":[]}}`, "unknown field resume.reviewers"},
		{"null", `{"run":{"reviewers":null}}`, "run.reviewers must be an array"},
		{"scalar", `{"run":{"reviewers":"alice"}}`, "run.reviewers must be an array"},
		{"object", `{"run":{"reviewers":{}}}`, "run.reviewers must be an array"},
		{"number", `{"run":{"reviewers":["alice",2]}}`, "run.reviewers[1] must be a JSON string"},
		{"boolean", `{"run":{"reviewers":[false]}}`, "run.reviewers[0] must be a JSON string"},
		{"null element", `{"run":{"reviewers":[null]}}`, "run.reviewers[0] must be a JSON string"},
		{"empty name", `{"run":{"reviewers":["  "]}}`, "run.reviewers[0] must be a plain GitHub username"},
		{"email", `{"run":{"reviewers":["alice@example.com"]}}`, "run.reviewers[0] must be a plain GitHub username"},
		{"mention", `{"run":{"reviewers":["@alice"]}}`, "run.reviewers[0] must be a plain GitHub username"},
		{"team", `{"run":{"reviewers":["org/team"]}}`, "run.reviewers[0] must be a plain GitHub username"},
		{"comma", `{"run":{"reviewers":["alice,bob"]}}`, "run.reviewers[0] must be a plain GitHub username"},
		{"space", `{"run":{"reviewers":["alice bob"]}}`, "run.reviewers[0] must be a plain GitHub username"},
		{"option", `{"run":{"reviewers":["--draft"]}}`, "run.reviewers[0] must be a plain GitHub username"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile(localConfigFile, []byte(tc.config), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := readLocalConfig()
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want %q", err, tc.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReviewerConfigCommandIsolation(t *testing.T) {
	for _, tc := range []struct {
		command string
		args    []string
		want    []string
		wantPR  bool
	}{
		{command: "run", want: []string{"Alice", "bob-2"}, wantPR: true},
		{command: "run-tui", want: []string{"Alice", "bob-2"}, wantPR: true},
		{command: "resume", wantPR: true},
		{command: "resume-tui", wantPR: true},
		{command: "run", args: []string{"--pr=false"}, want: []string{"Alice", "bob-2"}},
	} {
		t.Run(tc.command+strings.Join(tc.args, ""), func(t *testing.T) {
			t.Chdir(t.TempDir())
			config := `{"agent":"codex","pr":true,"run":{"query":"task","reviewers":[" Alice ","bob-2","alice","BOB-2"]},"resume":{"branch":"feature","query":"continue"}}`
			if err := os.WriteFile(localConfigFile, []byte(config), 0o600); err != nil {
				t.Fatal(err)
			}
			root, _ := newRootCmd()
			cmd, _, err := root.Find([]string{tc.command})
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.ParseFlags(tc.args); err != nil {
				t.Fatal(err)
			}
			if err := cmd.Args(cmd, nil); err != nil {
				t.Fatal(err)
			}
			// Read the same section through a captured options binding to inspect
			// configuration without starting Docker or taking over the terminal.
			var flags runFlags
			probe := &cobra.Command{}
			flags.bind(probe)
			if err := probe.ParseFlags(tc.args); err != nil {
				t.Fatal(err)
			}
			section := strings.TrimSuffix(tc.command, "-tui")
			if err := applyLocalConfig(probe, section, &flags); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(flags.Reviewers, tc.want) || flags.PR != tc.wantPR {
				t.Fatalf("reviewers = %v, PR = %t; want %v, %t", flags.Reviewers, flags.PR, tc.want, tc.wantPR)
			}
			if probe.Flags().Lookup("reviewers") != nil {
				t.Fatal("reviewers must remain JSON-only")
			}
		})
	}
}
