# Spec: Agent and model in stage summaries

Status: Approved.

## Objective

Help the person reviewing a Soft Factory run identify the agent and model used
by each executed stage directly in `summary.log`. Add two compact fields to
future summaries, preserving the existing stage result information.

The confirmed intent is to show only the final attempt's agent and model,
including when that attempt uses a fallback. Do not show attempt history.

## Scope and output contract

Apply this behavior wherever the workflow currently records numbered stage
results, including implementation, code review, security review, and risk
classification in initial, continued, and review-only workflows.

Each numbered stage entry contains exactly one `Agent:` line and one `Model:`
line, after `Status:` and before `Duration:`. Match the existing capitalization,
indentation, and value alignment:

```text
1. IMPLEMENTATION
   Status:    SUCCEEDED
   Agent:     x
   Model:     y
   Duration:  20 sec
   Changes:   Created hello.txt containing Hello, world!
```

- Values identify the agent and model of the final attempt actually started for
  that stage. A retry or fallback can replace the earlier selection.
- Apply the same rule to successful, failed, and interrupted stages. If a stage
  fails before starting an attempt, there is no runtime selection to report.
- Prefer authoritative runtime selection metadata over configured defaults.
  Configuration alone must not be presented as proof of the final selection.
- Render an undetermined value as `unavailable`, independently for each field.
  Missing metadata does not turn a successful stage into a workflow failure.
- Print names only, without configuration-source annotations or provider details.
- Reset stage metadata between stages so an unknown selection cannot inherit
  values from an earlier stage.
- Preserve existing skipped-stage messages. A disabled stage used no agent/model
  and does not gain a numbered result entry.
- Preserve headers, footers, numbering, status, duration, changes, and errors.

## Tech Stack

Go 1.26.0 or later, using the existing CLI and logging packages. The CLI launches
`agent-sandbox` as a subprocess. No new dependencies are proposed.

## Commands

Run from the repository root:

```sh
go build -o /tmp/software-factory ./cmd/factory
go test ./internal/executionlog ./internal/sandbox ./cmd/factory
go test ./...
go vet ./...
gofmt -w cmd internal
```

Optional manual verification with local configuration and `agent-sandbox` on
`PATH`: `go run ./cmd/factory` or `go run ./cmd/factory review`. These execute
real agent workflows and are not required for deterministic automated tests.

## Project Structure

- `internal/executionlog/log.go`: stage lifecycle and `summary.log` rendering.
- `internal/executionlog/log_test.go`: summary format and lifecycle tests.
- `internal/sandbox/runner.go`: subprocess execution and selection metadata.
- `internal/sandbox/*_test.go`: controlled subprocess/metadata fixtures.
- `cmd/factory/main.go`: stage orchestration and result recording.
- `cmd/factory/*_test.go`: workflow integration tests when needed.
- `internal/config/`: existing configuration precedence and stage overrides.
- `site-docs/`: user-facing documentation if existing summary examples need updates.
- `SPEC-summary-agent-model.md`: this specification, retained in version control.

## Code Style

Use idiomatic Go, `gofmt`, lowercase package names, PascalCase exported names,
and camelCase private names. Wrap underlying errors with `%w`, as the existing
summary writer does:

```go
if _, err := io.WriteString(r.summary, entry+"\n"); err != nil {
    return fmt.Errorf("write stage summary: %w", err)
}
```

Keep summary output deterministic and metadata capture separate from the
agent-authored change description.

## Testing Strategy

Use standard-library `testing` with temporary directories and controlled
subprocess or metadata fixtures. No coverage percentage is required.

Verify the new fields for a normal stage, a retry with unchanged selection, and
a fallback with a changed agent/model. Verify failed and interrupted stages,
missing values, failure before launch, and metadata reset between stages.

Check exact summary formatting and that no attempt list, attempt numbers,
retry/fallback annotations, or configuration-source annotations are added.
Retain assertions for existing result fields and skipped-stage behavior.
Run the build, full test suite, and `go vet` before submitting code changes.

## Boundaries

- Always: Report the final runtime selection when known; preserve existing
  summary fields; keep stage metadata isolated; run required verification.
- Ask first: Changes to `agent-sandbox` outside this repository, dependency
  additions/upgrades, or changes to retry/fallback execution behavior.
- Never: Add attempt history to the summary, rewrite old logs, log credentials
  or complete configuration files, or change unrelated user work.

## Success Criteria

1. Every executed stage result in a newly generated summary has exactly one
   `Agent:` and one `Model:` line in the specified positions.
2. When a fallback starts with different values, those final values appear;
   earlier values and attempt history do not appear in the new fields.
3. A final failed/interrupted attempt reports its known values; unknown values
   appear as `unavailable` without leaking metadata from another stage.
4. Existing summary information and skipped-stage behavior remain intact.
5. Existing historical summaries are untouched.
6. Build, tests, and vet pass without requiring live agent execution.

## Open Questions and implementation risks

No product questions remain. The implementation plan must establish a reliable
source for final runtime agent/model metadata: the current `selectionDetails`
helper describes configured selection rather than proving fallback outcomes.
If the installed sandbox does not expose reliable metadata, identify that
limitation before implementation and request approval for any external change.
