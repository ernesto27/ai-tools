package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrentBranch(t *testing.T) {
	for _, tc := range []struct {
		name   string
		branch string
		tag    bool
		nested bool
		detach bool
		cancel bool
	}{
		{name: "named branch", branch: "main"},
		{name: "branch and tag share name", branch: "main", tag: true},
		{name: "invocation from subdirectory with matching tag", branch: "main", tag: true, nested: true},
		{name: "nested branch", branch: "feature/login"},
		{name: "heads prefix is part of branch name", branch: "heads/main", tag: true},
		{name: "detached HEAD", detach: true},
		{name: "cancelled context", cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := openTestRepo(t)
			if tc.branch != "" && tc.branch != "main" {
				runTestGit(t, repo.Dir, "switch", "-c", tc.branch)
			}
			if tc.tag {
				runTestGit(t, repo.Dir, "tag", tc.branch)
			}
			if tc.detach {
				runTestGit(t, repo.Dir, "switch", "--detach")
			}
			if tc.nested {
				dir := filepath.Join(repo.Dir, "nested", "directory")
				require.NoError(t, os.MkdirAll(dir, 0o700))
				var err error
				repo, err = Open(dir)
				require.NoError(t, err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			branch, err := repo.CurrentBranch(ctx)
			if tc.cancel {
				require.ErrorIs(t, err, context.Canceled)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.branch, branch)
		})
	}
}

func TestPROriginURLs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pushURL string
		second  bool
		missing bool
	}{
		{name: "default push URL"},
		{name: "separate push URL", pushURL: "git@github.com:other/repo.git"},
		{name: "multiple push URLs rejected", pushURL: "git@github.com:other/repo.git", second: true},
		{name: "missing origin", missing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := openTestRepo(t)
			const origin = "https://github.com/owner/repo.git"
			if !tc.missing {
				runTestGit(t, repo.Dir, "remote", "add", "origin", origin)
			}
			if tc.pushURL != "" {
				runTestGit(t, repo.Dir, "config", "remote.origin.pushurl", tc.pushURL)
			}
			if tc.second {
				runTestGit(t, repo.Dir, "config", "--add", "remote.origin.pushurl", origin)
			}
			fetch, push, err := repo.OriginURLs(context.Background())
			if tc.missing || tc.second {
				require.Error(t, err)
				return
			}
			wantPush := origin
			if tc.pushURL != "" {
				wantPush = tc.pushURL
			}
			require.NoError(t, err)
			require.Equal(t, origin, fetch)
			require.Equal(t, wantPush, push)
		})
	}
}

func TestCommitPendingAndPushHead(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pending bool
	}{
		{name: "new changes committed", pending: true},
		{name: "clean branch with existing unpushed commit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, remote := prTestRemote(t)
			runTestGit(t, repo.Dir, "switch", "-c", "feature/login")
			writePRTestFile(t, repo.Dir, "fix.txt", "fix\n")
			if !tc.pending {
				runTestGit(t, repo.Dir, "add", "-A")
				runTestGit(t, repo.Dir, "commit", "-m", "already committed")
			}
			ctx := context.Background()
			require.NoError(t, repo.CommitPending(ctx, "fix login"))
			head := strings.TrimSpace(runGitOutput(t, repo.Dir, "rev-parse", "HEAD"))
			require.NoError(t, repo.CommitPending(ctx, "must not create an empty commit"))
			require.Equal(t, head, strings.TrimSpace(runGitOutput(t, repo.Dir, "rev-parse", "HEAD")))
			if tc.pending {
				require.Equal(t, "fix login", strings.TrimSpace(runGitOutput(t, repo.Dir, "log", "-1", "--format=%B")))
			}
			require.NoError(t, repo.PushHead(ctx))
			require.Equal(t, head, strings.TrimSpace(runGitOutput(t, remote, "rev-parse", "refs/heads/feature/login")))
			require.Equal(t, "origin/feature/login", strings.TrimSpace(runGitOutput(t, repo.Dir, "rev-parse", "--abbrev-ref", "@{upstream}")))
		})
	}
}

