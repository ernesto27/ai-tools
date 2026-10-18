# Spec: Unified Soft Factory configuration

Status: Approved; amended at the user's request to generate the sandbox JSON
on demand. This update changes requirements only, not application code.

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
- `software-factory.json` is the only user-maintained configuration source.
  Factory does not use an existing `agent-sandbox.json` as input: no fallback,
  merge, or automatic migration from it.
- Preserve existing Factory properties, including `documents`, `customSkills`,
  `disabledStages`, and `stages`.
- Preserve per-stage agent/model overrides and existing workflow behavior.
- Factory may report JSON decoding, filesystem, and subprocess errors. It must
  not introduce semantic validation of the embedded sandbox configuration.
- Configuration is loaded from the existing project-root
  `software-factory.json`; this feature does not add a Factory `--config` flag.
- Factory generates `agent-sandbox.json` in the subprocess working directory
  on demand from `agentSandbox`, keeps it available while sandbox subprocesses
  run, and cleans it up afterward. An existing local file is preserved and
  restored as described below, without using its contents as configuration.
- Do not change any code or files under `agent-sandbox/`. No new upstream
  configuration interface or dependency update is needed for this transport.

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
7. Before launching sandbox subprocesses, Factory materializes the embedded
   configuration as `./agent-sandbox.json` in their existing project working
   directory. The unmodified Agent Sandbox binary reads that generated file
   through its existing configuration loader. Keep existing per-stage flags
   and workflow controls. There is no configuration-path flag to pass, and
   `--filePrompt` selects a task prompt, not a configuration file.
8. An existing user-maintained `agent-sandbox.json`, whether valid, malformed,
   or conflicting, must not supply configuration. Preserve it through an
   exclusive backup-and-restore lifecycle; the child sees only the generated
   file. Restore the original file after execution without changing its
   contents or permissions. Never permanently overwrite or delete it.
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
13. Scope generated configuration to the workflow. Use restrictive permissions
    and clean up after success, failure, or cancellation, once all child
    processes have stopped. Preserve the subprocess working directory and
    repository/worktree discovery. Serialize Factory workflows that share the
    generated path so one run cannot replace another run's configuration.
14. Update README, repository guidance, CLI help, and user-facing site examples
    to teach the single-file format and explicitly describe the breaking change.
    Migration instructions are manual: move the old JSON object into
    `agentSandbox`; the old file is no longer a configuration source. Explain
    that `agent-sandbox.json` exists temporarily as a generated runtime file.
15. When implementing, update the local ignored `software-factory.json` to
    include the existing sandbox settings while preserving the user's stage
    assignments. Do not commit local configuration or delete the old local
    file permanently. Runtime backup/restore is explicitly allowed by this spec.

## Generated Configuration Lifecycle

The current Factory code calls `config.LoadSandbox()` to read the separate
file, and invokes the external `agent-sandbox` command from the project root.
Replace Factory's legacy reads with the already-loaded `Config.AgentSandbox`.
The existing Agent Sandbox CLI reads `./agent-sandbox.json`; Factory uses that
fixed filename as its transport. This resolves the previous integration
blocker entirely within Soft Factory.

1. Load `software-factory.json` once. Decode `agentSandbox` into the imported
   upstream struct for Factory consumers, and retain the original embedded
   JSON for transport. Do not marshal the typed struct to construct the
   generated file: its fields lack `omitempty`, and round-tripping would add
   null fields, drop unknown fields, and change upstream validation behavior.
   Preserve the original object's field values and omission/null distinctions;
   only omitted or top-level null `agentSandbox` becomes `{}`.
2. Before changing the generated path, acquire an exclusive per-project
   workflow lock. A competing Factory workflow must fail clearly or wait with
   cancellation support; it must never share or replace active configuration.
3. If a regular `agent-sandbox.json` already exists, move it to a unique private
   backup without parsing or merging it. Refuse symlinks and non-regular paths
   without modifying them. Preserve the original contents and permissions.
4. Create the generated file atomically with mode `0600`, from the retained
   embedded JSON, and close it before any sandbox subprocess starts. Register
   restoration/cleanup before starting mutation, including setup-failure paths.
