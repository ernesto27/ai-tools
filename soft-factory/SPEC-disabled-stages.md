# Spec: Configurable Disabled Pipeline Stages

Status: Approved — the user delegated all decisions; the proposed decisions
below are adopted as written. Ready for technical planning. No implementation
has been written yet.

## Objective

Let Soft Factory users list stages in `software-factory.json` that must not
execute. Users can disable any existing stage without editing code or changing
stage order. Both the default workflow and `review` honor this configuration.
Invalid configuration must produce a clear error rather than silently running a
stage the user intended to disable.

This is one capability: configuration-driven stage suppression. Configuration
validation, workflow gating, and report handling are parts of that capability;
a capability map is unnecessary.

## Requirements Established in Discussion

- Use the camelCase JSON property `disabledStages` and camelCase stage values
  such as `securityReview` and `riskClassification`.
- Apply it to both the default workflow and the `review` command.
- Allow any of the five existing stages to be disabled.
- Validate configuration and show an error for unknown stage names.

## Adopted Decisions

1. Public stage identifiers are exact, case-sensitive camelCase strings.
   Map these configuration values to existing internal pipeline identifiers;
   internal log labels, report names, and skill paths need not be renamed.
   Kebab-case stage values are invalid, not aliases.
2. Omitting `disabledStages` or supplying `[]` preserves current behavior.
   An explicitly supplied value must be an array of strings: `null`, scalar
   values, objects, and non-string entries (including `null`) are errors.
3. Duplicate stage names are accepted and have no additional effect. Blank,
   whitespace-padded, differently cased, or unknown names are rejected rather
   than normalized.
4. Disabling a stage does not disable downstream stages. Risk classification
   receives only reports actually produced by enabled reviews in this run.
5. A skipped stage prints a clear message; it does not execute its callback,
   load its custom skill file, or generate stage-specific prompts or reports.
6. The existing `continue` command shares the workflow and therefore honors
   the same list. No new commands or CLI flags are introduced.
7. Branch selection and shared preparation remain unchanged. Disabling
   implementation does not turn the default command into `review`. Remaining
   stages that resume work require an existing target worktree.

## Configuration Contract

Example configuration:

```json
{
  "documents": ["task.txt"],
  "disabledStages": ["securityReview", "riskClassification"]
}
```

Accepted identifiers and their existing command applicability:

| Stage identifier | Default workflow | `review` | `continue` |
|---|---|---|---|
| `implementation` | Yes | No | Yes, using existing continuation behavior |
| `codeReview` | Yes | Yes | Yes |
| `securityReview` | Yes | Yes | Yes |
| `riskClassification` | Yes | Yes | Yes |
| `reviewChanges` | Yes | No | Yes |

A valid identifier is accepted even if the selected command never runs that
stage. Listing `implementation` under `review`, for example, has no effect and
is not an error. The list is a set of exclusions, not an execution-order list.

Validate the entire list before invoking any agent or loading external task
context. Errors should identify `disabledStages`, the offending array index or
value where applicable, and the allowed identifiers for unknown names.

Illustrative diagnostic:

```text
Error: validate factory configuration: disabledStages[0]: unknown stage "securityReveiw"; allowed: implementation, codeReview, securityReview, riskClassification, reviewChanges
```

Malformed JSON and other existing configuration errors continue to be rejected.
Do not introduce unrelated global unknown-property validation or silently
accept `disabled-stages` as an alias.

## Required Workflow Behavior

- Run only stages normally included by the selected command and not listed in
  `disabledStages`. Preserve the relative order of enabled stages.
- Preserve existing failure, interruption, and best-effort walkthrough behavior
  for enabled stages.
- Treat a disabled stage as intentionally skipped, not failed or successful.
  Print `Skipping disabled stage: <camelCase identifier>` when reaching such a stage.
  Record that message in the existing execution log when logging is active;
  do not create execution records or result artifacts claiming it ran.
- Do not resolve custom skill files for disabled stages. Existing JSON validation
  still applies to configured custom skill names and other configuration fields.
  A syntactically valid skill name whose file is missing must not block execution
  when its stage is disabled.
- Risk classification must not receive empty report paths or old reports from
  disabled reviews. With one enabled review, use only its report. With both
  reviews disabled, inspect changes without review reports and explicitly
  describe the missing review evidence in the risk prompt.
- Disabling risk classification must not prevent enabled reviews or the enabled
  final walkthrough from executing.
- Disabling implementation must not suppress the final walkthrough on the
  default workflow or `continue`.
- If every applicable stage is disabled, invoke no agent subprocess and return
  success after normal shared preparation succeeds. Existing environment,
  sandbox branch configuration, Jira, and document-loading checks are not
  bypassed by this feature.
- Do not remove or overwrite pre-existing reports merely because their stages
  are disabled. Existing report cleanup behavior otherwise remains unchanged.
- Keep logging eligibility tied to the command, not to whether implementation
  is enabled. This feature does not add execution logging to standalone `review`.

