# Plan: Resume a Sandbox Worktree

- **Date:** 2026-09-21
- **Domain(s):** Go CLI / container orchestration / Git worktree lifecycle
- **Author:** plan-from-spec (reviewed with the user)
- **Status:** Implemented; documentation completion and integration coverage pending

## 1. Summary

`agent-sandbox resume` starts a new agent session
against an already registered sandbox worktree. The command identifies the
target by its branch/name as displayed by `worktree-list`, leaves the
worktree's commits and uncommitted edits intact, and uses the established
container, optional commit, and optional push flow to make further changes in
that same worktree. A normal run continues to reject existing worktree paths.

## 2. Scope

### In scope

- Add `resume -b <worktree-name>` as a root Cobra subcommand.
- Require `--agent` and one prompt source; retain the normal run flags that
  affect agent execution: `--model`, `--base-image`, `--push`,
  `--commit-message`, `--file-prompt`, and repeatable `--image`.
- Resolve the target only from the current repository's sandbox state records.
- Verify that the recorded target directory still exists before mounting it in
  a container. The sandbox state file, rather than Git worktree discovery, is
  the source of truth for authorization and path lookup.
- Reuse the agent image freshness/base-image setup, container configuration,
  cancellation handling, and publish behavior from a normal run.
- Leave the worktree's existing state record untouched, whether the resumed
  agent succeeds, fails, or is cancelled.
- Hold an advisory per-worktree lock from container launch through publish so
  concurrent resumes cannot edit, stage, commit, or push the same directory.
- Document the command and add unit/integration-level CLI and sandbox tests.

### Out of scope / non-goals

- Accepting filesystem paths as resume identifiers.
- Making a normal run implicitly reuse an existing worktree.
- Recording which agent or model created a worktree; every resumed session
  chooses its agent explicitly.
- Repairing a stale state record, recreating a missing worktree, or adopting a
  worktree created outside `agent-sandbox`.
- Limiting a resumed `--push` commit to only the latest agent session's edits.
- Changing `worktree-list`, delete, editor, Docker image, or credential-mount
  semantics beyond reuse of their existing support code.

## 3. Resolved decisions

| # | Question | Decision |
|---|----------|----------|
| 1 | How is a target selected? | `-b`/`--branch` supplies the branch/name shown by `worktree-list`, not a filesystem path. Resolution is limited to state records belonging to the repository of the invocation directory. |
| 2 | What is the command contract? | `agent-sandbox resume -b <worktree-name> -a <agent> [run flags] (<prompt...> \| -f <prompt-file>)`. |
| 3 | Is the prior agent inferred? | No. `--agent` remains required because records intentionally store only repository, path, branch, and creation time. |
| 4 | Is `--branch` meaningful during resume? | Yes. `-b`/`--branch` is required and identifies the registered worktree to resume. |
| 5 | What happens to existing work? | Preserve committed history and all staged, unstaged, and untracked changes so the new agent continues from them. |
| 6 | What does `--push` commit? | `git add -A` and commit every uncommitted change already present after the session, including earlier-session edits, exactly as the existing publish path does. Without `--push`, leave all changes in place. |
| 7 | What happens after a resume? | Do not create or remove a worktree, and do not append, rewrite, or otherwise alter its state record. It remains resumable until an explicit delete command succeeds. |
| 8 | How are missing or stale records handled? | Fail without container startup, worktree creation, or state mutation when the name is unrecorded or its recorded directory is absent. Error text directs the user to `worktree-list` and, when appropriate, `worktree-delete`. |
| 9 | Do existing run semantics change? | No. A normal run still generates or creates a worktree and still rejects an existing worktree path. |
| 10 | What user documentation is required? | Update the Spanish Go CLI README usage, flag table, examples, and worktree-management section with the resume command and its preservation/`--push` behavior. |
| 11 | Can two sessions resume one worktree? | No. Resume obtains a non-blocking advisory lock beside the worktree and refuses a second session until the first process releases it or exits. |

## 4. Design

### CLI shape

