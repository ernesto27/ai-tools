package sandbox

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	sandboxconfig "github.com/ernesto27/ai-tools/agent-sandbox/config"
	"github.com/spf13/cobra"
)

func TestSandboxArgsPrecedence(t *testing.T) {
	const data = `{"branch":"shared","agent":"codex","model":"shared-model","effort":"high","baseImage":"base","push":true,"pr":true,"hn":true,"file":["shared.txt"],"run":{"branch":"run-branch","agent":"claude","model":"run-model","effort":"low","query":"configured query","filePrompt":"task.txt","commitMessage":"fix task","file":[],"image":["one.png","two.png"],"apiKey":"secret-key","reviewers":["alice"]},"resume":{"branch":"resume-branch","model":"","baseImage":"","push":false,"pr":false,"hn":false,"file":["resume.txt"],"image":[]}}`
	var settings sandboxconfig.Config
	if err := json.Unmarshal([]byte(data), &settings); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		args    []string
		strings map[string]string
		bools   map[string]bool
		files   []string
		images  []string
	}{
		{
			name:    "run section and root fields",
			args:    []string{"run"},
			strings: map[string]string{"branch": "run-branch", "agent": "claude", "model": "run-model", "effort": "low", "baseImage": "base", "query": "configured query", "filePrompt": "task.txt", "commitMessage": "fix task"},
			bools:   map[string]bool{"push": true, "pr": true, "hn": true},
			files:   []string{}, images: []string{"one.png", "two.png"},
		},
		{
			name:    "resume keeps explicit empty and false values",
			args:    []string{"resume"},
			strings: map[string]string{"branch": "resume-branch", "agent": "codex", "model": "", "effort": "high", "baseImage": "", "query": "", "filePrompt": "", "commitMessage": ""},
			bools:   map[string]bool{"push": false, "pr": false, "hn": false},
			files:   []string{"resume.txt"}, images: []string{},
		},
		{
			name:    "stage and workflow overrides including prompt source",
			args:    []string{"run", "-a", "pi", "-m", "stage-model", "-f", "generated prompt.txt", "--push=false", "--pr=false", "--branch", ""},
			strings: map[string]string{"branch": "", "agent": "pi", "model": "stage-model", "effort": "low", "baseImage": "base", "query": "", "filePrompt": "generated prompt.txt", "commitMessage": "fix task"},
			bools:   map[string]bool{"push": false, "pr": false, "hn": true},
			files:   []string{}, images: []string{"one.png", "two.png"},
		},
		{
			name:    "query override and values resembling flags",
			args:    []string{"run", "-q", "--agent", "--file", "a,b with spaces.txt", "--image", "cli.png"},
			strings: map[string]string{"branch": "run-branch", "agent": "claude", "model": "run-model", "effort": "low", "baseImage": "base", "query": "--agent", "filePrompt": "", "commitMessage": "fix task"},
			bools:   map[string]bool{"push": true, "pr": true, "hn": true},
			files:   []string{"a,b with spaces.txt"}, images: []string{"cli.png"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			flags := cmd.Flags()
			for _, field := range []struct{ name, short string }{
				{"branch", "b"}, {"agent", "a"}, {"model", "m"}, {"effort", ""},
				{"baseImage", "i"}, {"query", "q"}, {"filePrompt", "f"}, {"commitMessage", "c"},
			} {
				flags.StringP(field.name, field.short, "", "")
			}
			flags.BoolP("push", "p", false, "")
			flags.Bool("pr", false, "")
			flags.Bool("hn", false, "")
			flags.StringArray("file", nil, "")
			flags.StringArray("image", nil, "")
			args := sandboxArgs(settings, tc.args)
			if strings.Contains(strings.Join(args, " "), "secret-key") || strings.Contains(strings.Join(args, " "), "alice") {
				t.Fatalf("unsupported credential or reviewer passed: %q", args)
			}
			if err := flags.Parse(args[1:]); err != nil {
				t.Fatal(err)
			}
			for name, want := range tc.strings {
				if got, err := flags.GetString(name); err != nil || got != want {
					t.Errorf("%s = %q, %v; want %q", name, got, err, want)
				}
			}
			for name, want := range tc.bools {
				if got, err := flags.GetBool(name); err != nil || got != want {
					t.Errorf("%s = %t, %v; want %t", name, got, err, want)
				}
			}
			for name, want := range map[string][]string{"file": tc.files, "image": tc.images} {
				if got, err := flags.GetStringArray(name); err != nil || !reflect.DeepEqual(got, want) {
					t.Errorf("%s = %q, %v; want %q", name, got, err, want)
				}
			}
		})
	}
}