5. Execute the workflow's sandbox subprocesses from the original working
   directory, using existing flags for stage overrides, generated prompts,
   branch selection, and workflow controls. The generated file supplies all
   other supported settings, including JSON-only fields. Factory-owned reads
   must use the loaded snapshot, not reread the generated file.
6. After all child processes stop, remove the generated file and restore the
   original backup, if any, before releasing the lock. If no original existed,
   leave no `agent-sandbox.json`. Report cleanup/restoration failures without
   hiding the original execution failure. Never delete the backup if restoring
   it fails, or clobber a file unexpectedly replaced by another process.
7. Normal cancellation must stop and wait for children before restoration.
   Forced termination or machine failure can bypass cleanup: retain enough
   ownership/recovery information to detect an interrupted lifecycle on the
   next run, stop with clear recovery instructions, and preserve the backup.
   Do not silently treat a stale generated file as user configuration or
   destroy recovery artifacts.
8. Keep generated configuration, backups, locks, and recovery artifacts out of
   commits, reports, and task-change evidence. Never log their configuration
   contents. Do not run a separate manual `agent-sandbox` workflow in the same
   directory while Factory owns the generated configuration; the unmodified
   external CLI does not participate in Factory's lock.

Do not introduce a nonexistent `--config` flag, change the subprocess working
directory, emulate upstream validation, or reduce the imported schema to only
the subset supported by flags. No changes under `agent-sandbox/` are allowed.

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
  Verify existing files are restored byte-for-byte with original permissions.
- Verify subprocesses receive embedded settings across default, continue,
  review, and recovery workflows, preserving precedence and relative paths.
- Verify task prompt assembly, branch selection, and selection logs agree with
  the configuration delivered to the subprocess.
- Test explicit false/empty values against conflicting shared settings.
- Verify the generated JSON preserves unknown fields and null/omission
  distinctions for upstream validation, including JSON-only settings.
- Verify children read the generated file from the correct working directory,
  and no configuration-path flag is invented or required.
- Verify downstream validation failures propagate without fallback, and
  cleanup/restoration works after success, setup failure, child failure, and
  cancellation. Verify file mode `0600`, symlink rejection, concurrent-run
  exclusion, and interrupted-lifecycle detection without losing backups.
- Verify no embedded secrets appear in log output or command arguments.
- Run the complete Go tests, vet, build, and documentation build after the
  implementation and documentation changes.

## Boundaries

- **Always:** Reuse the upstream struct; keep sandbox validation upstream;
  preserve stage overrides; generate the runtime JSON; preserve and restore
  pre-existing files; run the required checks; document the breaking change.
- **Ask first:** Unrelated workflow changes and new third-party dependencies.
- **Never:** Add a legacy fallback, duplicate sandbox semantic validation,
  permanently overwrite a user's legacy file, expose secrets, commit local
  configuration, change agent/model assignments as part of the migration,
  or modify any code or files under `agent-sandbox/`.

## Success Criteria

1. A project with only `software-factory.json` and its task/supporting files can
   run every supported Factory workflow using the embedded sandbox settings.
2. The embedded field's Go type is the exported Agent Sandbox `config.Config`;
   section pointer semantics survive decoding and handoff.
3. Stage overrides, section defaults, branch selection, prompt assembly, and
   log provenance consistently use the unified source.
4. A pre-existing `agent-sandbox.json` supplies no settings and is restored
   unchanged after execution; if none existed, no generated file remains.
5. Factory adds no sandbox semantic validation; downstream failures remain
   attributable to Agent Sandbox.
6. The selected transport covers all fields of the imported schema, preserves
   working-directory semantics, and does not create another maintained config.
7. Tests and documentation explicitly cover the breaking change and migration.
8. Required build, test, vet, and documentation checks pass.
9. The implementation requires no changes under `agent-sandbox/`, no new
   upstream input interface, and no dependency update for configuration transport.

## Scope and Implementation Authorization

The configuration handoff is decided: generate `agent-sandbox.json` on demand
and use the existing binary unchanged. The earlier requirement to obtain a new
upstream input mechanism is superseded. This request updates the specification
only; no application implementation is performed as part of this update.

When this spec is supplied as an implementation task, implement the feature
within Soft Factory using the lifecycle above. Do not substitute a blocker-only
document for the feature on the basis of the CLI's fixed configuration filename;
that existing filename is the agreed transport. Do not modify `agent-sandbox/`.
