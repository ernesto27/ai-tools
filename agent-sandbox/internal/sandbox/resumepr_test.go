package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestAppendPRChecklist(t *testing.T) {
	const checklist = "- [x] Fix task validation."
	for _, tc := range []struct {
		name, existing, want string
	}{
		{name: "empty description", want: checklist},
		{name: "existing text", existing: "Original", want: "Original\n\n" + checklist},
		{name: "one trailing newline", existing: "Original\n", want: "Original\n\n" + checklist},
		{name: "existing blank line", existing: "Original\n\n", want: "Original\n\n" + checklist},
		{name: "CRLF preserved", existing: "Original\r\n\r\n", want: "Original\r\n\r\n" + checklist},
		{name: "whitespace preserved", existing: " Original  \n\n\n", want: " Original  \n\n\n" + checklist},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, appendPRChecklist(tc.existing, checklist))
		})
	}
}

func TestPublishResumedPR(t *testing.T) {
	const changed = `{"title":"Ignored title","body":"- [x] Fix task validation.","has_changes":true}`
	const unchanged = `{"title":"Ignored title","body":"- [x] No net file changes.","has_changes":false}`
	const missingDecision = `{"title":"Ignored title","body":"- [x] Fix task validation."}`
	for _, tc := range []struct {
		name, artifact, failure, wantError, wantCalls, wantOutput string
		dirty, wantPush                                           bool
	}{
		{name: "append without host comparison", artifact: changed, wantPush: true, wantCalls: "view\nedit\n", wantOutput: "PR description updated"},
		{name: "agent reports no changes", artifact: unchanged, wantPush: true, wantOutput: "PR description unchanged"},
		{name: "missing decision", artifact: missingDecision, wantError: "must include a boolean has_changes"},
		{name: "dirty worktree", artifact: changed, dirty: true, wantError: "uncommitted changes"},
		{name: "push fails", artifact: changed, failure: "push", wantError: "pushing resumed PR branch"},
		{name: "body read fails after push", artifact: changed, failure: "view", wantPush: true, wantCalls: "view\n", wantError: "reading PR description after successful push"},
		{name: "body edit fails after push", artifact: changed, failure: "edit", wantPush: true, wantCalls: "view\nedit\n", wantError: "updating PR description after successful push"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			repoDir := setupDeleteTestRepo(t)
			repo := &git.Repo{Dir: repoDir}
			base, err := repo.CurrentBranch(ctx)
			require.NoError(t, err)
			remote := t.TempDir()
			runGit(t, remote, "init", "--bare", "--quiet")
			runGit(t, repoDir, "remote", "add", "origin", remote)
			record := addDeleteTestWorktree(t, repoDir, "feature")
			record.BaseBranch = base
			// Keeping HEAD at the base proves resume trusts the agent's decision
			// rather than requiring new commits or a reviewable branch comparison.
			if tc.dirty {
				require.NoError(t, os.WriteFile(filepath.Join(record.Path, "pending.txt"), []byte("pending"), 0o600))
			}
			if tc.failure == "push" {
				require.NoError(t, os.WriteFile(filepath.Join(remote, "hooks", "pre-receive"), []byte("#!/bin/sh\nexit 1\n"), 0o700))
			}
			artifactPath := filepath.Join(record.Path, prContentFile)
			require.NoError(t, os.WriteFile(artifactPath, []byte(tc.artifact), 0o600))

			const prURL = "https://github.com/owner/repo/pull/1"
			const original = "Original description with `quotes` and $(literal text)."
			view, err := json.Marshal(map[string]any{
				"url": prURL, "state": "OPEN", "body": original,
				"headRefName": "feature", "baseRefName": base,
				"headRepositoryOwner": map[string]string{"login": "owner"},
				"headRepository":      map[string]string{"name": "repo"},
			})
			require.NoError(t, err)
			fakeDir := t.TempDir()
			callsPath := filepath.Join(fakeDir, "calls")
			bodyPath := filepath.Join(fakeDir, "body")
			script := `#!/bin/sh
printf '%s\n' "$2" >> "$RESUME_TEST_CALLS"
[ "$#" -eq 7 ] && [ "$1" = pr ] && [ "$3" = "$RESUME_TEST_URL" ] && [ "$4" = --repo ] && [ "$5" = github.com/owner/repo ] || exit 1
git --git-dir "$RESUME_TEST_REMOTE" rev-parse --verify refs/heads/feature >/dev/null || exit 1
case "$2" in
  view)
    [ "$6" = --json ] || exit 1
    [ "$RESUME_TEST_FAILURE" != view ] || exit 1
    printf '%s' "$RESUME_TEST_VIEW"
    ;;
  edit)
    [ "$6" = --body-file ] && [ "$7" = - ] || exit 1
    cat > "$RESUME_TEST_BODY"
    [ "$RESUME_TEST_FAILURE" != edit ] || exit 1
    ;;
  *) exit 1 ;;
esac
`
			require.NoError(t, os.WriteFile(filepath.Join(fakeDir, "gh"), []byte(script), 0o700))
			t.Setenv("PATH", fakeDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("RESUME_TEST_CALLS", callsPath)
			t.Setenv("RESUME_TEST_BODY", bodyPath)
			t.Setenv("RESUME_TEST_REMOTE", remote)
			t.Setenv("RESUME_TEST_URL", prURL)
			t.Setenv("RESUME_TEST_VIEW", string(view))
			t.Setenv("RESUME_TEST_FAILURE", tc.failure)
			client, err := github.New(repoDir, "https://github.com/owner/repo.git")
			require.NoError(t, err)
			var out bytes.Buffer
			err = publishResult(ctx, Options{PR: true, Branch: "feature", prResume: true, prURL: prURL}, record, repo, client, 0, &out)
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
			} else {
				require.NoError(t, err)
			}
			if tc.wantOutput != "" {
				require.Contains(t, out.String(), tc.wantOutput)
			}
			if tc.wantError != "" {
				require.Empty(t, out.String(), "failure printed success output")
			}
			{
				_, err := os.Stat(artifactPath)
				require.True(t, os.IsNotExist(err), "consumed artifact still exists: %v", err)
			}
			pushErr := exec.CommandContext(ctx, "git", "--git-dir", remote, "rev-parse", "--verify", "refs/heads/feature").Run()
			require.Equal(t, tc.wantPush, (pushErr == nil))
			calls, err := os.ReadFile(callsPath)
			if err != nil {
				require.ErrorIs(t, err, os.ErrNotExist)
			}
			require.Equal(t, tc.wantCalls, string(calls))
			body, err := os.ReadFile(bodyPath)
			if strings.Contains(tc.wantCalls, "edit\n") {
				want := original + "\n\n- [x] Fix task validation."
				require.NoError(t, err)
				require.Equal(t, want, string(body))
			} else {
				require.True(t, os.IsNotExist(err), "unexpected edited body: %q, %v", body, err)
			}
		})
	}
}
