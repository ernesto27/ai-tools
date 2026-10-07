# Architecture reference

This document maps the current implementation. [README.md](../README.md)
describes the CLI contract; consult the linked source when changing behavior.

## Package responsibilities

```text
cmd/agent-sandbox -> sandbox -> agent -> docker
                            -> docker
                            -> git
                            -> github
                            -> utils
```

- [cmd/agent-sandbox](../cmd/agent-sandbox/root.go) uses Cobra for dispatch,
  flags, and usage errors. [config.go](../cmd/agent-sandbox/config.go) loads
  `./agent-sandbox.json` from the invocation directory. Explicit CLI flags
  override command-section values, which override shared root values.
  [run_tui.go](../cmd/agent-sandbox/run_tui.go) adds `run-tui` and `resume-tui`
  using the same execution and configuration as `run` and `resume`.
- [internal/sandbox](../internal/sandbox/sandbox.go) orchestrates image preparation,
  worktree creation or reuse, container execution, and publication.
  [Runtime](../internal/sandbox/runtime.go) routes output and execution events
  without creating a separate TUI execution workflow.
- [internal/git](../internal/git/git.go) wraps the Git CLI. `Repo.At(dir)` addresses
  the same repository through a worktree.
- [internal/docker](../internal/docker/run.go) uses the Docker Engine API for
  builds, throwaway probes (`Output`), and attached execution (`Run`).
- [internal/agent](../internal/agent/agent.go) defines agent-specific commands,
  models, authentication, and container options. It depends on Docker option types.
- [internal/github](../internal/github/github.go) wraps host-side GitHub CLI
  authentication and pull request operations, including creation-time reviewer
  requests and read-only confirmation after an ambiguous create failure.
- [internal/utils](../internal/utils/utils.go) provides prompt-file loading.

## Execution and worktree ownership

[Run and Resume](../internal/sandbox/sandbox.go) resolve the repository and
worktree before sharing container execution through `Runtime.execute`.
`Run` creates and registers a worktree; `Resume` requires an existing valid
record and locks that worktree for the session. The TUI commands call these
same entry points.

[config.go](../internal/sandbox/config.go) uses `os.UserConfigDir()` for
`agent-sandbox/worktrees/<repo>-<hash>/<branch>` and `agent-sandbox/worktrees.jsonl`.
On Linux this normally means `~/.config`, honoring `XDG_CONFIG_HOME`.
The slug combines the repository basename with a short hash of its resolved path.
New runs refuse existing worktree paths.

[worktreestate.go](../internal/sandbox/worktreestate.go) records repository,
path, branch, base branch, and creation time. Worktree management selects only
registered sandbox worktrees; manually created worktrees are outside its scope.
A run records its worktree before starting the agent and rolls back creation
if registration fails. Delete commands rewrite the registry.
[worktreelock.go](../internal/sandbox/worktreelock.go) coordinates resume and deletion.

## Authentication and container mounts

`containerOptions` in [sandbox.go](../internal/sandbox/sandbox.go) selects one
of two credential modes:

- Host authentication calls `Agent.Container(home)` to validate and bind mount
  existing configuration. Missing required paths fail rather than being created.
  [Codex](../internal/agent/codex.go) mounts `.codex`; [Claude](../internal/agent/claude.go)
  mounts `.claude` and `.claude.json`; [opencode](../internal/agent/opencode.go)
  mounts configuration, data, and state; [pi](../internal/agent/pi.go) mounts `.pi/agent`.
- JSON `api-key` authentication requires `APIKeyAgent`, implemented by Codex
  and Claude. The key is supplied as `OPENAI_API_KEY` or `ANTHROPIC_API_KEY` in
  the container environment. These runs use a tmpfs home and do not mount host
  agent credentials. There is no CLI `api-key` flag.

The container runs as the invoking UID/GID with the worktree at `/workspace`.
It also mounts the repository's common Git directory at its original absolute
path and receives Git directory settings and the host's commit identity.
That metadata mount is writable; the separate worktree does not isolate shared
Git metadata. Supported image attachments receive separate read-only mounts.
`HostNetwork` enables Docker host networking only when requested.

## Commit and publication flow

[Options.FullPrompt()](../internal/sandbox/options.go) appends mode-specific
instructions. Without `--push` or `--pr`, it forbids staging, committing, and
pushing. With either option, it instructs the agent to stage and commit related
changes inside the container, using `CommitMessage` exactly when supplied or
choosing a short message from the actual changes. It forbids agent-side push
and PR creation and constrains Git commands and metadata changes.

[publishResult](../internal/sandbox/pullrequest.go) skips publication after a
nonzero agent exit. Both publishing modes require a clean worktree; the host
does not create the commit. `--push` alone pushes the recorded branch to `origin`.

For `--pr`, preflight validates GitHub authentication, origin URLs, and a named
recorded base branch. Execution fetches a base snapshot before the agent runs.
The same agent session writes title and body to the untracked
`.agent-sandbox-pr.json` artifact after committing, using the complete branch
comparison. [prcontent.go](../internal/sandbox/prcontent.go) validates and
consumes that artifact before publication. The host checks for reviewable
differences against the snapshot, pushes, and reuses an open PR for the same
head and base or creates a PR ready for review. Combining `--push` and `--pr`
uses this single PR publication flow.

`run.reviewers` is a JSON-only list of GitHub usernames, normalized by the CLI
configuration loader and passed through `Options` to host publication. `run-tui`
uses the same configuration. Reviewer arguments are supplied only for new PR
creation; existing PRs are not modified. `Resume` clears reviewer options at
its entry point, and reviewers are not stored in worktree records or prompts.
After a failed create call with reviewers, recovery also checks outstanding
review requests before reporting success; missing requests remain an error.

## Image lifecycle and cleanup

[embed.go](../embed.go) embeds [Dockerfile](../Dockerfile) so the binary can
build the default image without a checkout. [baseimage.go](../internal/sandbox/baseimage.go)
generates Dockerfiles for supported external bases and selects their derived images.
Image preparation compares the selected agent's installed version with
`npm view <package> version` on every run or resume and rebuilds on a mismatch,
pinning its `BuildArg` and retaining the external base when applicable.

[docker.Run](../internal/docker/run.go) registers the next-exit wait before
starting the container, drains output, and stops and waits on cancellation
using a context derived with `context.WithoutCancel`. Deferred removal also
covers attach/start failures and unexpected wait failures. Preserve these
orderings so an agent cannot outlive the command and hold its worktree open.

## Exit status

[fail()](../cmd/agent-sandbox/main.go) maps `UsageError` to a usage synopsis and
exit 2, cancellation to exit 130, `StatusError` to its status, and other errors
to exit 1. Successful command dispatch returns the agent's own exit code.
Route Cobra flag and argument errors through the existing hooks. Check required
flags in `RunE` rather than `MarkFlagRequired`, which bypasses those hooks.

## Adding an agent

Implement `Agent` under [internal/agent](../internal/agent/agent.go) and add it
to `registry`, whose order determines CLI display order. Add the matching
version argument and npm install to both [Dockerfile](../Dockerfile) and the
external-base template in [baseimage.go](../internal/sandbox/baseimage.go).
`Container` validates and mounts host configuration. Implement `APIKeyAgent`
only if the agent supports isolated key authentication. `DefaultModel()`
returning an empty string leaves model selection to the agent; Codex and Claude
read defaults from [providers.json](../internal/agent/providers.json).
