# API Key and Reviewer Flags

Status: Approved by the user, implement

## Objective

Allow individual CLI invocations to supply API keys and PR reviewers without
editing `agent-sandbox.json`. Use the shared execution bindings and existing
configuration precedence on `run`, `resume`, `run-tui`, and `resume-tui`.
This is a configuration interface change; existing authentication and GitHub
publication adapters retain their behavior. No new dependencies are required.

## API key

- Register `--apiKey <key>` without a shorthand on all four commands.
- Explicit flags override the selected section's `apiKey`, including
  `--apiKey=""`, which selects host authentication and its required mounts.
  Omitted flags use section values and existing defaults.
- Nonempty keys remain supported only for Codex and Claude Code. Other agents
  produce the existing usage error. `--api-key` remains unsupported.
- Preserve disposable container homes and environment injection. Do not add
  keys to prompts, worktree records, help, or transcript output.

## Reviewers

- Register repeatable `--reviewers <username>` without a shorthand on all four
  commands, using one flag per username rather than comma-separated lists.
- On run commands, explicit CLI values replace `run.reviewers` entirely;
  omitted flags use the JSON list. `--reviewers=""` alone clears it.
- Reject empty or whitespace-only entries in other lists, including a clear
  value combined with named reviewers. Trim surrounding whitespace and accept
  letters, digits, and hyphens with an initial letter or digit. Reject mentions,
  email addresses, teams, comma-separated lists, and option-like strings.
- Deduplicate case-insensitively, retaining first occurrence order and spelling
  after trimming, as the JSON normalizer does.
- Invalid CLI values produce a usage error before sandbox execution. Resume
  commands apply the same syntax validation, then ignore valid reviewer values.
- Request reviews only when a run creates a new PR with `--pr`. Preserve reused
  PRs, push-only behavior, runs without `--pr`, no reviewable changes, failures,
  cancellation, and create recovery. Do not change existing-PR reviewers.
- Resume with `--pr` still requires an existing open PR, preserves its metadata,
  and never adds, removes, or re-requests reviewers.
- Keep the JSON schema unchanged: reviewers are accepted only in `run`;
  `resume.reviewers` and root-level `reviewers` remain invalid. Neither flag
  introduces new root defaults or effective reviewer configuration on resume.

## Help, documentation, and errors

Help identifies both flags, their value formats, and ignored resume reviewers.
README, architecture, and website reference, configuration, authentication, and
publication documentation describe the same behavior and remove JSON-only
claims. Preserve usage-error routing and exit status 2. Validate the entire
JSON file even when CLI flags override values.

## Verification

Use table-driven Go tests at the command parsing and configuration boundary,
without Docker, live credentials, or real GitHub calls. Cover registration and
help on all commands; absent JSON, section fallback, CLI overrides, empty keys,
and section isolation; reviewer replacement, clearing, normalization, invalid
values, and ignored resume values; full JSON validation and unsupported agents.
Retain authentication and publication regression coverage.

Run from the `agent-sandbox` module directory:

```bash
go build ./...
go vet ./...
go test ./...
npm --prefix site-docs run docs:build
```

Usage examples are not verification commands; actual sandbox execution requires
Docker, credentials, a repository, and potentially image builds and npm access:

```bash
agent-sandbox run -a codex --apiKey "$OPENAI_API_KEY" -q "inspect this repository"
agent-sandbox run -a claude --pr --reviewers alice --reviewers bob -q "fix the login redirect"
agent-sandbox resume -b fix-login -a codex --apiKey "$OPENAI_API_KEY" --pr -q "add a regression test"
agent-sandbox run -a codex --apiKey="" --reviewers="" -q "inspect this repository"
```

## Boundaries

Preserve authentication modes, precedence, ownership, exit statuses, cancellation
cleanup, and publication rules. Do not expand authentication providers, existing
PR reviewer mutations, dependencies, CI, aliases, shorthands, comma-list syntax,
JSON root defaults, JSON folder support, or unrelated configuration settings.
Do not store real secrets or log supplied key values.