## Tech Stack

- Go 1.25.3 or later, matching `go.mod`.
- Existing Cobra v1.10.2 command routing.
- Standard-library JSON parsing, validation, and testing.
- Existing `agent-sandbox` subprocess integration; no new dependencies.

## Commands

Run from the repository root:

```bash
go build -o /tmp/software-factory ./cmd/factory
go test ./...
go vet ./...
gofmt -w cmd internal
```

Manual verification, only with controlled local configuration and task content:

```bash
go run ./cmd/factory
go run ./cmd/factory review
go run ./cmd/factory continue
```

These workflow commands require `agent-sandbox` and local configuration. Automated
tests must not invoke live agents, Jira, or Google Drive.

## Project Structure

```text
SPEC-disabled-stages.md        Version-controlled feature specification
internal/config/config.go     Configuration parsing and validation
internal/config/*_test.go     Configuration contract tests
cmd/factory/main.go            Workflow gating and custom-skill loading
cmd/factory/*_test.go          Workflow and command behavior tests
internal/sandbox/runner.go     Risk prompt/report integration where needed
internal/sandbox/*_test.go     Prompt and controlled subprocess tests
internal/executionlog/         Existing log integration, only if needed
README.md                     Configuration reference and usage examples
software-factory.json         Ignored local configuration; do not commit
```

## Code Style

Follow idiomatic Go: lowercase packages, PascalCase exported names, camelCase
unexported names, tab indentation via `gofmt`, and contextual errors with `%w`.
Keep workflow orchestration under `cmd/factory` and reusable configuration logic
under `internal/config`. Use camelCase for both the JSON property and public
stage values; explicitly map values to existing internal stage identifiers.

Existing error-wrapping style to retain:

```go
if err := cfg.Validate(); err != nil {
	return Config{}, fmt.Errorf("validate factory configuration: %w", err)
}
```

## Testing Strategy

Use standard-library `testing`, with tests beside source as `*_test.go` and
named `TestXxx`. Prefer table-driven cases and temporary directories. There is
no numeric coverage target; every acceptance criterion below needs automated
coverage where applicable.

- Configuration tests: omitted field, empty array, each valid identifier, all
  identifiers, duplicates, malformed JSON, invalid field types, non-string and
  null entries, blank/padded names, case mismatches, kebab-case values, and
  unknown names.
- Workflow tests: each applicable stage disabled individually, multiple stages
  disabled, all disabled, unchanged default order, and both primary commands.
  Cover `continue` compatibility and unchanged branch selection.
- Controlled subprocess fixtures: prove disabled stages never invoke
  `agent-sandbox`; enabled stages still run and retain failure semantics.
- Custom-skill tests: missing files are ignored for disabled stages but remain
  errors for enabled stages; syntactically invalid names still fail validation.
- Risk integration tests: zero, one, and two enabled review reports; no blank or
  stale references; missing review evidence is explicit when reviews are skipped.
- Output/log checks: skip messages are visible; skipped stages create no prompt,
  report, or executed-stage result artifacts; `reviewChanges` can be suppressed.

## Boundaries

- **Always:** Validate before agent execution; preserve omitted-field behavior;
  keep stage order and enabled-stage semantics; document the public JSON field;
  run `go test ./...`, `go vet ./...`, and the build before submitting code.
- **Ask first:** Add dependencies, change branch selection, bypass shared
  preparation, change report retention, expand standalone `review`, introduce
  aliases or flags, or change unrelated configuration validation.
- **Never:** Commit credentials or local task/configuration content; launch live
  paid agents during automated verification; reorder stages; silently ignore
  unknown stage identifiers; disable downstream stages automatically; modify
  unrelated worktree changes.

## Success Criteria

1. `disabledStages` is documented and accepted in `software-factory.json`; absent
   and empty lists preserve all existing command behavior.
2. Each of the five stages can be independently disabled wherever applicable,
   with no callback, subprocess, custom-skill file load, or stage artifact for
   that stage.
3. Default, `review`, and `continue` workflows consistently apply the exclusions
   without changing which stages each command normally includes or their order.
4. Invalid JSON, invalid list types or entries, and unknown stage names cause a
   clear nonzero-exit error before any agent executes.
5. Enabled downstream stages still execute; risk classification uses only current
   enabled-review evidence, including the zero-report case.
6. Skip messages identify the suppressed stages; skipped stages are not reported
   as executed successfully or unsuccessfully.
7. An all-disabled invocation executes no agents and succeeds when its unchanged
   shared preparation succeeds.
8. README examples explain valid names, default behavior, validation failures,
   and the existing-worktree requirement when implementation is disabled.
9. The test suite, vet, and build pass using controlled fixtures.

## Review Gate

The established requirements are recorded above. The user delegated all open
decisions, so the proposed decisions are adopted as written: strict rejection
of `null`, accepted duplicates, `continue` applicability, and preservation of
branch selection and shared preparation. The next step is a technical plan;
code changes follow that plan.
