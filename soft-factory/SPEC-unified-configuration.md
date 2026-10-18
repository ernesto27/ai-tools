# Spec: Unified Soft Factory configuration

Status: Approved by the user on 2026-10-17WW

## Objective

Let Soft Factory CLI users configure a project using only
`software-factory.json`. Move the contents previously maintained in
`agent-sandbox.json` into a new top-level `agentSandbox` property. Reuse Agent
Sandbox's exported Go configuration struct, and leave validation of sandbox
settings to Agent Sandbox.

This is one capability: one configuration source used consistently by Factory
and its Agent Sandbox subprocesses.

## Confirmed Decisions and Assumptions

- The property name is `agentSandbox`.
- Remove support for reading the separate `agent-sandbox.json`; no fallback,
  merge, or automatic migration from that file.
- Preserve existing Factory properties, including `documents`, `customSkills`,
  `disabledStages`, and `stages`.
- Preserve per-stage agent/model overrides and existing workflow behavior.
- Factory may report JSON decoding, filesystem, and subprocess errors. It must
  not introduce semantic validation of the embedded sandbox configuration.
- Configuration is loaded from the existing project-root
  `software-factory.json`; this feature does not add a Factory `--config` flag.
- User-maintained configuration is one file. A private, temporary transport
  artifact is acceptable if the eventual integration needs one; it must not
  become a second user-maintained file.

## Configuration Contract

Example shape, combining the requested sandbox settings with stage overrides:

```json
{
  "documents": [],
  "stages": {
    "implementation": {
      "agent": "codex",
      "model": "gpt-6.1-sol"
    },
    "codeReview": {
      "agent": "claude",
      "model": "claude-opus-5-5"
    },
    "securityReview": {
      "agent": "claude",
      "model": "claude-opus-5-5"
    },
    "riskClassification": {
      "agent": "claude",
      "model": "sonnet"
    },
    "reviewChanges": {
      "agent": "codex",
      "model": "gpt-6-luna"
    }
  },
  "agentSandbox": {
    "agent": "codex",
    "model": "gpt-6-luna",
    "baseImage": "golang:1.26-alpine",
    "branch": "",
    "run": {
      "filePrompt": "task.txt"
    },
    "resume": {
      "push": false
    },
    "pr": false
  }
}
```

The empty branch above illustrates the requested configuration shape. This
spec does not promise that it is valid for every Agent Sandbox command; Agent
Sandbox determines command requirements and defaults.

## Required Behavior

1. Factory's configuration embeds
   `github.com/ernesto27/ai-tools/agent-sandbox/config.Config` under the JSON
   key `agentSandbox`. Do not recreate that schema as Factory-owned structs.
   Preserve upstream section types and pointer fields so explicit `false`
   and empty strings remain distinct from omitted section fields.
2. Load the unified configuration once for a workflow and pass its embedded
   settings to consumers. Branch selection, task-file reads, subprocess
   execution, and selection logging use the same loaded configuration.
3. All workflow entry points use the embedded settings: default implementation,
   `continue`, standalone `review`, and recovery `resume`. Preserve their
   existing run/resume section selection and explicit branch overrides.
4. Preserve configuration precedence: explicit Factory-generated invocation
   arguments, including stage agent/model overrides and existing workflow
   controls, take priority over the applicable sandbox section; that section
   takes priority over shared sandbox fields; Agent Sandbox owns its defaults.
   Preserve independent agent and model overrides.
5. Continue reading the original local task from `agentSandbox.run.filePrompt`,
   including workflows that execute through the sandbox's resume mode.
   Jira task overrides and supporting-document assembly retain existing behavior.
6. Preserve existing path semantics: Factory runs from the project root;
   relative task and sandbox paths retain that context. Passing configuration
   through temporary storage must not rebase paths to a temporary directory.
7. Every relevant Agent Sandbox subprocess receives the embedded configuration
   through a supported Agent Sandbox input mechanism. Merely changing Factory's
   reads is insufficient. Do not duplicate sandbox configuration merging and
   validation in Factory to emulate this handoff.
