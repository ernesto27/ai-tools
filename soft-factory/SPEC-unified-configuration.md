# Unified Soft Factory configuration

Specification approved by the user on 2026-10-17. Implementation is blocked
pending an upstream configuration input mechanism. This document describes
the intended behavior, not functionality available in the current release.

## Objective and contract

Use project-root `software-factory.json` as the single configuration source for
Factory and every Agent Sandbox subprocess. Embed the exported
`github.com/ernesto27/ai-tools/agent-sandbox/config.Config` directly as
`Config.AgentSandbox`, with JSON key `agentSandbox`. Do not recreate the schema.

```json
{
  "documents": [],
  "stages": {
    "implementation": { "agent": "codex", "model": "gpt-6.1-sol" },
    "codeReview": { "agent": "claude", "model": "claude-opus-5-5" },
    "securityReview": { "agent": "claude", "model": "claude-opus-5-5" },
    "riskClassification": { "agent": "claude", "model": "sonnet" },
    "reviewChanges": { "agent": "codex", "model": "gpt-6-luna" }
  },
  "agentSandbox": {
    "agent": "codex",
    "model": "gpt-6-luna",
    "baseImage": "golang:1.26-alpine",
    "branch": "",
    "run": { "filePrompt": "task.txt" },
    "resume": { "push": false },
    "pr": false
  }
}
```

Agent Sandbox determines whether an empty branch is valid for a command and
supplies its own defaults. The example does not override those requirements.

## Required behavior

1. Preserve `documents`, `googleDrive`, `customSkills`, `disabledStages`,
   `stages`, independent per-stage agent/model overrides, and workflow behavior.
   Preserve upstream section types and pointer fields, including explicit
   `false`, empty strings, empty lists, and omitted fields.
2. Load unified configuration once per workflow. Pass that snapshot to branch
   selection, task reads, execution, and selection logging.
3. Cover default implementation, `continue`, standalone `review`, and recovery
   `resume`. Preserve existing run/resume section selection and explicit branch
   overrides.
4. Precedence is explicit Factory invocation arguments, then the applicable
   sandbox section, then shared sandbox fields, then Agent Sandbox defaults.
   Apply agent and model overrides independently.
5. Read the original local task from `agentSandbox.run.filePrompt`, including
   workflows executing through sandbox resume. Preserve Jira task overrides and
   supporting-document assembly.
6. Keep subprocesses in the project root. Relative task and sandbox paths must
   retain project-root context, including when configuration is transported
   through a temporary file. Preserve repository and worktree discovery.
7. Deliver the complete embedded configuration through a supported Agent Sandbox
   input mechanism to every relevant subprocess. Do not emulate its merging or
   validation by translating configuration into flags.
8. Neither Factory nor its subprocesses may read or merge project-local
   `agent-sandbox.json`. Valid, malformed, or conflicting legacy files must have
   no effect and must remain untouched. There is no fallback or automatic
   migration. A legacy file alone does not satisfy Factory's configuration input.
9. Factory may report JSON decoding, filesystem, and subprocess errors. Keep
   existing Factory-owned validation, but never call or copy sandbox semantic
   validation from `Config.Validate`. Agent Sandbox owns allowed agents/models,
   defaults, section validation, and command-specific requirements.
10. Omitted or null `agentSandbox` means empty sandbox settings. Add no required
    sandbox check and consult no legacy file. A Factory-owned task read may
    report a contextual filesystem error when no usable path is configured.
11. Propagate downstream validation/execution failures through existing error
    reporting without retrying against legacy configuration.
12. Log provenance as `software-factory.json: agentSandbox...`; retain stage
    provenance under `stages`. Never print the full configuration or secrets.
13. Scope transport to the invocation, with restrictive permissions and cleanup
    after success, failure, or cancellation. It must not become another
    user-maintained file.
14. Update README, repository guidance, CLI help, and VitePress examples when
    implementation is available. Explain the breaking change and manual
    migration: move the old JSON object into `agentSandbox`; the old file is no
    longer consumed. Do not teach this as current behavior before it exists.
15. When implementing, migrate existing ignored local configuration while
    preserving all user stage assignments. Never commit local configuration or
    delete the legacy file. No local `software-factory.json` or
    `agent-sandbox.json` was present in this checkout during interface inspection.

Do not add a Factory `--config` flag. Do not add third-party dependencies or
make unrelated workflow changes. Agent Sandbox source changes are explicitly
prohibited for this task.

## Verified integration blocker

Interface inspection on 2026-10-18 resolved the pinned dependency using
`go mod download -json github.com/ernesto27/ai-tools/agent-sandbox` from
`soft-factory/`. The exact module is
`v0.0.0-20261016150234-8042bb4d5f95`.

In that module:

- `cmd/agent-sandbox/config.go` declares
  `const localConfigFile = "agent-sandbox.json"`; `readLocalConfig` calls
  `os.ReadFile(localConfigFile)` without an alternate path or environment input.
- `configRunArgs` invokes `applyLocalConfig`, which invokes `readLocalConfig`
  before applying explicit flags. Explicit arguments therefore do not prevent
  reading, parsing, and validating a conflicting or malformed legacy file.
