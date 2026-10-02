package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"soft-factory/internal/doctor"
)

func TestDoctorCommand(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if runtime.GOOS == "windows" {
		t.Skip("fixtures use Unix executable permissions")
	}
	for _, missing := range []string{"", "agent-sandbox", "git", "docker", "all"} {
		t.Run("missing_"+missing, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			t.Setenv("PATH", dir)
			for _, name := range []string{"agent-sandbox", "git", "docker"} {
				mode := os.FileMode(0755)
				if missing == name || missing == "all" {
					mode = 0644
				}
				// Executing these fixtures would fail; doctor must only look them up.
				if err := os.WriteFile(filepath.Join(dir, name), []byte("not an executable program"), mode); err != nil {
					t.Fatal(err)
				}
			}
			cmd := newRootCmd(func(workflowOptions) error {
				t.Fatal("doctor invoked the workflow")
				return nil
			})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"doctor", "--config", "nonexistent.json", "--jira", ""})
			err := cmd.Execute()
			if (err != nil) != (missing != "") {
				t.Fatalf("error = %v, missing = %q", err, missing)
			}
			for _, name := range []string{"agent-sandbox", "git", "docker"} {
				status := "FOUND"
				path := filepath.Join(dir, name)
				if missing == name || missing == "all" {
					status = "MISSING"
					path = "-"
				}
				row := name + strings.Repeat(" ", len("agent-sandbox")-len(name)+2) + status + strings.Repeat(" ", 7-len(status)+2) + path + "\n"
				if !strings.Contains(out.String(), row) {
					t.Errorf("missing row %q in %q", row, out.String())
				}
			}
			if strings.Contains(out.String(), "\x1b") {
				t.Error("NO_COLOR output has ANSI sequences")
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 3 {
				t.Fatalf("doctor created files: %v, %v", entries, err)
			}
		})
	}
}

func TestDoctorReport(t *testing.T) {
	var out bytes.Buffer
	calls := 0
	results := doctor.Check(func(name string) (string, error) {
		if out.Len() != 0 {
			t.Fatal("output before checks completed")
		}
		calls++
		if name == "git" {
			return "/bin/git", nil
		}
		return "", exec.ErrNotFound
	})
	if calls != 3 {
		t.Fatalf("lookups = %d", calls)
	}
	err := writeDoctorReport(&out, results, true)
	if err == nil || err.Error() != "missing dependencies: agent-sandbox, docker" {
		t.Fatalf("error = %v", err)
	}
	want := "Dependency checks:\n\nDependency     Status   Path\n-------------  -------  ----\nagent-sandbox  \x1b[31mMISSING\x1b[0m  -\ngit            \x1b[32mFOUND\x1b[0m    /bin/git\ndocker         \x1b[31mMISSING\x1b[0m  -\n\nResult: 1/3 dependencies found.\n"
	if out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

type failingDoctorWriter struct{ err error }

func (w failingDoctorWriter) Write([]byte) (int, error) { return 0, w.err }

func TestDoctorOutputError(t *testing.T) {
	want := errors.New("output unavailable")
	err := writeDoctorReport(failingDoctorWriter{want}, nil, false)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}

func TestDoctorHelpAndArguments(t *testing.T) {
	for _, args := range [][]string{{"doctor", "--help"}, {"--help"}, {"doctor", "extra"}} {
		cmd := newRootCmd(func(workflowOptions) error {
			t.Fatal("unexpected workflow")
			return nil
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs(args)
		err := cmd.Execute()
		if args[len(args)-1] == "extra" {
			if err == nil {
				t.Fatal("accepted extra argument")
			}
		} else if err != nil || !strings.Contains(out.String(), "doctor") {
			t.Fatalf("help = %q, error = %v", out.String(), err)
		}
	}
}

func TestDoctorCommandColors(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, noColor := range []string{"", "1"} {
		t.Setenv("NO_COLOR", noColor)
		cmd := newDoctorCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs(nil)
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected missing dependencies")
		}
		colored := strings.Contains(out.String(), "\x1b[31mMISSING\x1b[0m")
		if colored != (noColor == "") {
			t.Errorf("NO_COLOR=%q: output = %q", noColor, out.String())
		}
	}
}
