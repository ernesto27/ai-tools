# Unified Soft Factory configuration

Status: Specification approved by the user on 2026-10-17. Runtime implementation
is blocked on an Agent Sandbox configuration-input interface. This document
describes intended behavior, not a capability available in the current CLI.

## Objective

Configure Factory and its Agent Sandbox subprocesses using only the project-root
`software-factory.json`. Move the old sandbox JSON object into the top-level
`agentSandbox` property. Reuse the exported upstream Go schema and leave sandbox
validation, defaults, and command requirements to Agent Sandbox.

## Verified integration blocker (2026-10-18)

Factory pins
`github.com/ernesto27/ai-tools/agent-sandbox v0.0.0-20261016150234-8042bb4d5f95`.
Inspection of that exact downloaded module confirms:

- `cmd/agent-sandbox/config.go` defines
  `const localConfigFile = "agent-sandbox.json"`; `readLocalConfig` always reads
  that path with `os.ReadFile`. `configRunArgs` applies it for run/resume before
  prompt validation, including when explicit prompt flags are supplied.
- The run/resume flag registrations in `cmd/agent-sandbox/root.go` and
  `cmd/agent-sandbox/worktree.go` expose no alternate configuration path or
  configuration stream. The loader has no environment-variable override.
- The exported `config` package provides `Config`, `Section`, and
  `DefaultModel`, not a configuration-input or command-execution API. The
  configuration validation and execution packages are internal to Agent Sandbox.
- Upstream validates the raw JSON, including unknown fields and explicit nulls,
  before decoding its typed configuration. Serializing only the typed struct
  would lose unknown fields and distinctions needed by that validation.
- No installed `agent-sandbox` executable was found on `PATH` in this workspace;
  no installed-version compatibility claim can be made. The neighboring source
  checkout also uses the fixed local path, but is not evidence of an installed
  executable or a replacement for checking the pinned module.
- Building the exact pinned module into a temporary binary and inspecting
  run/resume help confirms the available flags. Controlled subprocess probes
  confirm both commands reject `--config` as unknown and both reject malformed
  legacy JSON even with explicit agent, prompt, and branch flags. These probes
  fail before container or authenticated execution.

Consequently, changing only Factory's reads cannot satisfy this specification.
Even supplying all public execution flags would still make the subprocess read
and validate a conflicting or malformed legacy file. Flags also do not provide
a complete raw configuration handoff. Changing directories would alter relative
paths and repository discovery; replacing the legacy file is prohibited.

The user explicitly prohibits changes to Agent Sandbox. No runtime change or
dependency update should be presented as completing this capability while this
blocker remains. No local user configuration files were present in this
workspace to migrate.

## Required upstream interface and version coordination

The proposed prerequisite is an upstream explicit configuration-path input for
run/resume, such as `--config <path>` (including their TUI equivalents). This is
a proposal, not an existing flag. When provided, it must replace the fixed-path
input completely, never read or merge the project-local legacy file, and fail
on an unreadable explicit path. Relative sandbox paths must continue to resolve
from the invocation's working directory, not from the configuration file's
directory. The same upstream raw JSON validation, merging, defaults, and
command validation must apply to the selected input.

Before Factory integration:

1. Obtain authorization for the upstream work and dependency coordination;
   do not modify Agent Sandbox as part of this Factory-only stage.
2. Implement and release the input upstream, with tests for path replacement,
   legacy-file isolation, raw validation, precedence, and working-directory
   semantics. Select the resulting actual module version; no version is assumed.
3. Coordinate Factory's pinned schema version with the minimum installed CLI
   version supporting that interface. The Go module dependency does not update
   the separately installed executable. Document the installation requirement
   and ensure unsupported executables surface a clear error without fallback.
4. Verify that actual released interface before implementing the Factory handoff.

With that interface, Factory can keep the original embedded JSON bytes alongside
the required typed field and write those bytes to an invocation-scoped private
temporary file. Omitted or null `agentSandbox` becomes `{}` for transport. An
explicit object must retain its raw fields, including unknown fields and nulls,
for upstream validation. Factory must not add unknown-field or sandbox semantic
checks of its own. Marshaling the typed value alone is insufficient: upstream
fields lack `omitempty`, and marshaling would introduce null section fields as
well as discard unrecognized input.

## Configuration contract

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

The empty branch illustrates the shape; Agent Sandbox determines whether it is
valid for a command. Preserve all Factory-owned properties, including
`documents`, `googleDrive`, `customSkills`, `disabledStages`, and `stages`.
Preserve existing stage assignments, independent agent/model overrides, and
workflow behavior.

The field must directly use the upstream schema:

```go
import sandboxconfig "github.com/ernesto27/ai-tools/agent-sandbox/config"

// Within the existing Factory Config:
// AgentSandbox sandboxconfig.Config `json:"agentSandbox"`
```

Do not mirror upstream fields in Factory-owned structs. Retain upstream section
types and pointers, so explicit false and empty strings differ from omission.

## Required runtime behavior

1. Load the existing project-root `software-factory.json` once per workflow.
   Add no Factory `--config` flag. Pass that loaded configuration through branch
   selection, task reads, execution, and selection logging.
2. Use embedded settings for default implementation, `continue`, standalone
   `review`, and recovery `resume`. Preserve their existing run/resume section
   selection and explicit branch overrides.
3. Precedence remains explicit Factory-generated arguments (including workflow
   controls and stage agent/model overrides), then the applicable sandbox
   section, then shared sandbox settings, then upstream defaults. Agent and
   model overrides remain independent.
