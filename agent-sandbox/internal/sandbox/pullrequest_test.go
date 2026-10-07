package sandbox

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-sandbox/internal/git"
	"agent-sandbox/internal/github"
)

func TestRequireCommittedPRWorktree(t *testing.T) {
	for _, tc := range []struct {
		name  string
		dirty bool
	}{
		{name: "committed branch"},
		{name: "agent left changes", dirty: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoDir := setupDeleteTestRepo(t)
			worktree := addDeleteTestWorktree(t, repoDir, "feature")
			if tc.dirty {
				if err := os.WriteFile(filepath.Join(worktree.Path, "change.txt"), []byte("pending\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := requireCommittedWorktree(&git.Repo{Dir: worktree.Path})
			if tc.dirty && (err == nil || !strings.Contains(err.Error(), "uncommitted changes")) {
				t.Fatalf("expected uncommitted changes error, got %v", err)
			}
			if !tc.dirty && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPublishAgentPRContent(t *testing.T) {
	for _, tc := range []struct {
		name                                                                       string
		status                                                                     int
		invalid, dirty, existing, unchanged, createFailure, cancelled, advanceBase bool
		wantError, wantOutput                                                      string
		wantPushed, wantConsumed, wantCreated                                      bool
		reviewers                                                                  []string
		recovered                                                                  bool
		reviewRequests                                                             string
		inspectFailure                                                             bool
		prDisabled, push                                                           bool
		cancelInspection                                                           bool
	}{
		{name: "new PR", wantPushed: true, wantConsumed: true, wantCreated: true, wantOutput: "Pull request ready for review"},
		{name: "new PR with reviewers", reviewers: []string{"Alice", "bob"}, wantPushed: true, wantConsumed: true, wantCreated: true, wantOutput: "Pull request ready for review"},
		{name: "PR disabled ignores reviewers", reviewers: []string{"Alice"}, prDisabled: true, wantOutput: "Skipping commit and push"},
		{name: "push only ignores reviewers", reviewers: []string{"Alice"}, prDisabled: true, push: true, wantPushed: true},
		{name: "existing PR keeps reviewers", reviewers: []string{"Alice", "bob"}, existing: true, wantPushed: true, wantConsumed: true, wantOutput: "Existing pull request"},
		{name: "no differences with reviewers", reviewers: []string{"Alice"}, unchanged: true, wantConsumed: true, wantOutput: "No changes to review"},
		{name: "failed agent with reviewers", reviewers: []string{"Alice"}, status: 7, wantOutput: "Skipping publication"},
		{name: "cancelled with reviewers", reviewers: []string{"Alice"}, cancelled: true, wantError: "context canceled"},
		{name: "recover without reviewers", createFailure: true, recovered: true, wantPushed: true, wantConsumed: true, wantCreated: true, wantOutput: "Existing pull request"},
		{name: "recover all reviewers", reviewers: []string{"Alice", "bob"}, createFailure: true, recovered: true, reviewRequests: `{"reviewRequests":[{"__typename":"User","login":"alice"},{"__typename":"User","login":"BOB"}]}`, wantPushed: true, wantConsumed: true, wantCreated: true, wantOutput: "Existing pull request"},
		{name: "recover missing reviewer", reviewers: []string{"Alice", "bob"}, createFailure: true, recovered: true, reviewRequests: `{"reviewRequests":[{"__typename":"User","login":"alice"}]}`, wantPushed: true, wantConsumed: true, wantCreated: true, wantError: "review requests are missing for: bob"},
		{name: "recover no reviewers", reviewers: []string{"Alice", "bob"}, createFailure: true, recovered: true, reviewRequests: `{"reviewRequests":[]}`, wantPushed: true, wantConsumed: true, wantCreated: true, wantError: "review requests are missing for: Alice, bob"},
		{name: "recover inspection fails", reviewers: []string{"Alice"}, createFailure: true, recovered: true, inspectFailure: true, wantPushed: true, wantConsumed: true, wantCreated: true, wantError: "checking reviewers on https://github.com/owner/repo/pull/1"},
		{name: "cancel during recovery inspection", reviewers: []string{"Alice"}, createFailure: true, recovered: true, cancelInspection: true, wantPushed: true, wantConsumed: true, wantCreated: true, wantError: "context canceled"},
		{name: "recover inspection malformed", reviewers: []string{"Alice"}, createFailure: true, recovered: true, reviewRequests: `{`, wantPushed: true, wantConsumed: true, wantCreated: true, wantError: "reading PR review requests"},
		{name: "base advances during session", advanceBase: true, wantPushed: true, wantConsumed: true, wantCreated: true, wantOutput: "Pull request ready for review"},
		{name: "existing PR", existing: true, wantPushed: true, wantConsumed: true, wantOutput: "Existing pull request (draft)"},
		{name: "no differences", unchanged: true, wantConsumed: true, wantOutput: "No changes to review"},
		{name: "invalid artifact", invalid: true, wantError: "reading agent PR text; branch has not been pushed"},
		{name: "dirty worktree", dirty: true, wantConsumed: true, wantError: "uncommitted changes"},
		{name: "failed agent", status: 7, wantOutput: "Skipping publication"},
		{name: "cancelled", cancelled: true, wantError: "context canceled"},
		{name: "create failure", createFailure: true, wantPushed: true, wantConsumed: true, wantCreated: true, wantError: "creating PR after successful push"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoDir := setupDeleteTestRepo(t)
			repo := &git.Repo{Dir: repoDir}
			baseBranch, err := repo.CurrentBranch(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			remote := t.TempDir()
			runGit(t, remote, "init", "--bare", "--quiet")
			runGit(t, repoDir, "remote", "add", "origin", remote)
			runGit(t, repoDir, "push", "origin", baseBranch)
			record := addDeleteTestWorktree(t, repoDir, "feature")
			record.BaseBranch = baseBranch
			worktree := repo.At(record.Path)
			base, err := worktree.FetchBase(context.Background(), baseBranch)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.unchanged {
				if err := os.WriteFile(filepath.Join(record.Path, "change.txt"), []byte("change"), 0o600); err != nil {
					t.Fatal(err)
				}
				runGit(t, record.Path, "add", "change.txt")
				runGit(t, record.Path, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "change")
			}
			if tc.advanceBase {
				// Advancing origin to the head would hide every change if publication
				// fetched again instead of using the snapshot supplied to the agent.
				runGit(t, record.Path, "push", "origin", "HEAD:"+baseBranch)
			}
			if tc.dirty {
				if err := os.WriteFile(filepath.Join(record.Path, "pending.txt"), []byte("pending"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			artifact := filepath.Join(record.Path, prContentFile)
			data := `{"title":"Fix login","body":"Changes\n\nTests passed"}`
			if tc.invalid {
				data = `{`
			}
			if !tc.prDisabled {
				if err := os.WriteFile(artifact, []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			fakeDir := t.TempDir()
			logPath := filepath.Join(fakeDir, "calls")
			bodyPath := filepath.Join(fakeDir, "body")
			script := `#!/bin/sh
printf '%s\n' "$@" >> "$PR_TEST_LOG"
if [ "$2" = list ]; then
  if [ "$PR_TEST_EXISTING" = 1 ] || { [ "$PR_TEST_RECOVERED" = 1 ] && [ -f "$PR_TEST_BODY" ]; }; then
    printf '[{"url":"https://github.com/owner/repo/pull/1","isDraft":true,"headRefName":"feature","baseRefName":"%s","headRepositoryOwner":{"login":"owner"},"headRepository":{"name":"repo"}}]\n' "$PR_TEST_BASE"
  else
    printf '[]\n'
  fi
elif [ "$2" = view ]; then
  if [ "$PR_TEST_CANCEL_INSPECTION" = 1 ]; then
    printf 'started' > "$PR_TEST_INSPECT_STARTED"
    exec sleep 30
  fi
  if [ "$PR_TEST_INSPECT_FAILURE" = 1 ]; then exit 1; fi
  printf '%s' "$PR_TEST_REVIEW_REQUESTS"
else
  cat > "$PR_TEST_BODY"
  if [ "$PR_TEST_FAILURE" = 1 ]; then exit 1; fi
  printf 'https://github.com/owner/repo/pull/1\n'
fi
`
			if err := os.WriteFile(filepath.Join(fakeDir, "gh"), []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", fakeDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("PR_TEST_LOG", logPath)
			t.Setenv("PR_TEST_BODY", bodyPath)
			t.Setenv("PR_TEST_BASE", baseBranch)
			t.Setenv("PR_TEST_EXISTING", "0")
			t.Setenv("PR_TEST_FAILURE", "0")
			t.Setenv("PR_TEST_RECOVERED", "0")
			t.Setenv("PR_TEST_INSPECT_FAILURE", "0")
			t.Setenv("PR_TEST_REVIEW_REQUESTS", tc.reviewRequests)
			inspectStarted := filepath.Join(fakeDir, "inspection-started")
			t.Setenv("PR_TEST_INSPECT_STARTED", inspectStarted)
			t.Setenv("PR_TEST_CANCEL_INSPECTION", "0")
			if tc.cancelInspection {
				t.Setenv("PR_TEST_CANCEL_INSPECTION", "1")
			}
			if tc.recovered {
				t.Setenv("PR_TEST_RECOVERED", "1")
			}
			if tc.inspectFailure {
				t.Setenv("PR_TEST_INSPECT_FAILURE", "1")
			}
			if tc.existing {
				t.Setenv("PR_TEST_EXISTING", "1")
			}
			if tc.createFailure {
				t.Setenv("PR_TEST_FAILURE", "1")
			}
			client, err := github.New(repoDir, "https://github.com/owner/repo.git")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if tc.cancelInspection {
				// Wait for the recovery command to start, so cancellation exercises
				// the publication error wrapper rather than its early context check.
				go func() {
					ticker := time.NewTicker(time.Millisecond)
					defer ticker.Stop()
					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							if _, err := os.Stat(inspectStarted); err == nil {
								cancel()
								return
							}
						}
					}
				}()
			}
			if tc.cancelled {
				cancel()
			}
			var out bytes.Buffer
			err = publishResult(ctx, Options{PR: !tc.prDisabled, Push: tc.push, Branch: "feature", prBase: base, Reviewers: tc.reviewers}, record, repo, client, tc.status, &out)
			if tc.cancelInspection && (!errors.Is(err, context.Canceled) || out.Len() != 0) {
				t.Fatalf("cancelled recovery error = %v, output = %q", err, out.String())
			}
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want %q", err, tc.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if tc.wantOutput != "" && !strings.Contains(out.String(), tc.wantOutput) {
				t.Fatalf("output = %q, want %q", out.String(), tc.wantOutput)
			}
			_, statErr := os.Lstat(artifact)
			if !tc.prDisabled && os.IsNotExist(statErr) != tc.wantConsumed {
				t.Fatalf("artifact removal = %v, want %t", statErr, tc.wantConsumed)
			}
			refs, err := exec.Command("git", "-C", remote, "show-ref", "--verify", "refs/heads/feature").CombinedOutput()
			if (err == nil) != tc.wantPushed {
				t.Fatalf("remote feature ref = %s, %v; want pushed %t", refs, err, tc.wantPushed)
			}
			if tc.wantPushed {
				paths, err := exec.Command("git", "-C", remote, "ls-tree", "--name-only", "feature").Output()
				if err != nil || strings.Contains(string(paths), prContentFile) {
					t.Fatalf("PR artifact entered remote commit: %s, %v", paths, err)
				}
			}
			calls, _ := os.ReadFile(logPath)
			if tc.prDisabled && len(calls) != 0 {
				t.Fatalf("unexpected GitHub calls without PR mode: %s", calls)
			}
			if strings.Contains(string(calls), "create\n") != tc.wantCreated {
				t.Fatalf("gh calls = %s, want create %t", calls, tc.wantCreated)
			}
			wantInspection := tc.recovered && len(tc.reviewers) > 0
			if strings.Contains(string(calls), "view\n") != wantInspection {
				t.Fatalf("gh calls = %s, want reviewer inspection %t", calls, wantInspection)
			}
			if strings.Contains(string(calls), "edit\n") || strings.Contains(string(calls), "api\n") {
				t.Fatalf("unexpected reviewer mutation: %s", calls)
			}
			for _, reviewer := range tc.reviewers {
				if strings.Contains(string(calls), "--reviewer\n"+reviewer+"\n") != tc.wantCreated {
					t.Fatalf("gh calls = %s, want reviewer %s passed %t", calls, reviewer, tc.wantCreated)
				}
			}
			if tc.wantCreated {
				body, err := os.ReadFile(bodyPath)
				if err != nil || string(body) != "Changes\n\nTests passed" || !strings.Contains(string(calls), "Fix login\n") {
					t.Fatalf("gh content = %q, calls = %s, error = %v", body, calls, err)
				}
			}
		})
	}
}
