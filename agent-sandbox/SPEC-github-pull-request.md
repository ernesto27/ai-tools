# Spec: GitHub pull requests from agent-sandbox

Status: Draft for human review. Specification only; implementation has not started.

## Objective

Let an agent-sandbox user finish an isolated coding session with a GitHub
pull request ready for review, without manually committing, pushing, or
writing the PR title and description.

The confirmed intent is a separate `--pr` flag that commits, pushes, and
opens a ready-for-review PR targeting the branch the sandbox started from.
`-p` / `--push` retains its existing behavior. The title and description are
generated automatically.

This is one capability: publishing a sandbox's work as a GitHub PR. Base
branch persistence, text generation, and GitHub integration support that
workflow; they do not need a capability map.

### Proposed assumptions requiring review

- Both `run` and `resume` support `--pr`.
- GitHub operations use an already authenticated `gh` executable on the host.
- The destination is the repository addressed by `origin`, matching today's
  push behavior. Fork workflows and alternate remotes are out of scope.
- Repeated publication reuses an open PR with the same repository, head,
  and base instead of creating duplicates or overwriting its description.
- Newly recorded worktrees remember the original base branch even when
  created without `--pr`, so a later `resume --pr` can use it.
- An older worktree record without a base branch fails clearly under
  `resume --pr`; the tool does not guess from the current or default branch.
- Missing PR prerequisites fail before image builds or agent execution.
- PR publication follows only a successful agent run. Existing `--push`
  behavior on nonzero agent exit codes is not changed by this feature.
- Text generation uses the selected agent and model in a separate sandbox
  invocation, with the same authentication mode as the coding session.

The user chose an AI-written summary of the changes, rather than text
assembled from Git metadata.

## Commands and CLI behavior

```bash
# Create a sandbox and publish its result as a ready-for-review PR.
agent-sandbox run -b fix-login -a codex --pr -q "fix the login redirect loop"

# Continue a recorded sandbox and create or reuse its PR.
agent-sandbox resume -b fix-login -a claude --pr -q "add a regression test"

# Existing commit-and-push behavior, without creating a PR.
agent-sandbox run -b fix-login -a codex -p -q "fix the login redirect loop"

# Disable a JSON PR default for one invocation.
agent-sandbox resume -b fix-login -a codex --pr=false -q "inspect the fix"

# Build and verify the implementation when that phase is authorized.
go build ./...
go build -o agent-sandbox ./cmd/agent-sandbox
go vet ./...
go test ./...

# User setup for the proposed host GitHub CLI dependency.
gh auth login --hostname github.com
gh auth status --active --hostname github.com
```

- Add a boolean long flag `--pr`, defaulting to false, to the shared
  `run` / `resume` flags. No short alias is required.
- `--pr=true` implies committing and pushing, including when `push` is false.
  Accept `--pr -p` without performing either operation twice.
- `--pr=false` disables PR creation; it does not disable an independently
  enabled `--push`.
- Accept a boolean `pr` in the existing `run` and `resume` sections of
  `./agent-sandbox.json`. Explicit `--pr=false` overrides JSON `pr: true`.
  Keep existing section isolation, strict type checks, and CLI precedence.
- `--commit-message` continues to control the Git commit message; absent
  that flag, use the resolved user prompt. PR text is generated separately.
- Exactly one existing prompt source remains required. There are no manual
  PR title/body flags, additional prompt sources, or interactive PR prompts.
- Worktree management commands, help, and version do not require `gh`.

```json
{
  "run": {
    "agent": "codex",
    "query": "fix the login redirect loop",
    "pr": true
  },
  "resume": {
    "branch": "fix-login",
    "agent": "claude",
    "query": "add a regression test",
    "pr": true
  }
}
```

## Base branch and state

1. Capture the named branch in the invocation worktree before creating the
   sandbox. Invocation from a repository subdirectory resolves the same branch.
2. Store it as an optional `base_branch` string on the sandbox's JSONL
   worktree record. Existing records remain readable by every existing verb;
   deletion and state rewrites preserve the new field.
