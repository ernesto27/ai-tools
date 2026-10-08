# Spec: CamelCase Configuration Properties

Status: Implemented. The proposed scope and rejection behavior were accepted as
written. The obsolete `code-review-skill` property points to its functional
replacement, `customSkills.codeReview`; no `codeReviewSkill` property exists.

## Objective

Give developers configuring Software Factory one consistent naming convention:
all properties owned by `software-factory.json` use lower camelCase, including
nested properties. This is a breaking configuration change with no fallback
support for previous spellings.

This is one capability: the factory configuration naming contract. No capability
map is required.

## Requirements and Assumptions

- The user requested camelCase for all properties and explicitly excluded fallback support.
- Scope: update loading, validation diagnostics, configuration examples,
  documentation, configuration references in skill instructions, and tests.
- Rejection behavior: old property names cause a clear configuration error
  before document loading, external clients, or agent execution. They must
  never be silently ignored, translated, or accepted alongside new names.
- CLI flags, command names, workflow stage identifiers, skill names and paths,
  and the external `agent-sandbox.json` contract retain their existing spelling.
- Property values, defaults, optionality, and existing content validation retain
  their current behavior. General unknown-key policy is outside this change;
  rejection of obsolete or incorrectly cased known property names is required.

## Configuration Contract

| Existing property path | Required property path |
|---|---|
| `documents` | `documents` |
| `google_drive` | `googleDrive` |
| `google_drive.folders` | `googleDrive.folders` |
| `google_drive.files` | `googleDrive.files` |
| `custom-skills` | `customSkills` |
| `custom-skills.code-review` | `customSkills.codeReview` |
| `custom-skills.security-review` | `customSkills.securityReview` |
| `custom-skills.risk-classification` | `customSkills.riskClassification` |
| `custom-skills.review-changes` | `customSkills.reviewChanges` |

Canonical example:

```json
{
  "documents": ["task.txt"],
  "googleDrive": {
    "folders": ["Project documents"],
    "files": ["https://docs.google.com/document/d/example-document-id/edit"]
  },
  "customSkills": {
    "codeReview": "test-code-review",
    "securityReview": "test-security-review",
    "riskClassification": "test-risk-classification",
    "reviewChanges": "test-review-changes"
  }
}
```

Only names change; for example, the skill value `test-code-review` remains
kebab-case. Users must manually rename configuration keys. No migration command,
automatic rewriting, or compatibility period is introduced.

Reject every renamed old key, including when its value is null, empty, or present
alongside the canonical key. Nested obsolete keys inside `customSkills` also fail.
Reject incorrect casing of known names rather than accepting Go JSON decoding's
case-insensitive matches. The obsolete top-level `code-review-skill` property
must fail for every value; do not invent a supported `codeReviewSkill` property.

Errors identify the offending property path and the canonical replacement when
one exists. For example:

```text
validate factory configuration: customSkills.code-review is unsupported; use customSkills.codeReview
```

Existing validation errors and custom-skill resolution errors must reference
canonical paths, such as `googleDrive.files[0]` and `customSkills.codeReview`.
Workflow stage labels may still use `code-review` independently of JSON paths.

## Previous Behavior

`internal/config/config.go` declared mixed snake_case and kebab-case JSON tags.
`Load` used `json.Unmarshal`, which ignores unknown fields and accepts matching
field names without regard to case. It also had a special null-only check for
the old `code-review-skill` property. `cmd/factory/main.go` used old configuration
paths in skill-resolution diagnostics. Tests and README examples used the old
spellings.

## Tech Stack

- Go 1.25.3 or later, matching `go.mod`.
- Standard-library JSON decoding and `testing`.
- Existing Cobra CLI and Google Drive integration; no dependency changes required.

## Commands

Run from the repository root:

```bash
go build -o /tmp/software-factory ./cmd/factory
go test ./...
go vet ./...
gofmt -w cmd internal
```

Manual workflow commands, only when deliberately testing with configured local
credentials, tasks, and `agent-sandbox`:

```bash
go run ./cmd/factory
go run ./cmd/factory review
```

Automated verification must not invoke live agents or Google APIs.

## Project Structure

- `internal/config/config.go`: configuration tags, property-name checks, loading, and validation.
- `internal/config/config_test.go`: table-driven configuration contract tests.
- `cmd/factory/main.go`: configuration-path diagnostics in orchestration.
- `cmd/factory/main_test.go`: integration tests for skill configuration and errors.
- `README.md`: examples and manual migration guidance.
- `skills/` and `.agents/skills/`: no references to this JSON contract required changes.
- `SPEC-camel-case-config.md`: this specification; generated reports remain under ignored `docs/`.

## Code Style

Keep exported Go names in PascalCase, local names in camelCase, and JSON tags
in lower camelCase. Use `gofmt` and wrap underlying errors with `%w`.

## Testing Strategy

Use standard-library, table-driven tests beside the affected packages with
temporary configuration files. No coverage percentage is imposed.

- Verify each canonical property loads correctly, including a complete example,
  omitted optional sections, and empty custom-skill settings.
- Reject each renamed old key at its applicable nesting level, alone and mixed
  with canonical keys, including null and empty values.
- Reject incorrect casing of known properties and obsolete `code-review-skill`
  with both null and non-null values.
- Assert errors identify the rejected path and replacement where applicable.
- Preserve document resolution, skill-name validation, folder validation, and
  Google Docs URL validation; assert canonical paths in diagnostics.
- Update CLI tests to use canonical keys and verify canonical skill-resolution errors.
- Verify invalid configuration stops before downstream execution with controlled
  fixtures or existing test seams; do not depend on live services.

## Boundaries

- **Always:** Use canonical property names, reject obsolete known keys, preserve
  existing value semantics, keep diagnostics actionable, and run build/tests/vet.
- **Ask first:** Expand the configuration schema, impose a new general unknown-key
  policy, add dependencies, change CI, or modify an external configuration contract.
- **Never:** Add fallback aliases, automatically rewrite user configurations,
  change CLI flags or skill values to camelCase, commit credentials or local
  configuration, edit vendor code, or delete tests to hide failures.

## Success Criteria

1. Every supported factory configuration property uses its exact canonical name.
2. Old or incorrectly cased known names fail clearly, including mixed old/new input.
3. No obsolete setting is silently dropped or used as a fallback.
4. Loading and skill-resolution diagnostics use canonical configuration paths.
5. Documentation and examples show camelCase and explain the breaking manual migration.
6. Existing value validation and workflow behavior continue to work with renamed keys.
7. Build, `go test ./...`, and `go vet ./...` pass.
8. No changes are made to CLI naming or `agent-sandbox.json`.