4. Read the original local task from `agentSandbox.run.filePrompt` even when
   executing through resume mode. Keep Jira overrides and supporting-document
   assembly unchanged.
5. Keep Factory and subprocesses in the project root. Preserve relative task,
   attachment, and sandbox paths and repository/worktree discovery when using
   temporary configuration transport.
6. Every relevant sandbox subprocess must receive the full embedded raw
   configuration through the verified upstream input mechanism. Do not emulate
   upstream configuration merging or validation in Factory.
7. Neither Factory nor its subprocesses may read or merge project-local
   `agent-sandbox.json`. Conflicting, malformed, and valid legacy files must all
   have no effect. Never overwrite or delete that file at runtime.
8. Factory may report decoding/type errors, filesystem errors, and subprocess
   errors. Its `Config.Validate` must retain Factory-owned validation without
   sandbox semantic checks or calls to upstream semantic validators. Agent
   Sandbox owns constraints, allowed agents/models, defaults, section
   validation, and command-specific requirements.
9. Omitted or null `agentSandbox` supplies empty settings without a required-field
   check or legacy fallback. A Factory-owned task read may still return a
   contextual filesystem error if no usable task path exists.
10. Propagate Agent Sandbox failures using existing error reporting. Never retry
    using legacy configuration.
11. Log sandbox provenance as `software-factory.json: agentSandbox...`; retain
    stage provenance under `software-factory.json: stages...`. Never log the
    complete sandbox object or secrets, including in command arguments.
12. Scope temporary transport to the invocation; use restrictive permissions
    and clean up on success, failure, and cancellation. It must not become a
    second user-maintained configuration file.

## Factory implementation sequence after the prerequisite

1. In `internal/config`, add the imported typed field and retain raw embedded
   bytes during the same file load. Remove `LoadSandbox` and keep existing
   Factory validation.
2. In `cmd/factory`, pass the loaded settings to branch selection and every
   workflow. Update command help and preserve recovery behavior.
3. In `internal/sandbox`, pass the settings through task reads, selection
   diagnostics, and all subprocess execution paths. Centralize private transport
   creation and cleanup at the subprocess boundary without changing its working
   directory. Keep existing explicit execution flags and prompt handling.
4. Update README, AGENTS.md, and VitePress configuration/workflow examples to
   explain the breaking change, manual migration, and minimum Agent Sandbox CLI
   version. Present single-file behavior as available only when implemented.
5. If local ignored configurations exist, manually embed the existing sandbox
   object in `software-factory.json` while preserving the user's stage
   assignments. Never commit local configuration or delete the old local file
   without a separate request.

## Breaking change and manual migration

Once implemented, users must move the entire old sandbox JSON object into
`agentSandbox` in `software-factory.json`, keeping other Factory properties.
Factory will no longer consume `agent-sandbox.json`; there will be no automatic
migration, merging, or fallback. Retaining the old file will have no effect on
Factory workflows. This migration must wait for the supported CLI handoff.

## Verification and acceptance

Use Go 1.26.0 or later, standard-library `encoding/json` and `testing`, Cobra,
and `os/exec`. Avoid new third-party dependencies. Use idiomatic Go, `gofmt`,
explicit returns, and contextual errors wrapping underlying errors with `%w`.

Tests beside affected packages must use isolated temporary configurations and
controlled executable fixtures; Docker, live agents, and credentials are not
required. Cover:

- Shared settings, run/resume sections, stage overrides, and explicit pointer
  values, including false/empty section values conflicting with shared values.
- Semantically invalid sandbox agent/model values accepted by Factory loading;
  malformed JSON/type mismatches and invalid Factory settings still rejected.
- Absent/null embedding; conflicting/malformed legacy files ignored and left
  intact; a legacy-only project does not satisfy the Factory configuration input.
- Full subprocess handoff for default, continue, review, and recovery workflows;
  independent stage overrides, explicit workflow controls, and relative paths.
- Agreement between task prompt assembly, branch selection, selection logs,
  and delivered settings; Jira and document behavior retained.
- Upstream unknown-field/null and semantic failures propagated without fallback;
  no secrets in logs or arguments; private transport permissions and cleanup
  after success, failure, and cancellation.

After implementation, run from `soft-factory/`:

```sh
gofmt -w cmd internal
go build -o /tmp/software-factory ./cmd/factory
go test ./...
go vet ./...
npm --prefix site-docs run docs:build
```

Use controlled subprocess fixtures for implementation, continue, and review
smoke checks; inspect `go run ./cmd/factory resume --help`. Real authenticated
workflow execution is not necessary for verification.

Success requires every supported workflow to run from the unified file, direct
reuse of upstream types, preserved pointer semantics and precedence, upstream
validation of the full configuration, no observable legacy-file effect,
accurate provenance, complete migration documentation, and all required checks
passing. The current integration blocker must be resolved before these criteria
can be claimed.

## Verification performed during the blocked stage

The unchanged Factory baseline passed Go build, vet, tests, and the VitePress
documentation build. `gofmt -l cmd internal` reported no formatting changes.
Factory recovery command help also ran successfully. The first test run failed
because inherited Git directory variables redirected temporary test repositories
to the host repository; rerunning with `GIT_DIR`, `GIT_WORK_TREE`,
`GIT_INDEX_FILE`, and `GIT_COMMON_DIR` removed from the test process environment
passed. No Git configuration was changed. These checks verify the existing
baseline and the blocker evidence, not the proposed unified runtime behavior.
