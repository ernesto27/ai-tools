# Unified Soft Factory configuration

Status: Approved by the user on 2026-10-17WW.

## Objective

Let Soft Factory CLI users configure a project using only
`software-factory.json`. Move the object previously maintained in
`agent-sandbox.json` into the top-level `agentSandbox` property. Reuse
`github.com/ernesto27/ai-tools/agent-sandbox/config.Config` directly and leave
sandbox validation to Agent Sandbox. This is one configuration source used
consistently by Factory and its Agent Sandbox subprocesses.

## Configuration contract

```json
{
  "documents": [],
  "stages": {
    "implementation": {"agent": "codex", "model": "gpt-6.1-sol"},
    "codeReview": {"agent": "claude", "model": "claude-opus-5-5"},
    "securityReview": {"agent": "claude", "model": "claude-opus-5-5"},
    "riskClassification": {"agent": "claude", "model": "sonnet"},
    "reviewChanges": {"agent": "codex", "model": "gpt-6-luna"}
  },
  "agentSandbox": {
    "agent": "codex",
    "model": "gpt-6-luna",
    "baseImage": "golang:1.26-alpine",
    "branch": "",
    "run": {"filePrompt": "task.txt"},
    "resume": {"push": false},
    "pr": false
  }
}
```

The empty branch illustrates the shape, not a promise of validity for every
Agent Sandbox command. Agent Sandbox owns command requirements and defaults.

## Required behavior

1. Embed the exported upstream `config.Config` under `agentSandbox`; do not
   recreate its schema. Preserve upstream section types and pointer semantics
   so explicit false and empty values differ from omitted section fields.
2. Load project-root `software-factory.json` once per workflow. Branch selection,
   task reads, subprocess execution, and selection logging share that snapshot.
   This feature adds no Factory `--config` flag.
3. Use embedded settings for default implementation, `continue`, standalone
   `review`, and recovery `resume`. Preserve existing run/resume section selection,
   explicit branch overrides, stage agent/model overrides, and workflow behavior.
4. Explicit Factory invocation arguments (including independent agent/model
   overrides and existing workflow controls) win over the applicable sandbox
   section, which wins over shared sandbox fields. Agent Sandbox owns defaults.
5. Read the original local task from `agentSandbox.run.filePrompt`, including
   workflows executing in resume mode. Preserve Jira overrides and supporting
   document assembly.
6. Keep the project-root working directory. Relative task and sandbox paths
   retain that context even if configuration is transported through temporary
   storage. Do not change repository or worktree discovery.
7. Every relevant Agent Sandbox subprocess must receive the complete embedded
   configuration through a supported upstream input mechanism. Do not emulate
   handoff by translating a subset into flags or duplicating merging/validation.
8. Neither Factory nor its subprocesses may read or merge a project-local
   `agent-sandbox.json` during Factory workflows. Valid, invalid, and conflicting
   legacy files have no effect. Never overwrite or delete them at runtime.
9. Factory may report filesystem, JSON decoding/type, and subprocess errors.
   It must not call a sandbox semantic validator from `Config.Validate` or copy
   its checks. Agent Sandbox owns constraints, allowed agents/models, defaults,
   section validation, and command requirements. Preserve Factory-owned checks.
10. Omitted or null `agentSandbox` supplies empty settings, without a required
    check or legacy fallback. Agent Sandbox applies defaults and requirements;
    Factory may still report a contextual task-file filesystem error.
11. Surface downstream execution/validation failures through existing error
    reporting; never retry against legacy configuration.
12. Name configuration provenance as `software-factory.json: agentSandbox...`,
    retaining `stages` provenance. Do not dump configuration or secrets in logs
    or command arguments.
13. Scope transport to each invocation, use restrictive permissions, and clean
    up after success, failure, or cancellation. Do not create another maintained
    config file or temporarily replace the legacy file.
14. Update README, repository guidance, CLI help, and site examples for this
    breaking change. Manual migration moves the old entire JSON object into
    `agentSandbox`, preserving existing Factory properties and stage assignments.
15. When existing local ignored configuration is available, include its sandbox
    settings in local `software-factory.json` without changing stage assignments.
    Do not commit local config or remove the old file without a separate request.

## Integration decision and release coordination

The pinned Go module is
`v0.0.0-20261016150234-8042bb4d5f95`. Its CLI and the inspected repository CLI
originally read a fixed `agent-sandbox.json` and exposed no alternate input.
No installed `agent-sandbox` binary was available in this workspace. The user
subsequently delegated all implementation decisions without further questions,
including the required upstream input change in this shared repository.

Add a supported upstream `--config <path>` option to run/resume and their TUI
variants. An explicit file is required to exist, completely replaces local
configuration input, and uses the same upstream validation, merging, and defaults.
Relative paths remain relative to cwd. Direct Agent Sandbox use without the new
flag retains its existing local-file behavior.

Factory retains the original embedded JSON as well as its typed representation.
This preserves unknown fields and explicit nulls for upstream strict validation;
marshalling only the typed representation would silently discard unknown fields.
Omitted/null outer `agentSandbox` becomes `{}`. Each invocation writes that
object to a temporary mode-0600 file, passes `--config`, and removes the artifact
when the subprocess returns, including failure/cancellation.

The upstream exported Go schema does not change, so Factory keeps its exact
pinned Go dependency; no third-party dependencies are added. Release/install the
updated Agent Sandbox CLI before or alongside Factory. Old binaries fail on the
unsupported flag without legacy fallback. Source checkouts can build both CLIs
from their respective module directories. Do not invent a published module
version for this runtime CLI change.

## Verification

Use Go 1.26.0 or later, standard-library JSON/testing, Cobra, and `os/exec`.
Tests create isolated temporary configuration files and controlled executables;
no Docker, live agent, or credentials are required. Cover:

- Shared settings, run/resume sections, stage overrides, explicit false/empty
  pointer values, and the full raw object across decoding and subprocess handoff.
- Factory acceptance of semantically invalid sandbox settings while rejecting
  malformed JSON/types and invalid Factory-owned settings.
- Absent/null sandbox configuration, ignored valid/conflicting/malformed legacy
  files, and projects containing only the legacy file.
- Every workflow entry, branch and prompt consistency, relative paths, independent
  agent/model precedence, and selection provenance without secret disclosure.
- Downstream failures without fallback and transport cleanup after success,
  failure, cancellation, and launch failure.
- Upstream explicit-file validation, strict unknown/null handling, existing
  CLI/section/shared precedence, and preservation of cwd.

Run from `soft-factory/`:

```sh
gofmt -w cmd internal
go build -o /tmp/software-factory ./cmd/factory
go test ./...
go vet ./...
npm --prefix site-docs run docs:build
```

Also build, test, and vet the changed Agent Sandbox module. Use controlled
subprocess fixtures for default, continue, review, and recovery smoke tests;
`go run ./cmd/factory resume --help` requires no authenticated execution.

## Boundaries and success

Always reuse the upstream struct, keep sandbox validation upstream, preserve
stage overrides, ignore legacy config, run required checks, and document manual
migration. Upstream input/dependency changes, unrelated workflow changes, and new
third-party dependencies ordinarily require approval; this execution has the
user's explicit instruction to decide without questions. Never introduce legacy
fallback, copy sandbox semantic validation, overwrite a user's legacy file,
expose secrets, commit local config, or change model assignments during migration.

Success means every supported workflow can use only `software-factory.json`
and its referenced task/supporting files, all consumers agree on configuration,
legacy files remain untouched, upstream owns sandbox validation, transport is
complete and private, and tests, vet, builds, and documentation builds pass.
