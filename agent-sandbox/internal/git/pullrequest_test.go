package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				var err error
				repo, err = Open(dir)
				if err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			branch, err := repo.CurrentBranch(ctx)
			if tc.cancel {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("error = %v, want cancellation", err)
				}
				return
			}
			if err != nil || branch != tc.branch {
				t.Fatalf("branch = %q, error = %v, want %q", branch, err, tc.branch)
			}
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
				if err == nil {
					t.Fatal("expected invalid origin error")
				}
				return
			}
			wantPush := origin
			if tc.pushURL != "" {
				wantPush = tc.pushURL
			}
			if err != nil || fetch != origin || push != wantPush {
				t.Fatalf("origin = %q/%q, error = %v", fetch, push, err)
			}
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
			if err := repo.CommitPending(ctx, "fix login"); err != nil {
				t.Fatal(err)
			}
			head := strings.TrimSpace(runGitOutput(t, repo.Dir, "rev-parse", "HEAD"))
			if err := repo.CommitPending(ctx, "must not create an empty commit"); err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(runGitOutput(t, repo.Dir, "rev-parse", "HEAD")); got != head {
				t.Fatal("clean worktree created an empty commit")
			}
			if tc.pending && strings.TrimSpace(runGitOutput(t, repo.Dir, "log", "-1", "--format=%B")) != "fix login" {
				t.Fatal("commit message was not preserved")
			}
			if err := repo.PushHead(ctx); err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(runGitOutput(t, remote, "rev-parse", "refs/heads/feature/login")); got != head {
				t.Fatalf("remote head = %s, want %s", got, head)
			}
			if got := strings.TrimSpace(runGitOutput(t, repo.Dir, "rev-parse", "--abbrev-ref", "@{upstream}")); got != "origin/feature/login" {
				t.Fatalf("upstream = %q", got)
			}
		})
	}
}

func TestReviewComparisonUsesFullMergeBaseDiff(t *testing.T) {
	repo, _ := prTestRemote(t)
	runTestGit(t, repo.Dir, "branch", "feature")
	writePRTestFile(t, repo.Dir, "base-only.txt", "base advanced\n")
	runTestGit(t, repo.Dir, "add", "-A")
	runTestGit(t, repo.Dir, "commit", "-m", "base-only commit")
	if err := repo.PushHead(context.Background()); err != nil {
		t.Fatal(err)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	comparison, changed, err := repo.ReviewComparison(context.Background(), base)
	if err != nil || !changed {
		t.Fatalf("changed = %t, error = %v", changed, err)
	}
	for _, name := range []string{"first.txt", "second.txt", "binary.dat"} {
		if !strings.Contains(comparison.Diff, name) || !strings.Contains(comparison.ChangedPaths, name) || !strings.Contains(comparison.Statistics, name) {
			t.Errorf("comparison omits %s", name)
		}
	}
	if !strings.Contains(comparison.Diff, "Binary files") || strings.Contains(comparison.Diff, "binary\x00data") {
		t.Fatal("binary contents were decoded into the text diff")
	}
	for _, excluded := range []string{"caller-pending.txt", "base-only.txt"} {
		if strings.Contains(comparison.Diff, excluded) || strings.Contains(comparison.ChangedPaths, excluded) {
			t.Errorf("comparison includes %s outside PR changes", excluded)
		}
	}
	for _, message := range []string{"add first.txt", "add second.txt", "add binary"} {
		if !strings.Contains(comparison.CommitMessages, message) {
			t.Errorf("comparison omits commit %q", message)
		}
	}
	if strings.Contains(comparison.CommitMessages, "base-only commit") {
		t.Fatal("comparison includes a base-only commit")
	}
}

func TestReviewComparisonWithoutReviewableChanges(t *testing.T) {
	for _, name := range []string{"identical head", "empty commit", "change reverted"} {
		t.Run(name, func(t *testing.T) {
			repo, _ := prTestRemote(t)
			base, err := repo.FetchBase(context.Background(), "main")
			if err != nil {
				t.Fatal(err)
			}
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
			if err != nil || changed {
				t.Fatalf("changed = %t, error = %v, want no reviewable changes", changed, err)
			}
		})
	}
}

func TestRemoteBranchRejectsDeletedBaseDespiteTrackingRef(t *testing.T) {
	repo, remote := prTestRemote(t)
	runTestGit(t, remote, "update-ref", "-d", "refs/heads/main")
	if !strings.Contains(runGitOutput(t, repo.Dir, "show-ref"), "refs/remotes/origin/main") {
		t.Fatal("expected stale tracking ref")
	}
	if _, err := repo.RemoteBranch(context.Background(), "main"); err == nil {
		t.Fatal("deleted origin base was accepted")
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
			if err := os.WriteFile(filepath.Join(hookDir, tc.hook), []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			commitErr := repo.CommitPending(context.Background(), "recoverable change")
			if tc.hook == "pre-commit" {
				if commitErr == nil {
					t.Fatal("rejected commit returned no error")
				}
				if got := strings.TrimSpace(runGitOutput(t, repo.Dir, "rev-parse", "HEAD")); got != before {
					t.Fatal("failed commit changed HEAD")
				}
				if !repo.HasStagedChanges() {
					t.Fatal("failed commit discarded staged changes")
				}
			} else {
				if commitErr != nil {
					t.Fatal(commitErr)
				}
				committed := strings.TrimSpace(runGitOutput(t, repo.Dir, "rev-parse", "HEAD"))
				if err := repo.PushHead(context.Background()); err == nil {
					t.Fatal("rejected push returned no error")
				}
				if got := strings.TrimSpace(runGitOutput(t, repo.Dir, "rev-parse", "HEAD")); got != committed {
					t.Fatal("failed push discarded local commit")
				}
				if got := strings.TrimSpace(runGitOutput(t, remote, "rev-parse", "refs/heads/main")); got != before {
					t.Fatal("rejected push modified remote branch")
				}
			}
			data, err := os.ReadFile(filepath.Join(repo.Dir, "recoverable.txt"))
			if err != nil || string(data) != "keep this work\n" {
				t.Fatalf("recoverable work changed: %q, %v", data, err)
			}
		})
	}
}

func prTestRemote(t *testing.T) (*Repo, string) {
	t.Helper()
	repo := openTestRepo(t)
	remote := t.TempDir()
	runTestGit(t, remote, "init", "--bare", "--quiet")
	runTestGit(t, repo.Dir, "remote", "add", "origin", remote)
	if err := repo.PushHead(context.Background()); err != nil {
		t.Fatal(err)
	}
	return repo, remote
}

func writePRTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
