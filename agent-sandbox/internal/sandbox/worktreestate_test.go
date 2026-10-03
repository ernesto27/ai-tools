package sandbox

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestConfigPathsDoesNotCreateDirectories(t *testing.T) {
	configDir := setupWorktreeState(t)

	config, err := configPaths()
	if err != nil {
		t.Fatal(err)
	}
	wantStateFile := filepath.Join(configDir, "agent-sandbox", "worktrees.jsonl")
	if config.StateFile != wantStateFile {
		t.Fatalf("Config.StateFile = %q, want %q", config.StateFile, wantStateFile)
	}
	wantWorktreesDir := filepath.Join(configDir, "agent-sandbox", "worktrees")
	if config.WorktreesDir != wantWorktreesDir {
		t.Fatalf("Config.WorktreesDir = %q, want %q", config.WorktreesDir, wantWorktreesDir)
	}
	if _, err := os.Stat(filepath.Dir(config.WorktreesDir)); !os.IsNotExist(err) {
		t.Fatalf("configuration directory exists before a write: %v", err)
	}
}

func TestRepoSlugScopesSameNamedRepositoriesAndResolvesSymlinks(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "one", "api")
	second := filepath.Join(root, "two", "api")
	if err := os.MkdirAll(first, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(second, 0o755); err != nil {
		t.Fatal(err)
	}

	firstSlug := repoSlug(first)
	secondSlug := repoSlug(second)
	if firstSlug == secondSlug {
		t.Fatalf("repoSlug(%q) = repoSlug(%q) = %q, want distinct slugs", first, second, firstSlug)
	}
	if !strings.HasPrefix(firstSlug, "api-") || !strings.HasPrefix(secondSlug, "api-") {
		t.Fatalf("repo slugs = %q, %q, want names prefixed with api-", firstSlug, secondSlug)
	}

	link := filepath.Join(root, "link")
	if err := os.Symlink(first, link); err != nil {
		t.Fatal(err)
	}
	if got := repoSlug(link); got != firstSlug {
		t.Fatalf("repoSlug(%q) = %q, want %q", link, got, firstSlug)
	}
}

func TestRecordWorktreeAndReadRecords(t *testing.T) {
	configDir := setupWorktreeState(t)

	empty, err := readRecords()
	if err != nil {
		t.Fatal(err)
	}
	if empty != nil {
		t.Fatalf("readRecords() before writing = %#v, want nil", empty)
	}
	if _, err := os.Stat(filepath.Join(configDir, "agent-sandbox")); !os.IsNotExist(err) {
		t.Fatalf("state directory exists after a read: %v", err)
	}

	records := []worktreeRecord{
		{Repo: "/repos/one", Path: "/worktrees/one", Branch: "one", Created: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)},
		{Repo: "/repos/two", Path: "/worktrees/two", Branch: "two", Created: time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)},
	}
	for _, record := range records {
		if err := recordWorktree(record); err != nil {
			t.Fatal(err)
		}
	}

	got, err := readRecords()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, records) {
		t.Fatalf("readRecords() = %#v, want %#v", got, records)
	}
}

func TestWriteRecordsCreatesStateDirectory(t *testing.T) {
	configDir := setupWorktreeState(t)

	if err := writeRecords(nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configDir, "agent-sandbox", "worktrees.jsonl")
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		t.Fatalf("state file = (%v, %v), want an existing file", info, err)
	}
}

func TestReadRecordsSkipsMalformedLines(t *testing.T) {
	setupWorktreeState(t)
	record := worktreeRecord{Repo: "/repo", Path: "/worktree", Branch: "feature"}
	if err := recordWorktree(record); err != nil {
		t.Fatal(err)
	}

	config, err := configPaths()
	if err != nil {
		t.Fatal(err)
	}
	path := config.StateFile
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("not json\n"); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := readRecords()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []worktreeRecord{record}) {
		t.Fatalf("readRecords() = %#v, want only %#v", got, record)
	}
}

func TestRecordedWorktreesFiltersByResolvedRepository(t *testing.T) {
	setupWorktreeState(t)
	repoDir := t.TempDir()
	repoLink := filepath.Join(t.TempDir(), "repo")
	if err := os.Symlink(repoDir, repoLink); err != nil {
		t.Fatal(err)
	}

	mine := []worktreeRecord{
		{Repo: repoDir, Path: "/worktrees/one", Branch: "one"},
		{Repo: repoLink, Path: "/worktrees/two", Branch: "two"},
	}
	for _, record := range append(mine, worktreeRecord{Repo: t.TempDir(), Path: "/worktrees/other", Branch: "other"}) {
		if err := recordWorktree(record); err != nil {
			t.Fatal(err)
		}
	}

	got, err := recordedWorktrees(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, mine) {
		t.Fatalf("recordedWorktrees() = %#v, want %#v", got, mine)
	}
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
		if err := recordWorktree(record); err != nil {
			t.Fatal(err)
		}
	}

	path, err := recordedWorktreePath(repoDir, "feature")
	if err != nil {
		t.Fatal(err)
	}
	if path != records[0].Path {
		t.Fatalf("recordedWorktreePath() = %q, want %q", path, records[0].Path)
	}

	path, err = recordedWorktreePath(repoDir, "missing")
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Fatalf("recordedWorktreePath() = %q, want empty path", path)
	}
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
		if err := recordWorktree(record); err != nil {
			t.Fatal(err)
		}
	}

	if err := forgetWorktrees(repoDir, []string{pathToRemove}); err != nil {
		t.Fatal(err)
	}

	got, err := readRecords()
	if err != nil {
		t.Fatal(err)
	}
	want := []worktreeRecord{records[1], records[2]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readRecords() after forget = %#v, want %#v", got, want)
	}
}

func TestForgetWorktreesDoesNothingWithoutPaths(t *testing.T) {
	setupWorktreeState(t)
	record := worktreeRecord{Repo: "/repo", Path: "/worktree", Branch: "feature"}
	if err := recordWorktree(record); err != nil {
		t.Fatal(err)
	}

	if err := forgetWorktrees(record.Repo, nil); err != nil {
		t.Fatal(err)
	}
	got, err := readRecords()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []worktreeRecord{record}) {
		t.Fatalf("readRecords() = %#v, want %#v", got, record)
	}
}

func TestWriteRecordsReplacesState(t *testing.T) {
	setupWorktreeState(t)
	if err := recordWorktree(worktreeRecord{Repo: "/repo", Path: "/worktree/old", Branch: "old"}); err != nil {
		t.Fatal(err)
	}

	want := []worktreeRecord{{Repo: "/repo", Path: "/worktree/new", Branch: "new"}}
	if err := writeRecords(want); err != nil {
		t.Fatal(err)
	}
	got, err := readRecords()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readRecords() after replacement = %#v, want %#v", got, want)
	}

	if err := writeRecords(nil); err != nil {
		t.Fatal(err)
	}
	got, err = readRecords()
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("readRecords() after clearing = %#v, want nil", got)
	}
}

func setupWorktreeState(t *testing.T) string {
	t.Helper()

	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	return configDir
}
