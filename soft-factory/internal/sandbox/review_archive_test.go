package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"soft-factory/internal/executionlog"
)

func TestArchiveReviewReportPreservesAgentContent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	output, err := exec.Command("git", "init", "-b", "main").CombinedOutput()
	require.NoError(t, err, "%s", output)

	run, err := executionlog.NewRun("feature/logs", executionlog.CommandContinue, nil)
	require.NoError(t, err)
	defer run.Finish(nil)
	source := "agent-report.md"
	destination := filepath.Join(run.Path(), "05-reviewChanges.md")
	require.True(t, filepath.IsAbs(run.Path()))
	require.Equal(t, "feature-logs", filepath.Base(filepath.Dir(run.Path())))
	projectDirectory := filepath.Dir(filepath.Dir(run.Path()))
	require.Equal(t, filepath.Join(os.Getenv("HOME"), ".config", "software-factory", "logs"), filepath.Dir(projectDirectory))
	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(filepath.Base(projectDirectory), filepath.Base(cwd)+"-"))
	require.True(t, strings.HasPrefix(filepath.Base(run.Path()), "continue-"))
	content := "# Changes\n\n### File: `./a.go`\nThe branch has no changed files.\n" + strings.Repeat("extra output\n", 200)
	require.NoError(t, os.WriteFile(source, []byte(content), 0644))
	require.NoError(t, archiveReport(source, destination, false))
	got, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, content, string(got))
	info, err := os.Stat(destination)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())

	_, err = os.Stat(source)
	require.NoError(t, err)
}

func TestArchiveReviewReportRejectsInvalidSource(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "05-reviewChanges.md")
	for _, source := range []string{filepath.Join(dir, "missing.md"), filepath.Join(dir, "empty.md"), filepath.Join(dir, "link.md")} {
		switch filepath.Base(source) {
		case "empty.md":
			require.NoError(t, os.WriteFile(source, nil, 0600))
		case "link.md":
			require.NoError(t, os.Symlink("empty.md", source))
		}
		assert.Error(t, archiveReport(source, destination, false))

		_, err := os.Stat(destination)
		assert.ErrorIs(t, err, os.ErrNotExist)
	}
}

func TestArchiveReportReplacement(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprint(replace), func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "new.html")
			destination := filepath.Join(dir, "audit.html")
			require.NoError(t, os.WriteFile(source, []byte("<html>new</html>"), 0600))
			require.NoError(t, os.WriteFile(destination, []byte("previous"), 0600))
			err := archiveReport(source, destination, replace)
			want := "previous"
			if replace {
				require.NoError(t, err)
				want = "<html>new</html>"
			} else {
				require.ErrorIs(t, err, os.ErrExist)
			}
			got, err := os.ReadFile(destination)
			require.NoError(t, err)
			require.Equal(t, want, string(got))
			// A rejected replacement must preserve the previous complete report.
			require.NoError(t, os.WriteFile(source, nil, 0600))
			require.Error(t, archiveReport(source, destination, true))
			got, err = os.ReadFile(destination)
			require.NoError(t, err)
			require.Equal(t, want, string(got))
			leftovers, err := filepath.Glob(filepath.Join(dir, ".factory-report-*.tmp"))
			require.NoError(t, err)
			require.Empty(t, leftovers)
		})
	}
}