func TestReviewComparisonUsesFullMergeBaseDiff(t *testing.T) {
	repo, _ := prTestRemote(t)
	runTestGit(t, repo.Dir, "branch", "feature")
	writePRTestFile(t, repo.Dir, "base-only.txt", "base advanced\n")
	runTestGit(t, repo.Dir, "add", "-A")
	runTestGit(t, repo.Dir, "commit", "-m", "base-only commit")
	require.NoError(t, repo.PushHead(context.Background()))
	runTestGit(t, repo.Dir, "switch", "feature")
	for _, file := range []string{"first.txt", "second.txt"} {
		writePRTestFile(t, repo.Dir, file, "review "+file+"\n")
		runTestGit(t, repo.Dir, "add", file)
		runTestGit(t, repo.Dir, "commit", "-m", "add "+file)
	}
	writePRTestFile(t, repo.Dir, "binary.dat", "binary\x00data\xff")
	runTestGit(t, repo.Dir, "add", "binary.dat")
	runTestGit(t, repo.Dir, "commit", "-m", "add binary")
	writePRTestFile(t, repo.Dir, "caller-pending.txt", "must not enter comparison\n")
	base, err := repo.FetchBase(context.Background(), "main")
	require.NoError(t, err)
	comparison, changed, err := repo.ReviewComparison(context.Background(), base)
	require.NoError(t, err)
	require.True(t, changed, "changed = %t, error = %v", changed, err)
	for _, name := range []string{"first.txt", "second.txt", "binary.dat"} {
		assert.Contains(t, comparison.Diff, name)
		assert.Contains(t, comparison.ChangedPaths, name)
		assert.Contains(t, comparison.Statistics, name)
	}
	require.Contains(t, comparison.Diff, "Binary files")
	require.NotContains(t, comparison.Diff, "binary\x00data")
	for _, excluded := range []string{"caller-pending.txt", "base-only.txt"} {
		assert.NotContains(t, comparison.Diff, excluded)
		assert.NotContains(t, comparison.ChangedPaths, excluded)
	}
	for _, message := range []string{"add first.txt", "add second.txt", "add binary"} {
		assert.Contains(t, comparison.CommitMessages, message)
	}
	require.NotContains(t, comparison.CommitMessages, "base-only commit")
}

func TestReviewComparisonWithoutReviewableChanges(t *testing.T) {
	for _, name := range []string{"identical head", "empty commit", "change reverted"} {
		t.Run(name, func(t *testing.T) {
			repo, _ := prTestRemote(t)
			base, err := repo.FetchBase(context.Background(), "main")
			require.NoError(t, err)
			if name == "empty commit" {
				runTestGit(t, repo.Dir, "commit", "--allow-empty", "-m", "no content changes")
			}
			if name == "change reverted" {
				writePRTestFile(t, repo.Dir, "temporary.txt", "temporary\n")
				runTestGit(t, repo.Dir, "add", "-A")
				runTestGit(t, repo.Dir, "commit", "-m", "temporary change")
				runTestGit(t, repo.Dir, "revert", "--no-edit", "HEAD")
			}
			_, changed, err := repo.ReviewComparison(context.Background(), base)
			require.NoError(t, err)
			require.False(t, changed, "changed = %t, error = %v, want no reviewable changes", changed, err)
		})
	}
}

func TestRemoteBranchRejectsDeletedBaseDespiteTrackingRef(t *testing.T) {
	repo, remote := prTestRemote(t)
	runTestGit(t, remote, "update-ref", "-d", "refs/heads/main")
	require.Contains(t, runGitOutput(t, repo.Dir, "show-ref"), "refs/remotes/origin/main")
	{
		_, err := repo.RemoteBranch(context.Background(), "main")
		require.Error(t, err)
	}
}

func TestPRGitFailuresPreserveLocalWork(t *testing.T) {
	for _, tc := range []struct {
		name string
		hook string
	}{
		{name: "commit rejected", hook: "pre-commit"},
		{name: "push rejected", hook: "pre-receive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, remote := prTestRemote(t)
			before := strings.TrimSpace(runGitOutput(t, repo.Dir, "rev-parse", "HEAD"))
			writePRTestFile(t, repo.Dir, "recoverable.txt", "keep this work\n")
			hookDir := filepath.Join(repo.Dir, ".git", "hooks")
			if tc.hook == "pre-receive" {
				hookDir = filepath.Join(remote, "hooks")
			}
			require.NoError(t, os.WriteFile(filepath.Join(hookDir, tc.hook), []byte("#!/bin/sh\nexit 1\n"), 0o700))
			commitErr := repo.CommitPending(context.Background(), "recoverable change")
			if tc.hook == "pre-commit" {
				require.Error(t, commitErr)
				require.Equal(t, before, strings.TrimSpace(runGitOutput(t, repo.Dir, "rev-parse", "HEAD")))
				require.True(t, repo.HasStagedChanges(), "failed commit discarded staged changes")
			} else {
				require.NoError(t, commitErr)
				committed := strings.TrimSpace(runGitOutput(t, repo.Dir, "rev-parse", "HEAD"))
				require.Error(t, repo.PushHead(context.Background()))
				require.Equal(t, committed, strings.TrimSpace(runGitOutput(t, repo.Dir, "rev-parse", "HEAD")))
				require.Equal(t, before, strings.TrimSpace(runGitOutput(t, remote, "rev-parse", "refs/heads/main")))
			}
			data, err := os.ReadFile(filepath.Join(repo.Dir, "recoverable.txt"))
			require.NoError(t, err)
			require.Equal(t, "keep this work\n", string(data))
		})
	}
}

func prTestRemote(t *testing.T) (*Repo, string) {
	t.Helper()
	repo := openTestRepo(t)
	remote := t.TempDir()
	runTestGit(t, remote, "init", "--bare", "--quiet")
	runTestGit(t, repo.Dir, "remote", "add", "origin", remote)
	require.NoError(t, repo.PushHead(context.Background()))
	return repo, remote
}

func writePRTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}
