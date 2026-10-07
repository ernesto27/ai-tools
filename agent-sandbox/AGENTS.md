# Agent guidance

`agent-sandbox` is a Go CLI that runs Codex, Claude Code, opencode, or pi in
Docker against a separate Git worktree.

## Start here

- Read [README.md](README.md) before changing user-visible behavior. It is the
  current CLI documentation, including configuration, authentication, and publication.
- Read [docs/architecture.md](docs/architecture.md) for package responsibilities,
  execution flow, and implementation constraints. Verify behavior against the linked code.
- This module is `agent-sandbox/` inside the `ai-tools` repository. The Git root
  is one level up (`..`); use `git rev-parse --show-toplevel` to resolve it.

## Commands

Run from this module directory:

```bash
go build ./...
go build -o agent-sandbox ./cmd/agent-sandbox   # gitignored binary
go vet ./...
go test ./...
```

Use table-driven tests. A manual sandbox run requires Docker and a real Git
repository, checks npm, and may build an image; it is not a cheap smoke test.

## Editing constraints

- Keep orchestration in `internal/sandbox` and CLI presentation in
  `cmd/agent-sandbox`. See the architecture reference for the actual dependency graph.
- Preserve both authentication modes: host configuration mounts, and JSON
  `api-key` authentication for Codex and Claude with disposable container homes.
  See `internal/agent` and `containerOptions` in `internal/sandbox/sandbox.go`.
- With `--push` or `--pr`, the agent commits inside the container; the host
  validates and publishes the result. Without either option, the prompt forbids
  staging, committing, and pushing. See `Options.FullPrompt()` in
  `internal/sandbox/options.go` and `internal/sandbox/pullrequest.go`.
- Preserve recorded worktree ownership, exit status handling, and cancellation
  cleanup. Their implementation entry points are linked in the architecture reference.
- Comments explain why in full sentences, especially around stream draining,
  wait-before-start ordering, and temporary homes. Match that style where a
  decision needs a rationale.

## Release tags

[The release workflow](../.github/workflows/agent-sandbox-release.yml) builds
the Linux amd64 archive that `install.sh` downloads.

Use `agent-sandbox-vMAJOR.MINOR.PATCH`, for example `agent-sandbox-v0.0.3`.
When using the `push-github-tag` skill, this project-specific format overrides
its generic `vMAJOR.MINOR.PATCH` format.

Fetch tags from `origin` and consider only stable tags that exactly match
`agent-sandbox-vMAJOR.MINOR.PATCH`. Compare version components numerically and
increment only the patch component. Ignore bare `v*` tags, other projects' tags,
prereleases, and build metadata. If no matching tag exists, propose
`agent-sandbox-v0.0.1`. Keep the skill's confirmation and safety checks,
including never overwriting an existing tag.