`newRootCmd` registers a `newResumeCmd` child beside the existing worktree
verbs. The subcommand owns its execution flags, including required
`-b`/`--branch`, while `runFlags` keeps the shared agent/model/image/publish
flags and option construction identical for fresh and resumed sessions. Its
argument validator requires exactly one prompt source and preserves the
project's `sandbox.UsageError` protocol.

`runFlags` centralizes the shared flag registration and option construction.
Each command retains only its branch handling: optional branch creation for the
root run versus required existing name for `resume`.

### Sandbox orchestration

`Resume` accepts the
resolved `Options`, the requested worktree name, context, and output writer.
It will:

1. Open the repository from the caller's current directory.
2. Load only its recorded worktrees and find the matching branch/name.
3. Validate that the recorded directory exists before Docker/image work begins.
4. Construct the selected image client, ensure the image using the existing
   `ensureImage` path, derive the existing container options against the
   recorded directory, then call the existing streaming `client.Run`.
5. Acquire the per-worktree advisory lock immediately before starting the
   container; retain it through `publish`, then release it on every return.
6. Call the existing `publish` function with the recorded directory and branch
   after a successful container invocation.

The normal `Run` path remains responsible for branch validation, directory
creation, `git worktree add`, state recording, and rollback. Refactor only
shared setup where it improves clarity; do not let resume reach creation or
recording code.

### State lookup and locking

Keep `worktrees.jsonl` as the source of authorization and path lookup: only a
matching record in the current repository qualifies. A missing directory is a
stale-record failure before Docker work begins; resume intentionally does not
call `git worktree list` to rediscover the target.

`<worktree-path>.agent-sandbox.lock` is a persistent, empty sibling file. It
uses Linux advisory `flock`, so the OS releases the lock if its process exits
or crashes. The file is never removed, avoiding an inode race between lock
waiters; it is outside the Git worktree, so `git add -A` cannot stage it.

## 5. Interfaces & contracts

### Command

```text
agent-sandbox resume -b <worktree-name> -a <codex|claude|opencode|pi> \
  [-m <model>] [-i <image>] [-p] [-c <commit-message>] \
  [--image <file>]... [--] (<prompt...> | -f <prompt-file>)
```

- `-b <worktree-name>` is a required, exact state-record branch match.
- `--agent` and exactly one prompt source are required.
- `--image` remains validated for Codex and ignored by other agents as today.
- The resolved prompt continues to be used as the default commit message when
  `--push` is selected without `--commit-message`.

### Failures

- Unknown name: return a descriptive non-usage error naming the requested
  worktree and directing the user to `worktree-list`.
- Missing directory: return a descriptive stale-record error, do not start
  Docker or mutate state, and direct the user to explicit cleanup with
  `worktree-delete`.
- Locked worktree: return an error that another session is already using the
  recorded branch; do not start a second container or publish operation.
- Agent configuration, Docker, image, container, and publish errors retain
  the existing error and exit-status behavior.

## 6. Behavior & states

```text
resume request
  -> parse/validate required -b name, agent, flags, and one prompt source
  -> open invoking repository
  -> look up matching sandbox state record
     -> absent: fail; no side effects
  -> verify recorded path exists
     -> stale: fail; no side effects
  -> ensure selected image
  -> acquire sibling advisory lock
     -> locked: fail; no side effects
  -> run agent with recorded path mounted at /workspace
  -> agent exit/error/cancellation uses existing container lifecycle
  -> --push? stage all current worktree changes, commit, push
             otherwise leave all current changes in place
  -> preserve the existing state record, release lock
```

The resumed session is deliberately not idempotent: each successful invocation
can change the same worktree. Lookup and stale-target failures are side-effect
free with respect to worktrees and state records.

## 7. Implementation tasks

- [x] Refactor `cmd/agent-sandbox/root.go` enough to share run-option flag
  registration and option assembly without changing root-run syntax, help, or
  behavior.
- [x] Add `newResumeCmd` in `cmd/agent-sandbox/worktree.go`, with the
  documented `Use`, examples, required `-b` worktree name
  and prompt validation, agent completion, and the project-standard
  `UsageError` wrapping.