3. `resume --pr` uses that recorded branch, even if the caller has since
   checked out a different branch. Never substitute the GitHub default branch.
4. Reject detached-HEAD starts under `--pr`, missing recorded bases under
   `resume --pr`, identical head/base names, and missing base branches on
   `origin` before agent execution. Runs without PRs still permit their
   existing Git behavior; a detached start has no recorded named base.
5. Keep using `worktrees.jsonl` as the source of truth. Do not discover or
   adopt manually created worktrees as part of this feature.
6. A pre-existing head branch may be reused by `run` under today's rules;
   the recorded base still comes from the invocation branch, and PR contents
   include all changes in that head relative to the base.

## Publication behavior

For `--pr`, validate the host GitHub CLI, its active authentication, the
`origin` GitHub repository, and base/head suitability before costly work.
Bind all GitHub calls explicitly to the `origin` repository; do not let a
GitHub CLI default repository redirect publication elsewhere.

After the agent succeeds:

1. Stage all changes in the sandbox worktree. Commit only if there are staged
   changes, preserving the existing commit-message rules.
2. Compare the sandbox head against the recorded base as it exists on
   `origin`. Use the PR's merge-base comparison for content generation.
   If there are no changes to review, print a clear message, skip PR creation,
   and finish successfully without an empty commit.
3. Push the head to `origin`, setting its upstream. A clean worktree with
   existing unpushed changes is still eligible; do not return early solely
   because there is nothing new to commit.
4. Look up an open PR matching this repository, head repository/branch, and
   base. If one exists, print its URL. Preserve its title and description;
   report an existing draft as a draft without silently changing its state.
5. If none exists, generate nonempty title and description, create the PR
   ready for review, and print its URL. Recheck for an existing matching PR
   after a create failure to recover from a concurrent creation or ambiguous
   network response; otherwise report the failure.

An agent error, nonzero agent status, or cancellation prevents the new PR
publication workflow. Nonzero agent statuses retain their status. Commit,
push, text-generation, or GitHub failures stop subsequent publication steps
and return an error through the existing exit-status protocol. Preserve the
worktree and any already created commit or pushed branch for recovery.
Diagnostics identify the failed step and whether pushing already succeeded.

### Generated PR content

- After pushing and confirming there is no existing matching open PR, invoke
  the selected agent and model once more to write the PR text. Reuse the
  existing image and agent authentication mode; no separate summarizer
  service, model flag, or GitHub credential is required.
- The host prepares inputs from the merge-base comparison of the remote base
  and sandbox head: the complete text diff, changed paths/statuses and diff
  statistics, commit messages, and resolved user instruction. Binary files
  are represented by their paths/statuses, not decoded as text. Existing
  uncommitted edits in the caller's working copy are never inputs.
- Mount prepared inputs read-only and a separate temporary output directory
  writable. Do not expose a writable source worktree or writable Git metadata
  to this generation invocation. No additional prompt images are needed.
- Ask the agent to write one JSON object with string fields `title` and
  `body` into the designated output directory. Parse that artifact on the
  host instead of extracting content from streamed progress logs. It must be
  a regular file inside that directory, not a symlink to another host path.
- Treat repository text and diffs as material to summarize, not operational
  instructions. The generation instruction permits writing the output
  artifact only, and forbids source edits and Git/GitHub operations.
- Generate a concise, single-line title describing the result.
- Generate a description explaining the changes, grounded in the full PR
  comparison, rather than only the latest resume prompt or last commit.
- Include relevant validation evidence only when actually available. Do not
  invent test runs, successful checks, or issue references.
- Keep the sandbox's fixed house rules out of the PR text.
- Malformed or empty generated content is a failure; do not open a PR with
  missing fields or silently substitute unrelated text.
- Do not add generated content to the user's source tree or Git commit.
- Generation nonzero exit, missing output, invalid JSON, invalid field types,
  whitespace-only fields, or a multiline title stops PR creation with a
  generation error. No silent fallback to commit-message text is allowed.