8. Neither Factory nor its subprocesses may read or merge a project-local
   legacy `agent-sandbox.json` during Factory workflows. An existing legacy file,
   whether valid, invalid, or conflicting, must have no effect. Do not overwrite
   or delete a user's legacy file at runtime.
9. Agent Sandbox owns sandbox field constraints, allowed agents/models, defaults,
   section validation, and command-specific requirements. Factory must not call
   a sandbox semantic validator as part of its own `Config.Validate` or copy
   those checks. JSON decoding/type errors are distinct from semantic validation.
   Existing validation of Factory-owned properties remains in place.
10. If `agentSandbox` is omitted or null, Factory must not add a sandbox-required
    check or consult the legacy file. Agent Sandbox applies its own defaults
    and command requirements to the resulting empty settings. Factory-owned
    task-file reads may still report a contextual filesystem error when no
    usable task path is available.
11. Surface Agent Sandbox execution/validation failures through existing error
    reporting. Do not silently retry against the legacy configuration.
12. Update configuration provenance in logs and diagnostics to name
    `software-factory.json: agentSandbox...`, while retaining stage provenance
    under `stages`. Do not print the complete embedded configuration or secrets.
13. Keep configuration transport scoped to the invocation. Any temporary
    artifact must use restrictive permissions and be cleaned up after success,
    failure, or cancellation. Do not change the subprocess working directory
    in a way that changes repository or worktree discovery.
14. Update README, repository guidance, CLI help, and user-facing site examples
    to teach the single-file format and explicitly describe the breaking change.
    Migration instructions are manual: move the old JSON object into
    `agentSandbox`; the old file is no longer consumed.
15. When implementing, update the local ignored `software-factory.json` to
    include the existing sandbox settings while preserving the user's stage
    assignments. Do not commit local configuration or delete the old local
    file without a separate request.

## Integration Constraint

The current Factory code calls `config.LoadSandbox()` to read the separate
file, and invokes the external `agent-sandbox` command from the project root.
The inspected local Agent Sandbox CLI reads a fixed `agent-sandbox.json` path;
its exported `config` package provides shared types and model defaults, rather
than an alternate configuration-input mechanism.

The plan must verify the exact pinned module and installed CLI interfaces and
choose a supported handoff that satisfies requirements 7–9. If Agent Sandbox
needs a new input mechanism, identify the upstream change and dependency/version
coordination for approval before implementation. Do not assume a nonexistent
`--config` flag, temporarily replace the legacy file, or bypass Agent Sandbox's
validation by translating only a subset of settings into command-line flags.

## Tech Stack

- Go 1.26.0 or later; standard-library `encoding/json` and `testing`.
- Existing Cobra CLI and subprocess execution through `os/exec`.
- Current Agent Sandbox dependency:
  `v0.0.0-20261016150234-8042bb4d5f95`.
- Existing VitePress documentation site.

## Commands

Run from the repository root:

```sh
go build -o /tmp/software-factory ./cmd/factory
go test ./...
go vet ./...
gofmt -w cmd internal
npm --prefix site-docs run docs:build
```

For implementation smoke checks with a controlled subprocess fixture:

```sh
go run ./cmd/factory
go run ./cmd/factory continue
go run ./cmd/factory review
go run ./cmd/factory resume --help
```

Use real authenticated workflow execution only when appropriate; unit and
integration verification must not require Docker, live agents, or credentials.

## Project Structure

- `internal/config/`: unified configuration decoding and Factory-owned validation;
  colocated configuration tests.
- `cmd/factory/`: workflow setup, branch selection, command help, and workflow tests.
- `internal/sandbox/`: task reads, configuration handoff, execution, selection
  logging, and controlled subprocess integration tests.
- `internal/executionlog/`: existing log/report behavior; update only as required
  to keep configuration provenance accurate.
