# Push and pull requests

Choose what should happen when the agent finishes:

| Mode | Result |
| --- | --- |
| No publishing flags | The agent is instructed to leave changes uncommitted in its worktree. |
| `--push` / `-p` | The agent commits; the host pushes the branch to `origin`. |
| `--pr` | The agent commits and prepares PR text; the host pushes and creates or reuses a GitHub PR. |

For either publishing mode, configure your Git commit identity on the host
and ensure you can push to `origin`. The agent uses that identity inside
the container. Git push and GitHub operations happen on the host.

## Push a branch

```bash
agent-sandbox run -b add-test -a claude --push \
  -q "add a regression test for the login redirect"
```

Add `-c "Fix login redirect"` to specify the exact commit message. Otherwise,
the agent chooses a message from its actual changes.

## Create a GitHub pull request

Install GitHub CLI and authenticate it on the host for the server used by
`origin`. For github.com:

```bash
gh auth login
gh auth status
```

Start from a named local branch that exists on `origin`. Its name becomes the
recorded PR base; it must differ from the task's branch. The origin fetch and
push URLs must identify the same GitHub repository, with one push destination.

```bash
agent-sandbox run -b fix-login -a codex --pr \
  -q "fix the login redirect loop and add a regression test"
```

`--pr` includes the push; you do not need `--push`. The agent prepares the
title and body during the same session, using the full branch comparison.
A new PR is created ready for review, rather than as a draft.

If an open PR already exists for the same repository, head branch, and base,
the tool prints its URL and preserves its title, body, and draft status.

## Update the PR

```bash
agent-sandbox resume -b fix-login -a codex --pr \
  -q "address the review feedback and run the relevant tests"
```

The original base is retained for later resumes. Detached starts and older
records without a stored base cannot use `--pr`.

## Request reviewers

Add plain GitHub usernames, without `@`, to the `run` section:

```json
{
  "agent": "codex",
  "run": {
    "pr": true,
    "reviewers": ["alice", "bob"]
  }
}
```

Then supply your prompt with `agent-sandbox run -q "your task"`.
Reviewer requests apply only when creating a new PR through `run` or `run-tui`.
They do not modify existing PRs and are not supported in `resume`.

