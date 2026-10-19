package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfigPathsDoesNotCreateDirectories(t *testing.T) {
	configDir := setupWorktreeState(t)

	config, err := configPaths()
	require.NoError(t, err)
	wantStateFile := filepath.Join(configDir, "agent-sandbox", "worktrees.jsonl")
	require.Equal(t, wantStateFile, config.StateFile)
	wantWorktreesDir := filepath.Join(configDir, "agent-sandbox", "worktrees")
	require.Equal(t, wantWorktreesDir, config.WorktreesDir)
	{
		_, err := os.Stat(filepath.Dir(config.WorktreesDir))
		require.True(t, os.IsNotExist(err), "configuration directory exists before a write: %v", err)
	}
}

func TestRepoSlugScopesSameNamedRepositoriesAndResolvesSymlinks(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "one", "api")
	second := filepath.Join(root, "two", "api")
	require.NoError(t, os.MkdirAll(first, 0o755))
	require.NoError(t, os.MkdirAll(second, 0o755))

	firstSlug := repoSlug(first)
	secondSlug := repoSlug(second)
	require.NotEqual(t, secondSlug, firstSlug)
	require.True(t, strings.HasPrefix(firstSlug, "api-"), "repo slugs = %q, %q, want names prefixed with api-", firstSlug, secondSlug)
	require.True(t, strings.HasPrefix(secondSlug, "api-"), "repo slugs = %q, %q, want names prefixed with api-", firstSlug, secondSlug)

	link := filepath.Join(root, "link")
	require.NoError(t, os.Symlink(first, link))
	require.Equal(t, firstSlug, repoSlug(link))
}

func TestRecordWorktreeAndReadRecords(t *testing.T) {
	configDir := setupWorktreeState(t)

	empty, err := readRecords()
	require.NoError(t, err)
	require.Nil(t, empty)
	{
		_, err := os.Stat(filepath.Join(configDir, "agent-sandbox"))
		require.True(t, os.IsNotExist(err), "state directory exists after a read: %v", err)
	}

	records := []worktreeRecord{
		{Repo: "/repos/one", Path: "/worktrees/one", Branch: "one", Created: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)},
		{Repo: "/repos/two", Path: "/worktrees/two", Branch: "two", Created: time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)},
	}
	for _, record := range records {
		require.NoError(t, recordWorktree(record))
	}

	got, err := readRecords()
	require.NoError(t, err)
	require.Equal(t, records, got)
}

func TestWriteRecordsCreatesStateDirectory(t *testing.T) {
	configDir := setupWorktreeState(t)

	require.NoError(t, writeRecords(nil))
	path := filepath.Join(configDir, "agent-sandbox", "worktrees.jsonl")
	{
		info, err := os.Stat(path)
		require.NoError(t, err)
		require.False(t, info.IsDir(), "state file = (%v, %v), want an existing file", info, err)
	}
}

func TestReadRecordsSkipsMalformedLines(t *testing.T) {
	setupWorktreeState(t)
	record := worktreeRecord{Repo: "/repo", Path: "/worktree", Branch: "feature"}
	require.NoError(t, recordWorktree(record))

	config, err := configPaths()
	require.NoError(t, err)
	path := config.StateFile
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	if _, err := file.WriteString("not json\n"); err != nil {
		file.Close()
		require.NoError(t, err)
	}
	require.NoError(t, file.Close())

	got, err := readRecords()
	require.NoError(t, err)
	require.Equal(t, []worktreeRecord{record}, got)
}

func TestRecordedWorktreesFiltersByResolvedRepository(t *testing.T) {
	setupWorktreeState(t)
	repoDir := t.TempDir()
	repoLink := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.Symlink(repoDir, repoLink))

	mine := []worktreeRecord{
		{Repo: repoDir, Path: "/worktrees/one", Branch: "one"},
		{Repo: repoLink, Path: "/worktrees/two", Branch: "two"},
	}
	for _, record := range append(mine, worktreeRecord{Repo: t.TempDir(), Path: "/worktrees/other", Branch: "other"}) {
		require.NoError(t, recordWorktree(record))
	}

	got, err := recordedWorktrees(repoDir)
	require.NoError(t, err)
	require.Equal(t, mine, got)
}

func TestRecordedWorktreePathFindsOnlyTheCurrentRepositoryBranch(t *testing.T) {
	setupWorktreeState(t)
	repoDir := t.TempDir()
	otherRepo := t.TempDir()
	records := []worktreeRecord{
		{Repo: repoDir, Path: "/worktrees/feature", Branch: "feature"},
		{Repo: otherRepo, Path: "/worktrees/other-feature", Branch: "feature"},
	}
	for _, record := range records {
		require.NoError(t, recordWorktree(record))
	}

	path, err := recordedWorktreePath(repoDir, "feature")
	require.NoError(t, err)
	require.Equal(t, records[0].Path, path)

	path, err = recordedWorktreePath(repoDir, "missing")
	require.NoError(t, err)
	require.Equal(t, "", path)
}

func TestForgetWorktreesRemovesOnlyMatchingRepositoryAndPath(t *testing.T) {
	setupWorktreeState(t)
	repoDir := t.TempDir()
	pathToRemove := filepath.Join(t.TempDir(), "remove")
	pathToKeep := filepath.Join(t.TempDir(), "keep")
	records := []worktreeRecord{
		{Repo: repoDir, Path: pathToRemove, Branch: "remove"},
		{Repo: repoDir, Path: pathToKeep, Branch: "keep"},
		{Repo: t.TempDir(), Path: pathToRemove, Branch: "other-repo"},
	}
	for _, record := range records {
		require.NoError(t, recordWorktree(record))
	}

	require.NoError(t, forgetWorktrees(repoDir, []string{pathToRemove}))

	got, err := readRecords()
	require.NoError(t, err)
	want := []worktreeRecord{records[1], records[2]}
	require.Equal(t, want, got)
}

func TestForgetWorktreesDoesNothingWithoutPaths(t *testing.T) {
	setupWorktreeState(t)
	record := worktreeRecord{Repo: "/repo", Path: "/worktree", Branch: "feature"}
	require.NoError(t, recordWorktree(record))

	require.NoError(t, forgetWorktrees(record.Repo, nil))
	got, err := readRecords()
	require.NoError(t, err)
	require.Equal(t, []worktreeRecord{record}, got)
}

func TestWriteRecordsReplacesState(t *testing.T) {
	setupWorktreeState(t)
	require.NoError(t, recordWorktree(worktreeRecord{Repo: "/repo", Path: "/worktree/old", Branch: "old"}))

	want := []worktreeRecord{{Repo: "/repo", Path: "/worktree/new", Branch: "new"}}
	require.NoError(t, writeRecords(want))
	got, err := readRecords()
	require.NoError(t, err)
	require.Equal(t, want, got)

	require.NoError(t, writeRecords(nil))
	got, err = readRecords()
	require.NoError(t, err)
	require.Nil(t, got)
}

func setupWorktreeState(t *testing.T) string {
	t.Helper()

	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	return configDir
}
