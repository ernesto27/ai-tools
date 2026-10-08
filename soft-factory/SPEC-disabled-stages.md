# Spec: Configurable Disabled Pipeline Stages

## Objective

Let Soft Factory users list stages in `software-factory.json` that must not
execute. Users can disable any existing stage without editing code or changing
stage order. Both the default workflow and `review` honor this configuration.
Invalid configuration must produce a clear error rather than silently running a
stage the user intended to disable.

This is one capability: configuration-driven stage suppression. Configuration
validation, workflow gating, and report handling are parts of that capability.

## Requirements

- Use the camelCase JSON property `disabledStages` and camelCase stage values
  such as `securityReview` and `riskClassification`.
- Apply it to both the default workflow and the `review` command.
- Allow any of the five existing stages to be disabled.
- Validate configuration and show an error for unknown stage names.

## Decisions

1. Public stage identifiers are exact, case-sensitive camelCase strings.
   Configuration values map to existing internal pipeline identifiers;
   internal log labels, report names, and skill paths are not renamed.
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

```json
{
  "documents": ["task.txt"],
  "disabledStages": ["securityReview", "riskClassification"]
}
```

| Stage identifier | Default workflow | `review` | `continue` |
|---|---|---|---|
| `implementation` | Yes | No | Yes, using existing continuation behavior |
| `codeReview` | Yes | Yes | Yes |
| `securityReview` | Yes | Yes | Yes |
| `riskClassification` | Yes | Yes | Yes |
| `reviewChanges` | Yes | No | Yes |

A valid identifier is accepted even if the selected command never runs that
stage. The list is a set of exclusions, not an execution-order list.

The entire list is validated before any agent runs or external task context
loads. Errors identify `disabledStages`, the offending array index or value
where applicable, and the allowed identifiers for unknown names:

```text
Error: validate factory configuration: disabledStages[0]: unknown stage "securityReveiw"; allowed: implementation, codeReview, securityReview, riskClassification, reviewChanges
```

Malformed JSON and other existing configuration errors continue to be rejected.
No global unknown-property validation or `disabled-stages` alias is introduced.

## Workflow Behavior

- Run only stages normally included by the selected command and not listed in
  `disabledStages`, preserving the relative order of enabled stages.
- Preserve existing failure, interruption, and best-effort walkthrough behavior
  for enabled stages.
- A disabled stage is intentionally skipped, not failed or successful. Print
  `Skipping disabled stage: <camelCase identifier>` when reaching it and record
  that line in `summary.log` when execution logging is active, without stage
  log files, numbered results, or report artifacts.
- Custom skill files are not resolved for disabled stages. JSON validation
  still applies to configured custom skill names.
- Risk classification never receives empty report paths or old reports from
  disabled reviews. With both reviews disabled it inspects changes without
  review reports, and its prompt describes the missing review evidence.
- Disabling risk classification or implementation does not prevent enabled
  reviews or the enabled final walkthrough from executing.
- If every applicable stage is disabled, no agent subprocess runs and the
  command succeeds after normal shared preparation succeeds.
- Pre-existing reports are not removed or overwritten because their stages are
  disabled.
- Logging eligibility stays tied to the command; standalone `review` gains no
  execution logging.

## Verification

Run from the `soft-factory` directory:

```bash
go build -o /tmp/software-factory ./cmd/factory
go test ./...
go vet ./...
gofmt -l cmd internal
```

Automated workflow tests use a stub `agent-sandbox` on `PATH` and temporary Git
repositories; they never invoke live agents, Jira, or Google Drive.

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
