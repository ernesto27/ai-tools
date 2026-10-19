package sandbox

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/git"
	"github.com/ernesto27/ai-tools/agent-sandbox/internal/github"
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
				require.NoError(t, os.WriteFile(filepath.Join(worktree.Path, "change.txt"), []byte("pending\n"), 0o600))
			}
			err := requireCommittedWorktree(&git.Repo{Dir: worktree.Path})
			if tc.dirty {
				require.ErrorContains(t, err, "uncommitted changes")
			} else {
				require.NoError(t, err)
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
			require.NoError(t, err)
			remote := t.TempDir()
			runGit(t, remote, "init", "--bare", "--quiet")
			runGit(t, repoDir, "remote", "add", "origin", remote)
			runGit(t, repoDir, "push", "origin", baseBranch)
			record := addDeleteTestWorktree(t, repoDir, "feature")
			record.BaseBranch = baseBranch
			worktree := repo.At(record.Path)
			base, err := worktree.FetchBase(context.Background(), baseBranch)
			require.NoError(t, err)
			if !tc.unchanged {
				require.NoError(t, os.WriteFile(filepath.Join(record.Path, "change.txt"), []byte("change"), 0o600))
				runGit(t, record.Path, "add", "change.txt")
				runGit(t, record.Path, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "change")
			}
			if tc.advanceBase {
				// Advancing origin to the head would hide every change if publication
				// fetched again instead of using the snapshot supplied to the agent.
				runGit(t, record.Path, "push", "origin", "HEAD:"+baseBranch)
			}
			if tc.dirty {
				require.NoError(t, os.WriteFile(filepath.Join(record.Path, "pending.txt"), []byte("pending"), 0o600))
			}
			artifact := filepath.Join(record.Path, prContentFile)
			data := `{"title":"Fix login","body":"Changes\n\nTests passed"}`
			if tc.invalid {
				data = `{`
			}
			if !tc.prDisabled {
				require.NoError(t, os.WriteFile(artifact, []byte(data), 0o600))
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
			require.NoError(t, os.WriteFile(filepath.Join(fakeDir, "gh"), []byte(script), 0o700))
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
			require.NoError(t, err)
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
			if tc.cancelInspection {
				require.ErrorIs(t, err, context.Canceled)
				require.Empty(t, out.String())
			}
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
			} else {
				require.NoError(t, err)
			}
			if tc.wantOutput != "" {
				require.Contains(t, out.String(), tc.wantOutput)
			}
			_, statErr := os.Lstat(artifact)
			if !tc.prDisabled {
				require.Equal(t, tc.wantConsumed, os.IsNotExist(statErr), "artifact removal: %v", statErr)
			}
			refs, err := exec.Command("git", "-C", remote, "show-ref", "--verify", "refs/heads/feature").CombinedOutput()
			require.Equal(t, tc.wantPushed, err == nil, "remote feature ref: %s; error: %v", refs, err)
			if tc.wantPushed {
				paths, err := exec.Command("git", "-C", remote, "ls-tree", "--name-only", "feature").Output()
				require.NoError(t, err)
				require.NotContains(t, string(paths), prContentFile)
			}
			calls, _ := os.ReadFile(logPath)
			if tc.prDisabled {
				require.Empty(t, calls, "unexpected GitHub calls without PR mode")
			}
			require.Equal(t, tc.wantCreated, strings.Contains(string(calls), "create\n"))
			wantInspection := tc.recovered && len(tc.reviewers) > 0
			require.Equal(t, wantInspection, strings.Contains(string(calls), "view\n"))
			require.NotContains(t, string(calls), "edit\n")
			require.NotContains(t, string(calls), "api\n")
			for _, reviewer := range tc.reviewers {
				require.Equal(t, tc.wantCreated, strings.Contains(string(calls), "--reviewer\n"+reviewer+"\n"))
			}
			if tc.wantCreated {
				body, err := os.ReadFile(bodyPath)
				require.NoError(t, err)
				require.Equal(t, "Changes\n\nTests passed", string(body))
				require.Contains(t, string(calls), "Fix login\n")
			}
		})
	}
}