- `cmd/agent-sandbox/root.go` and `worktree.go` expose no alternate configuration
  input flag. Running both `go run ./cmd/agent-sandbox run --help` and
  `go run ./cmd/agent-sandbox resume --help` from the downloaded pinned module
  confirmed the available flags. JSON-only settings include section `apiKey`
  and `run.reviewers`; there is no complete CLI configuration transport.
- The exported `config` package contains schema types and model defaults, not
  an alternate input or CLI execution API. Validation and merging live in the
  CLI's `main` package.
- The local `agent-sandbox/` source has the same fixed-file input restriction.
  No installed `agent-sandbox` executable was available on `PATH`; an installed
  version cannot be assumed to offer any additional interface.

Consequently, Factory-only changes cannot satisfy requirements 7–9 with this
dependency. Changing Factory's reads alone would still let subprocesses read
the legacy file. Overwriting that file, changing the working directory,
reimplementing sandbox validation, or forwarding a subset of settings as flags
would violate the contract. No runtime implementation or dependency update is
made while the required upstream interface is unavailable.

## Proposed upstream coordination

This is a prerequisite proposal, not an existing or approved CLI interface:

1. Agent Sandbox must offer an explicit configuration-path input for run/resume
   that reads only the selected file and bypasses the project-local legacy file
   entirely. An unreadable or invalid selected input must fail without fallback.
   It must use existing upstream parsing, strict field validation, section
   merging, defaults, and command validation. Relative values must continue to
   resolve against the subprocess working directory, not the input file's
   directory. Coordinate the final flag or environment name upstream.
2. Publish an upstream revision containing the mechanism and coordinate both
   Factory's Go module pin and the minimum installed Agent Sandbox CLI version.
   Updating the module alone cannot update the external executable. Do not
   invent a revision or assume the mechanism is already installed.
3. Preserve the original embedded JSON bytes alongside the imported typed
   representation. Upstream currently rejects unknown fields, explicit null
   sections, and null scalar/list fields. Typed decoding and re-encoding loses
   unknown fields and omission/null distinctions; the exported types also lack
   `omitempty`, so marshaling them introduces null fields that upstream rejects.
   Transport the original object for upstream validation. Normalize only an
   omitted or top-level null `agentSandbox` to `{}`, as required by this spec.
   Do not invent Factory-owned unknown-field checks.

The upstream work and dependency/version coordination require separate
authorization under the specification. This task's prohibition on Agent
Sandbox changes prevents implementing that prerequisite here.

## Factory implementation plan after the prerequisite is available

1. Add the imported `Config.AgentSandbox` and retain raw embedded JSON during
   the single `config.Load`. Remove `LoadSandbox` and all legacy reads without
   adding sandbox semantic checks.
2. Pass the loaded snapshot through workflow setup and `TaskContext` to branch
   selection, original task reads, execution, and selection diagnostics. Keep
   recovery branch overrides, stage controls, Jira, and document behavior.
3. Write raw sandbox JSON to a private temporary file, close it before starting
   the child, and defer removal immediately. Supply it through the agreed
   upstream input for every invocation, retaining the project working directory.
   Keep secrets out of arguments and logs; do not log the transport contents.
4. Update configuration provenance, help, README, AGENTS.md, and site examples.
   Perform manual local migration only when local settings exist.
5. Add standard-library tests beside `internal/config`, `cmd/factory`, and
   `internal/sandbox`, using isolated temporary files and controlled executables.

## Verification and completion criteria

Test representative shared settings, run/resume sections, stage overrides,
explicit false/empty values, and independent agent/model precedence. Show that
Factory accepts semantic sandbox errors while rejecting malformed JSON, type
errors, and invalid Factory-owned settings. Cover omitted/null embedded settings,
conflicting/malformed legacy files, and legacy-only projects.

Exercise all workflow entry points with controlled subprocess fixtures. Verify
complete configuration delivery, section selection, branch selection, relative
paths, prompt assembly, Jira overrides, supporting documents, selection
provenance, failure propagation, no fallback, no secret disclosure, and temporary
file permissions/cleanup on success, failure, and cancellation. Verify strict
upstream unknown-field/null behavior survives the raw JSON handoff. No check
may require Docker, credentials, or live agents.

Run from `soft-factory/` after implementation:

```sh
go build -o /tmp/software-factory ./cmd/factory
go test ./...
go vet ./...
gofmt -w cmd internal
npm --prefix site-docs run docs:build
```

With controlled subprocess fixtures, smoke-check default, `continue`, and
`review` workflows plus `go run ./cmd/factory resume --help`.

Completion requires every supported workflow to run from the unified file,
upstream pointer semantics and validation to survive transport, legacy files to
have no observable effect, and all required checks to pass. The interface
blocker above means those completion criteria are not met by this document.

### Assessment checks on 2026-10-18

The existing Factory CLI build, `go vet ./...`, and VitePress documentation
build passed. Documentation dependencies were installed with the existing
lockfile using `npm --prefix site-docs ci --no-audit --no-fund`.
`go run ./cmd/factory resume --help` and the pinned sandbox run/resume help
commands passed without Docker or credentials.

`go test ./...` failed in existing executionlog and sandbox tests: temporary
repository initialization attempted to write the inherited repository's Git
configuration and received `Resource busy` or lock errors. These failures do
not demonstrate unified-configuration behavior. No runtime code was changed,
and no live workflow was executed.
