# Spec: Default Worktree Branch Names

## Objective

`agent-sandbox` no longer requires a positional branch name for a run. An
operator may supply a branch with `-b` or `--branch`; otherwise the command
generates `<lowercase-word>-<six-digit-number>`, for example `sequoia-482917`.
The chosen name is used for the Git branch, sandbox worktree directory,
persisted worktree record, and `--push` upstream branch.

## Commands

```bash
go build ./...
go vet ./...
go test ./...
go build -o agent-sandbox ./cmd/agent-sandbox
```

Do not use a full binary run as a routine test: it requires Docker, an
authenticated host agent, and can build images.

## Project Structure

| Location | Responsibility |
| --- | --- |
| `cmd/agent-sandbox/` | CLI syntax, help, argument validation, exit status |
| `internal/sandbox/` | options, generated name, orchestration |
| `internal/git/` | Git branch and worktree operations |
| `internal/sandbox/*_test.go` | tests with temporary Git repositories |
| `docs/specs/` | reviewed feature specifications |
| `README.md` | user-facing Go CLI documentation |

The dependency direction remains `cmd -> sandbox -> {git, docker, agent}`.
Name generation belongs in `internal/sandbox`; Cobra parsing stays in
`cmd/agent-sandbox`.

## Code Style

Use Go formatting and the repository's explanatory-comment style around
safety and lifecycle decisions. Keep flag parsing at the command boundary and
construction of a complete invocation in `internal/sandbox`:

```go
// An omitted branch is generated after command-line parsing, so the
// orchestrator receives one complete invocation regardless of how it was named.
opts, err := sandbox.NewOptions(branch, agentName, model, baseImage, prompt, push)
if err != nil {
    return err
}
```

Preserve the `UsageError`/`StatusError` exit-status protocol and wrap external
command errors with operation context. Do not hide generated-name collisions
by retrying or reusing an existing Git branch.

## Testing Strategy

- Unit-test generator output for `^[a-z]+-[0-9]{6}$`, without depending on a
  particular random value.
- Test root parsing for omitted `-b`, `-b`, `--branch`, and rejection of the
  old positional branch form, including the established usage-exit protocol.
- Test generated-name collision validation without Docker: an existing branch
  or target path fails, while an explicit branch retains current reuse rules.
- Run all commands in the Commands section before merging.

## Boundaries

- Always: validate explicit and generated branch names before worktree
  creation; document the new syntax; retain explicit-name behavior.
- Ask first: adding dependencies, changing the name format, automatically
  retrying collisions, or retaining positional-branch compatibility.
- Never: modify the legacy Bash tool, change credential handling, weaken
  worktree ownership/state protections, or use Docker as a smoke test here.

## Success Criteria

1. `agent-sandbox --agent codex "fix the login redirect"` generates a name
   matching `^[a-z]+-[0-9]{6}$` and accepts the full positional text as prompt.
2. `agent-sandbox -b fix-login --agent codex "fix the login redirect"` and
   `--branch fix-login` use `fix-login` exactly.
3. A positional branch is rejected: `agent-sandbox fix-login --agent codex
   "..."` does not interpret `fix-login` as a branch.
4. The word comes from a built-in lowercase word source; no network, system
   dictionary, or new dependency is needed. The suffix has exactly six digits.
5. A generated name colliding with an existing Git branch or target worktree
   path produces a clear error, does not retry, does not reuse the branch, and
   creates no worktree or state record.
6. Explicit `-b` names retain existing Git validation and existing-branch
   behavior. Worktree-management verbs keep their own `-b` selector.
7. Help, examples, and `README.md` describe the flag-only explicit form and
   generated default. Build, vet, and tests pass.

## Open Questions

None. The format, flag-only explicit naming, and collision behavior are
confirmed.
