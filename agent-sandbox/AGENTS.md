# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`agent-sandbox` is a single Go binary that runs a coding agent (codex, claude, opencode, pi)
inside a Docker container against a **git worktree of its own**, so the agent never touches the
caller's working copy. This directory is the Go port; `../bash/` holds the original shell
implementation and its `README.md` (Spanish) is the user-facing documentation of the CLI —
flags, per-agent model formats, worktree cleanup. Read it before changing user-visible behavior.

The git repository root is two levels up (`better-development/skills`); this module is one
component inside it.

## Commands

```bash
go build ./...                      # compile everything
go build -o agent-sandbox ./cmd/agent-sandbox   # the binary (gitignored)
go vet ./...
go test ./...                       # run the Go test suite
```

There is no Makefile and no linter config. Releases are cut by pushing a tag matching
`agent-sandbox-v*`; `.github/workflows/agent-sandbox-release.yml` at the repo root builds the Linux amd64
archive that `install.sh` downloads.

Running the tool requires Docker and a real git repo, and it will build/rebuild the
`agent-sandbox` image on first use, so a manual run is not a cheap smoke test.

## Test-driven development

Use table-driven test

## Architecture

Dependency direction is strictly one way — `cmd` → `sandbox` → {`git`, `docker`, `agent`} —
and the three leaf packages know nothing about each other:

- **`cmd/agent-sandbox`** — argument dispatch, local JSON defaults, and exit status, built
  on `cobra`. The root command dispatches explicit `run` and `resume` verbs alongside the
  worktree management commands. Only `run` creates a new worktree. `run` and `resume` read
  `./agent-sandbox.json` from the invocation directory; CLI flags override its values.
  `-q`/`--query` supplies an instruction, `-f`/`--file-prompt` reads one from a file,
  and positional instructions are rejected. `-p`/`--push` commits and pushes.
  `fail()` is the single exit path and decides the status from the error's type.
- **`internal/sandbox`** — the orchestration: parse options, ensure the image is current,
  create the worktree, run the container, optionally commit and push.
- **`internal/git`** — thin wrappers over the `git` CLI (`exec.Command`), not a git library.
  `Repo` is addressed by a working directory; `Repo.At(dir)` re-addresses the same repository
  through a worktree.
- **`internal/docker`** — the Docker Engine API client: build, `Output` (a throwaway probe
  container), and `Run` (stream-attached, TTY-aware).
- **`internal/agent`** — per-agent knowledge, behind the `Agent` interface.

`embed.go` at the module root embeds `Dockerfile` into the binary, so the released binary can
build its own image from anywhere with no build context beyond that one file.

### Invariants worth knowing before editing

- **Credentials only ever arrive as bind mounts** of the agent's host config directory. No
  API keys are passed as environment variables, and the agent must already be authenticated on
  the host — a missing config directory is a hard error (`hostConfigError`), never created.
- **The worktree lives under the user's configuration directory**, at
  `~/.config/agent-sandbox/worktrees/<repo>-<hash>/<branch>`. The repository
  basename plus a short hash of its resolved path keeps common branch names in
  unrelated repositories from colliding. Existing paths are refused rather
  than reused.
- **`~/.config/agent-sandbox/worktrees.jsonl` is the source of truth for what the sandbox
  created**, one JSON line per worktree, appended by a run and rewritten by the delete verbs
  (`internal/sandbox/worktreestate.go`). Every worktree verb reads it and nothing else — the
  `git worktree list` parsing is gone — so a worktree the user made by hand is invisible to
  them, and none of them can delete it.
- **Exit status is a protocol.** `UsageError` → print the synopsis, exit 2. `StatusError` →
  print the message only, exit with its status. `context.Canceled` → exit 130. Otherwise the
  agent's own exit code becomes the command's exit code. Everything cobra can reject has to
  be funnelled into `UsageError` to keep that protocol: flag errors through
  `SetFlagErrorFunc` on the root, positional-argument errors through `usageArgs` or
  `runArgs`.
  A required flag is therefore checked in `RunE`, **not** with `MarkFlagRequired`, whose
  error cobra raises outside both hooks and which would otherwise exit 1 with no synopsis.
- **A cancelled run stops its container and waits for it** (`docker.stop`), on a context
  derived with `context.WithoutCancel`, because the agent holds the worktree open and must not
  outlive the process. `AutoRemove` handles cleanup only once a container has started; the code
  removes it by hand on every path where it did not.
- **Image freshness is checked on every run**: `ensureImageLatest` compares the version the
  agent reports inside the image against `npm view <package> version`, and rebuilds with the
  agent's `BuildArg` pinned to the newer version.
- `Options.FullPrompt()` appends fixed house rules to the user's prompt (no spec skills, no
  git operations by the agent). When `--push` creates a commit, its message uses a nonempty
  `CommitMessage`; otherwise it uses `Prompt`, never `FullPrompt`.

### Adding an agent

Implement `agent.Agent` in a new file under `internal/agent`, add it to `registry` in
`agent.go` (order there is the order shown in usage text), and add the matching
`ARG <NAME>_VERSION` + global npm install to `Dockerfile`. `Container(home)` is where the
agent's host config is validated and mounted; `DefaultModel()` returning `""` means the flag is
omitted and the agent resolves the model itself (opencode, pi). Existing agents show the two
established shapes: a mounted `*_HOME` env var (codex, claude) versus a tmpfs `HOME` with the
real config mounted underneath it (opencode, pi).

## Code style

The comments in this codebase explain *why*, in full sentences, and are unusually dense at
decision points (stream draining, wait-before-start ordering, tmpfs homes). Match that register
rather than the surrounding Go norm of terse doc comments — an added function that needs a
rationale should carry one.