- Do not silently truncate the comparison. If the chosen model cannot process
  the supplied changes, report generation failure and preserve the pushed
  branch. Model-specific chunking is a later refinement, not required here.
- Document that a new PR requires an additional model invocation, with its
  associated time and usage cost; reusing an existing PR skips generation.
- Clean up temporary input/output artifacts after success, failure, or
  cancellation. Keep cancellation and container stop/wait semantics consistent
  with the coding session.

### GitHub CLI contract

Use an explicitly selected repository, head, base, title, and body to avoid
interactive prompts. Send the description through standard input or a
temporary body file with real newlines. Run commands as argument arrays with
`exec.CommandContext`; do not construct shell commands from generated text.

For example, with the generated description on standard input:

```bash
gh pr list --repo OWNER/REPO --state open --head fix-login --base develop --json url,isDraft,headRepositoryOwner,headRepository
gh pr create --repo OWNER/REPO --head fix-login --base develop --title "Fix login redirect loop" --body-file -
```

The GitHub CLI documents explicit `--head` as preventing implicit push/fork
behavior, `--base` as overriding the default target, and `--body-file -` as
reading the body from standard input. A new PR is ready for review when
`--draft` is omitted. See [gh pr create](https://cli.github.com/manual/gh_pr_create).
Open-PR filtering and JSON fields are documented in
[gh pr list](https://cli.github.com/manual/gh_pr_list). Authenticate using the
active account for the target host, without exposing its token; see
[gh auth status](https://cli.github.com/manual/gh_auth_status).

## Tech stack

Use the existing Go 1.26.0 module, Cobra 1.10.2, Docker client
28.5.2, and standard-library `os/exec` and `encoding/json`. The proposed
new host runtime dependency is GitHub CLI (`gh`), required only for PR
publication. No new Go dependency or AI provider SDK is required. Text
generation uses the existing coding-agent integration and Docker image.

## Project structure

- `cmd/agent-sandbox/root.go` and `config.go`: shared flags, JSON defaults,
  precedence, and option translation; adjacent CLI/config tests.
- `internal/sandbox/`: PR orchestration, generated-content handling, and
  base-branch persistence; adjacent table-driven tests.
- `internal/git/`: current-branch discovery, remote/base checks, comparison
  data, and existing commit/push operations; adjacent Git tests.
- Proposed `internal/github/`: thin host `gh` wrapper for prerequisites,
  PR lookup, and creation, with adjacent tests. This is a leaf package;
  it does not import `git`, `docker`, or `agent`.
- `internal/agent/`: existing per-agent invocation and authentication support
  for the text-generation step; no GitHub operations inside implementations.
- `README.md`: Spanish usage, JSON examples, prerequisites, failure recovery,
  and the distinction between `--push` and `--pr`.
- `SPEC-github-pull-request.md`: this specification. The original Bash
  implementation and `../bash/README.md` remain reference material.

Keep dependency direction `cmd` → `sandbox` → leaf packages. `sandbox`
coordinates Git, Docker, agents, and the proposed GitHub wrapper; leaves
remain independent of one another.

## Code style

Use `gofmt`, descriptive names, wrapped errors with concrete step context,
and full-sentence comments explaining decisions. Match existing CLI flag
binding style. This existing publication pattern shows the intended register:

```go
commitMessage := opts.Prompt
if opts.CommitMessage != "" {
	commitMessage = opts.CommitMessage
}

if err := worktree.Commit(commitMessage); err != nil {
	return err
}
return worktree.Push(opts.Branch)
```

Example user-facing results:

```text
Pull request ready for review: https://github.com/OWNER/REPO/pull/123
Existing pull request: https://github.com/OWNER/REPO/pull/123
No changes to review against develop. Skipping pull request creation.
```

## Testing strategy

Use Go's `testing` package and table-driven tests with named `t.Run` cases.
Keep tests adjacent to their packages. Write meaningful behavioral tests
before implementation. Use real temporary Git repositories and local bare
remotes for base selection, diffs, commits, and pushes. Use a fake `gh`
executable or an appropriate narrow command seam for GitHub behavior.
Generation behavior must be testable without real model calls.

Cover these contracts:

- Flag/config combinations, explicit false precedence, both commands, and
  rejection of wrongly typed `pr` values.
- Capture of a non-default base from a subdirectory and correct use after
  the caller switches branches; JSONL round trips and legacy records.
- Detached HEAD, unavailable remote base, same head/base, unsupported origin,
  missing `gh`, and missing authentication fail before expensive side effects.
- New changes commit and push once; clean worktrees with existing changes
  still publish; no reviewable difference skips creation.
- Generated content uses the whole PR diff and stays outside the commit;
  the generation workspace cannot modify source or Git state; temporary
  artifacts are removed. Test all four agents' selected model/auth wiring
  without real model calls. Empty/malformed/missing output, symlink artifacts,
  nonzero generation status, and cancellation fail clearly. Literal quotes,
  newlines, backticks, and shell syntax remain data.
- PR calls use the correct repository/head/base and omit draft creation;
  existing PRs are reused without text/state changes; create-race recovery
  checks the exact head repository as well as its branch name.
- Agent failure and cancellation prevent PR publication. Commit, push,
  generation, and GitHub errors preserve recoverable work and the exit protocol.
- Existing `--push`, worktree management, and help behavior remain intact.

Run `go test ./...`, `go vet ./...`, and `go build ./...` for implementation
validation. There is no numeric coverage target; every success criterion
below requires observable verification. Normal tests require no Docker,
GitHub account, network, or authenticated agent. A real Docker/GitHub run
is an opt-in end-to-end check in a disposable repository.

## Boundaries

- **Always:** Preserve the caller's working copy, sandbox state ownership,
  existing push semantics, and exit-status protocol. Validate PR prerequisites
  before running the agent. Keep GitHub credentials on the host. Run the
  required checks before implementation commits and document new behavior.
- **Ask first:** Additional dependencies beyond the proposed `gh` requirement,
  changes to CI/releases, fork support, alternate remote selection, a manual
  base override, or new PR customization options.
- **Never:** Run GitHub operations inside the coding-agent container, expose
  host GitHub tokens to the agent, merge PRs, force-push, silently choose a
  different base, delete recoverable work after publication failure, or claim
  tests passed without evidence.

Out of scope: automated code review, PR merging, reviewer assignment, labels,
draft creation, other hosting platforms, fork workflows, and manual title/body
entry. Existing draft PRs are reported rather than promoted automatically.

## Success criteria

1. `run --pr` alone commits any pending work, pushes its head to `origin`,
   creates a ready-for-review GitHub PR, and prints its URL.
2. The PR targets the named branch from the invocation worktree, including
   a non-default branch; `resume --pr` retains that target after checkout changes.
3. The generated nonempty title and description describe the full PR changes
   without manual input or fabricated validation results.
4. `--pr -p` publishes once; `-p` alone creates no PR and keeps today's behavior.
5. JSON `pr: true` works for each command and explicit `--pr=false` wins,
   independently of the merged `push` value.
6. A clean worktree with existing reviewable changes can create a PR. With no
   reviewable changes, the command succeeds with an explicit skip message.
7. Repeated publication to an existing open matching PR prints its URL without
   creating duplicates or replacing user-edited text or draft state.
8. PR prerequisites and missing legacy base metadata fail before agent
   execution; ordinary run/resume and management commands still work without `gh`.
9. Agent failures/cancellation prevent PR publication; later publication
   failures preserve work and identify the failed stage.
10. The implementation passes `go test ./...`, `go vet ./...`, and
    `go build ./...`; the Spanish README documents all new behavior.

## Open questions

No unanswered product question remains from the interview. Human review must
confirm or refine the proposed assumptions above, particularly host `gh`,
the separate invocation of the selected agent/model, resume/legacy handling,
and reuse of existing PRs.

The spec-driven-development skill requires human validation of this spec
before Plan, Tasks, or Implement. This document does not authorize creating
a real PR or publishing this repository during specification work.