- [x] Register the new command from `newRootCmd`; keep root and resume branch
  flags local to their respective commands while sharing execution flags.
- [x] Add `sandbox.Resume` plus focused helpers in `internal/sandbox` to find
  the current repository's record, distinguish unknown from stale targets,
  run the existing image/container path against `record.Path`, and call the
  existing `publish` behavior without recording or deleting anything.
- [x] Keep state lookup entirely in `internal/sandbox`; no Git worktree-list
  wrapper is used for resume authorization.
- [x] Add a sibling advisory-lock helper and contention/release tests so only
  one resumed session can use a worktree at a time.
- [x] Add command tests for shared flags/options and missing required resume
  branch while preserving the
  preserved usage-status protocol.
- [ ] Add sandbox/state integration tests using temporary initialized Git
  repositories and registered worktrees for success lookup, repository
  scoping, unknown target, missing directory, state-record preservation, and
  preservation of dirty changes. Keep Docker out of these tests by testing
  validation and setup boundaries directly.
- [ ] Complete the staged `README.md` update in Spanish: synopsis, parameter descriptions,
  resume example, and worktree-management guidance that names are listed by
  `worktree-list`, edits persist, `--push` commits all pending work, and
  explicit delete controls cleanup.
- [x] Run `gofmt`, `go vet ./...`, and `go test ./...`; manually inspect
  generated Cobra help for the root and `resume` subcommand.

## 8. Testing

- **Unit tests**
  - Required `--agent`, exact-one prompt source, and required resume `-b` all
    produce `UsageError`/status-2 behavior.
  - Shared flag plumbing yields the same `Options` values for root-run and
    resume invocations, including file prompts, model, images, commit message,
    push, and base image.
  - State lookup is scoped to the invocation repository and does not mutate
    records for any lookup failure.
  - A second lock attempt fails while the first lock is held; after release,
    the next session acquires the same persistent sibling lock file.

- **Integration tests — pending**
  - Build a temporary Git repository and recorded worktree, make staged,
    unstaged, and untracked edits, then exercise resume-target resolution to
    prove the same path and branch are selected without cleanup or record
    duplication.
  - Verify absent records and manually removed worktree directories fail before
    any Docker setup is invoked.
  - Verify the existing `publish` behavior against a dirty resumed worktree:
    `--push` stages all changes and no-push leaves them present. Use a local
    test remote where push verification is performed; if the current test
    suite's architecture makes container execution non-injectable, test this
    through `publish` and the new pre-container resume helpers rather than
    requiring Docker.

## 9. Acceptance criteria

- `worktree-list` output names can be passed with `resume -b` from the same
  repository, and the agent container receives the matching recorded directory
  as `/workspace`.
- Resuming does not create a new directory, branch, or JSONL record, and the
  target still appears in `worktree-list` afterward.
- Existing committed and uncommitted work is visible during the resumed
  session; without `--push`, all pending work remains in the worktree.
- With `--push`, every pending change is staged, committed with the explicit
  commit message or resolved prompt, and pushed to the recorded branch.
- Unknown, stale, and already-locked targets fail before container execution
  and give actionable `worktree-list`/`worktree-delete` guidance.
- A normal run against an existing branch/path still fails rather than silently
  resuming it.
- Existing CLI exit-status and error-printing conventions remain intact.
- `go vet ./...` and `go test ./...` pass, and documentation matches the
  implemented command syntax.

## 10. Risks & open items

- No product decisions remain open; integration coverage and the staged README
  edit remain implementation follow-ups.
- Resume deliberately trusts the sandbox state record for the path/branch pair.
  The recorded directory's existence is the remaining pre-container check.
- Advisory `flock` is Linux-specific, matching the supported Linux amd64
  release target.
- Existing top-level orchestration is not dependency-injected, so tests should
  isolate Git/state/container-option boundaries and reuse `publish` for Git
  integration coverage instead of requiring a Docker daemon.
