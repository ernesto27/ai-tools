# Spec: Agent and model in stage summaries

Status: Approved.

## Objective

Help the person reviewing a Soft Factory run identify the agent and model used
by each executed stage directly in `summary.log`. Add two compact fields to
future summaries, preserving the existing stage result information.

Show only the final attempt's agent and model, including when that attempt uses
a fallback. Do not show attempt history.

## Scope and output contract

Apply this wherever the workflow currently records numbered stage results,
including implementation, code review, security review, and risk classification
in initial, continued, and review-only workflows.

Each numbered stage entry contains exactly one `Agent:` line and one `Model:`
line, after `Status:` and before `Duration:`:

```text
1. IMPLEMENTATION
   Status:    SUCCEEDED
   Agent:     x
   Model:     y
   Duration:  20 sec
   Changes:   Created hello.txt containing Hello, world!
```

- Report the agent and model of the final attempt actually started. A retry or
  fallback can replace the earlier selection.
- Apply the same rule to successful, failed, and interrupted stages. A stage
  failing before starting an attempt has no runtime selection to report.
- Prefer authoritative runtime selection metadata over configured defaults.
  Configuration alone is not proof of the final selection.
- Render an undetermined value as `unavailable`, independently for each field.
  Missing metadata must not turn a successful stage into a workflow failure.
- Print names only, without configuration-source annotations or provider details.
- Reset stage metadata between stages to prevent inherited selections.
- Preserve skipped-stage messages. Disabled stages gain no numbered entry.
- Preserve headers, footers, numbering, status, duration, changes, and errors.
- Do not rewrite historical summaries or log credentials or full configurations.

## Tech stack and project structure

Use Go 1.26.0 or later, existing CLI and logging packages, and no new dependencies.
The CLI launches `agent-sandbox` as a subprocess.

- `internal/executionlog/log.go`: stage lifecycle and summary rendering.
- `internal/executionlog/log_test.go`: summary formatting and lifecycle tests.
- `internal/sandbox/runner.go`: subprocess execution and selection metadata.
- `internal/sandbox/*_test.go`: controlled subprocess and metadata fixtures.
- `cmd/factory/main.go`: stage orchestration and result recording.
- `cmd/factory/*_test.go`: workflow integration tests when needed.
- `internal/config/`: configuration precedence and stage overrides.
- `site-docs/`: user-facing summary documentation.
- `SPEC-summary-agent-model.md`: this specification, retained in version control.

## Code style

Use idiomatic Go and gofmt. Wrap underlying errors with `%w`. Keep output
deterministic and metadata capture separate from agent-authored change reports.

## Testing and verification

Use standard-library testing, temporary directories, and controlled subprocess
or metadata fixtures; no live agents or coverage threshold are required.
Verify normal stages, retries with unchanged selections, fallbacks with changed
agent/model, failures, interruptions, missing fields, prelaunch failures, and
metadata isolation. Assert exact summary formatting, existing result fields,
and skipped-stage behavior, with no attempt or configuration annotations.

Run from the repository root:

```sh
go build -o /tmp/software-factory ./cmd/factory
go test ./internal/executionlog ./internal/sandbox ./cmd/factory
go test ./...
go vet ./...
gofmt -w cmd internal
```

Optional manual verification with local configuration and `agent-sandbox` on
PATH: `go run ./cmd/factory` or `go run ./cmd/factory review`. These execute real
agent workflows and are not required for deterministic tests.

## Boundaries and success criteria

Always report final runtime selections when known, preserve summary information
and stage isolation, and run required verification. Ask first before changing
`agent-sandbox` outside this repository, adding/upgrading dependencies, or
changing retry/fallback execution behavior. Never add attempt history, rewrite
old logs, expose credentials/full configuration, or change unrelated work.

Success means every executed stage result in new summaries has exactly one
agent and model field in the specified positions. Final fallback, failed, and
interrupted attempts report known selections; unknown fields are `unavailable`
without leaking another stage's metadata. Existing fields, skipped stages, and
historical summaries remain intact. Build, tests, and vet pass without live agents.

## Implementation risk

No product questions remain. Establish a reliable runtime source before
implementation: `selectionDetails` describes configured selection and does not
prove fallback outcomes. If the sandbox does not expose reliable metadata,
identify that limitation and seek approval before any external change.