- `README.md`, `AGENTS.md`, `site-docs/guide/`: setup, configuration, workflow,
  and migration guidance.
- `SPEC-unified-configuration.md`: this specification, tracked with the code.
- Local `software-factory.json`: ignored user configuration, never a committed
  fixture; tests create isolated temporary configuration files.

## Code Style

Use idiomatic Go, `gofmt`, lowercase package names, explicit return values, and
contextual errors wrapping underlying errors with `%w`. Avoid named returns
and bare returns in new or changed functions.

Illustrative field within the existing Factory configuration struct:

```go
import sandboxconfig "github.com/ernesto27/ai-tools/agent-sandbox/config"

type Config struct {
    Documents    []string             `json:"documents"`
    AgentSandbox sandboxconfig.Config `json:"agentSandbox"`
    // Existing Factory-owned fields remain here.
}
```

Use the imported schema directly; do not mirror its fields in a new local type.

## Testing Strategy

Use standard-library tests beside the affected packages. There is no numeric
coverage threshold. Cover the configuration and subprocess boundary with
temporary files and controlled executable fixtures.

- Decode representative shared settings, run/resume sections, stage overrides,
  and explicit pointer values such as `resume.push: false` without losing them.
- Show Factory accepts semantically invalid sandbox agent/model settings at
  config-load time while still rejecting malformed JSON/type mismatches and
  invalid Factory-owned settings.
- Exercise absent/null `agentSandbox` without legacy fallback.
- Prove conflicting and malformed legacy files are ignored, and a project
  containing only the legacy file does not satisfy the Factory config input.
- Verify subprocesses receive embedded settings across default, continue,
  review, and recovery workflows, preserving precedence and relative paths.
- Verify task prompt assembly, branch selection, and selection logs agree with
  the configuration delivered to the subprocess.
- Test explicit false/empty values against conflicting shared settings.
- Verify downstream validation failures propagate without fallback, and
  temporary transport cleanup works on failure and cancellation if applicable.
- Verify no embedded secrets appear in log output or command arguments.
- Run the complete Go tests, vet, build, and documentation build after the
  implementation and documentation changes.

## Boundaries

- **Always:** Reuse the upstream struct; keep sandbox validation upstream;
  preserve stage overrides; ignore legacy configuration; run the required
  checks; document the breaking configuration change.
- **Ask first:** Upstream Agent Sandbox changes or dependency updates needed
  for transport, unrelated workflow changes, and new third-party dependencies.
- **Never:** Add a legacy fallback, duplicate sandbox semantic validation,
  overwrite a user's legacy file, expose secrets, commit local configuration,
  or change agent/model assignments as part of the migration.

## Success Criteria

1. A project with only `software-factory.json` and its task/supporting files can
   run every supported Factory workflow using the embedded sandbox settings.
2. The embedded field's Go type is the exported Agent Sandbox `config.Config`;
   section pointer semantics survive decoding and handoff.
3. Stage overrides, section defaults, branch selection, prompt assembly, and
   log provenance consistently use the unified source.
4. A legacy `agent-sandbox.json` has no observable effect and remains untouched.
5. Factory adds no sandbox semantic validation; downstream failures remain
   attributable to Agent Sandbox.
6. The selected transport covers all fields of the imported schema, preserves
   working-directory semantics, and does not create another maintained config.
7. Tests and documentation explicitly cover the breaking change and migration.
8. Required build, test, vet, and documentation checks pass.

## Open Questions for Planning

- Which supported Agent Sandbox input mechanism will carry the embedded
  configuration? Confirm the pinned and installed interfaces; if none exists,
  propose the required upstream work before implementing Factory integration.
- Confirm whether upstream needs the original JSON bytes to retain its strict
  unknown-field validation alongside the typed Factory representation. Do not
  invent unknown-field checks in Factory or silently weaken upstream behavior.

The user approved this specification and explicitly requested no implementation.
The next phase is technical planning when requested; implementation remains
deferred until explicitly authorized.
