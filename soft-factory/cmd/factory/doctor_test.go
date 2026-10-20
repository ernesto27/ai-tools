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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("not an executable program"), mode))
			}
			cmd := newRootCmd(func(workflowOptions) error {
				require.FailNow(t, "doctor invoked the workflow")
				return nil
			})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"doctor", "--jira", ""})
			err := cmd.Execute()
			if missing != "" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			for _, name := range []string{"agent-sandbox", "git", "docker"} {
				status := "FOUND"
				path := filepath.Join(dir, name)
				if missing == name || missing == "all" {
					status = "MISSING"
					path = "-"
				}
				row := name + strings.Repeat(" ", len("agent-sandbox")-len(name)+2) + status + strings.Repeat(" ", 7-len(status)+2) + path + "\n"
				assert.Contains(t, out.String(), row)
			}
			assert.NotContains(t, out.String(), "\x1b")
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			require.Len(t, entries, 3)
		})
	}
}

func TestDoctorReport(t *testing.T) {
	var out bytes.Buffer
	calls := 0
	results := doctor.Check(func(name string) (string, error) {
		require.Zero(t, out.Len(), "output before checks completed")
		calls++
		if name == "git" {
			return "/bin/git", nil
		}
		return "", exec.ErrNotFound
	})
	require.Equal(t, 3, calls)
	err := writeDoctorReport(&out, results, true)
	require.EqualError(t, err, "missing dependencies: agent-sandbox, docker")
	want := "Dependency checks:\n\nDependency     Status   Path\n-------------  -------  ----\nagent-sandbox  \x1b[31mMISSING\x1b[0m  -\ngit            \x1b[32mFOUND\x1b[0m    /bin/git\ndocker         \x1b[31mMISSING\x1b[0m  -\n\nResult: 1/3 dependencies found.\n"
	assert.Equal(t, want, out.String())
}

type failingDoctorWriter struct{ err error }

func (w failingDoctorWriter) Write([]byte) (int, error) { return 0, w.err }

func TestDoctorOutputError(t *testing.T) {
	want := errors.New("output unavailable")
	err := writeDoctorReport(failingDoctorWriter{want}, nil, false)
	require.ErrorIs(t, err, want)
}

func TestDoctorHelpAndArguments(t *testing.T) {
	for _, args := range [][]string{{"doctor", "--help"}, {"--help"}, {"doctor", "extra"}} {
		cmd := newRootCmd(func(workflowOptions) error {
			require.FailNow(t, "unexpected workflow")
			return nil
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs(args)
		err := cmd.Execute()
		if args[len(args)-1] == "extra" {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Contains(t, out.String(), "doctor")
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
		require.Error(t, cmd.Execute())
		colored := strings.Contains(out.String(), "\x1b[31mMISSING\x1b[0m")
		assert.Equal(t, noColor == "", colored)
	}
}
