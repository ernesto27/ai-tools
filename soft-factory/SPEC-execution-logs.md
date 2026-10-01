# Spec: Execution Logs

Status: Implementation authorized with the approved plain-text format.
User instruction: add the missing automated tests. This supersedes the earlier
request to defer automated tests until after manual verification.

## Objective

Add a Go package that saves enough execution history to debug a completed,
failed, or partially completed Soft Factory run without watching the terminal.
The user is the developer running the CLI. Success means they can open local
files and determine which stages ran, what prompts and commands were used,
what output was produced, and where execution stopped.

This is one capability: persistent execution history. Prompt snapshots,
command records, stage metadata, and output capture belong to the same history
and do not require a separate capability map.

## Confirmed Intent

- Save logs under `logs/`, organized by execution and stage.
- Keep storage simple and readable: one plain-text `.log` file per execution,
  with stage headings and multiline prompts/output. This format is approved.
- Record model, provider, executed prompts, results, terminal output, errors,
  and commands used.
- Include commands run internally by the agent when available.
- Preserve failed and partial runs.
- Exclude token usage, cost tracking, a log viewer, and a new CLI command.

## Assumptions Proposed for Review

1. Logging is automatic for the default workflow, currently invoked without a
   `run` positional argument. Do not introduce a new `run` subcommand.
2. `logs/` resolves relative to the CLI's working directory, like existing
   report storage. The logging package owns this fixed path; `NewRun()` takes
   no directory parameter. No log-path configuration is provided.
3. Review stages inside the default workflow are logged; the standalone
   `review` command is outside this initial scope.
4. Capture begins after successful CLI argument parsing and before environment,
   configuration, task, and supporting-document loading. Help and invalid CLI
   arguments do not create log directories.
5. Logging is best effort, per the user's explicit correction. If initialization,
   writing, or closing fails, warn once and continue execution and later stages.
   Retain already saved text. Logging failure never changes execution status or
   the CLI exit code; genuine workflow and existing report errors still do.
6. Logs remain local until the user removes them. No rotation or retention
   policy is introduced.

## Current Behavior and Evidence

- `cmd/factory/main.go` sequences implementation, code review, security review,
  and risk classification. Command failures stop later stages.
- Before this change, `internal/sandbox/runner.go` starts `agent-sandbox run` for implementation
  and `agent-sandbox resume -f <temporary-prompt>` for later stages.
- Implementation stdout and stderr currently go directly to the terminal.
- Review and risk output is combined, displayed, and saved under `docs/`.
- Generated prompt files are deleted after each stage.
- Local tasks come from `run.file-prompt` in `agent-sandbox.json`; Jira input
  can replace the task, and supporting documents are assembled before execution.
- The installed `agent-sandbox run --help` lists `--model` and agent selection,
  but no structured event-output flag. This does not establish whether other
  event/session interfaces exist; planning must investigate them before
  promising internal-command extraction or effective model/provider detection.
- Interface investigation found local source at
  `/home/eponce/code/better-development/skills/agent-sandbox/go`: agent adapters
  use text/terminal modes; Docker forwards stdout/stderr (combined for TTY),
  and run/resume expose no separate command-event feed. Preserve all emitted
  output, identify structured command capture as unavailable, and avoid
  guessing effective model or API-backend values from configured agent names.
  The user subsequently clarified that Provider means the selected coding agent,
  so obtain it directly from the sandbox's agent setting.

## Tech Stack

- Go 1.25.3 or later, matching `go.mod`.
- Existing `os/exec` subprocess integration with `agent-sandbox`.
- Standard-library filesystem, text formatting, time, and I/O facilities.
  Existing JSON configuration reading remains unchanged.
- No additional dependency required for basic log persistence.

## Commands

Run from the repository root:

```bash
go build -o /tmp/factory ./cmd/factory
go test ./...
go vet ./...
gofmt -w cmd internal
```

Manual workflow verification, only with a controlled task and configured sandbox:

```bash
go run ./cmd/factory -config config.json
go run ./cmd/factory -config config.json --jira 'https://your-company.atlassian.net/browse/ENG-123'
```

Tests must use controlled subprocess fixtures rather than live agents or APIs.
The standalone review command retains its current behavior:

```bash
go run ./cmd/factory -config config.json review
```

## Project Structure

```text
SPEC-execution-logs.md       Version-controlled feature specification
internal/executionlog/      New package owning log storage and lifecycle
internal/executionlog/*_test.go
                            Package tests using temporary directories
internal/sandbox/runner.go  Prompt and subprocess capture integration
internal/sandbox/*_test.go  Controlled subprocess integration tests
cmd/factory/main.go         Run lifecycle and stage sequencing
cmd/factory/*_test.go       Workflow and failure-path tests where needed
README.md                  User instructions for locating and reading logs
.gitignore                 Ignore generated logs/
logs/                      Generated execution history; never committed
docs/                      Existing generated review reports
```

## Required Behavior

### Execution and Stage Lifecycle

Each invocation creates a unique file that cannot overwrite a previous
or concurrent run. Print its path when initialized. Record
run ID, working directory, start time, end time when known, status, and errors.
Use UTC timestamps with RFC 3339 precision consistently.

Create permanent log files with `os.OpenFile` and `O_WRONLY|O_CREATE|O_EXCL`,
using a UTC timestamp with nanoseconds as the filename. If that name already
exists, retry with an incrementing numeric suffix without modifying the
existing file. Do not use `CreateTemp` to create execution logs.

The run header lists all four planned stages in execution order:

1. `implementation`
2. `code-review`
3. `security-review`
4. `risk-classification`

Stage statuses distinguish pending, running, succeeded, failed, and interrupted.
A preparation failure is attached to its stage before subprocess launch.
Preflight failures are attached to the run. Never label an unstarted stage as
successful; retain pending stages when an earlier failure stops the workflow.

Persist the run header before execution and append sections at lifecycle
boundaries. Never rewrite earlier content. Stream output to disk during execution
rather than waiting until process exit or buffering the entire run in memory.

On a handled interrupt, finalize the run and active stage as interrupted when
possible. A missing execution-finished section indicates an incomplete execution.
Abrupt termination may leave a partial final section; earlier saved text must
remain readable. Document this behavior. No guarantee is made for unflushed
bytes after forced termination or power loss.

### Storage Contract

Use one append-only plain-text `.log` file per execution. Prompts and output
appear as ordinary multiline text, without JSON encoding or escaped newlines.
No per-run or per-stage directories, separate manifests, or extra log artifacts:

```text
logs/
  <unique-run-id>.log
  <another-run-id>.log
```

Use clear separators and human-readable labels. The approved structure is:

```text
EXECUTION
Run: <run-id>
Started: <timestamp>
Directory: <working-directory>
Planned stages: implementation, code-review, security-review, risk-classification

==================================================
STAGE 1: IMPLEMENTATION
==================================================
Started: <timestamp>
Model: <selection and source, or unavailable with reason>
Provider: <selected agent from agent-sandbox.json, or unavailable with reason>
Command: agent-sandbox run -f /tmp/factory-prompt.txt

--- PROMPT ---
<complete multiline prompt>

--- OUTPUT ---
<output streamed as it arrives>

--- AGENT COMMAND ---
<internal command, when exposed>
Result: <outcome, when exposed>

--- RESULT ---
Status: SUCCEEDED
Exit code: 0
Finished: <timestamp>

[Remaining stages use the same structure]

==================================================
EXECUTION FINISHED
Status: SUCCEEDED
Finished: <timestamp>
```

Model, provider, and internal-command information may become available during
execution; append labeled updates where observed rather than delaying output
or rewriting earlier text. Repeat output and agent-command sections as needed
to preserve observed order. Use an `ERROR` section for execution or preparation
failures. The final summary identifies unstarted stages as pending.

Retain workflow messages and subprocess output in observed arrival order.
Use a readable stderr label when switching to stderr, without prefixing every
line or escaping its contents for implementation output. For report-producing
stages, assign the same writer instance to subprocess stdout and stderr so
`os/exec` shares one pipe and copy goroutine. Tee that combined stream to the
terminal, execution log, and report buffer; label it `OUTPUT (stdout + stderr)`.
Do not wrap the streams separately, which would introduce cross-pipe reordering.
Serialize concurrent log writes so headings and chunks do not corrupt each
other. Do not promise perfect original ordering between independently emitted
implementation streams or reorder output buffered by the child itself.
Do not truncate captured output.

Stage sections record identity, timestamps, status, command, working directory,
subprocess exit code when available, errors, agent selection when available,
model/provider metadata, capture availability, and existing report path when
applicable. Startup failures have no exit code. Render executable and arguments
with unambiguous quoting where needed so paths with spaces or empty arguments
remain understandable. The format is intended for reading, not machine parsing.

Results are the captured stage output and existing review/risk reports. Do not
invent a separate final-answer extractor when the agent only exposes raw output.
A zero exit code means execution succeeded, independent of review findings such
as `UNRESOLVED` or `BLOCKED` appearing in the result.

### Prompt Capture

Save the complete prompt supplied to each subprocess before starting it.
Preserve generated text including task overrides, supporting documents, review
skills, and risk-stage report content. For an implementation that relies on
`run.file-prompt`, snapshot that resolved file rather than saving only its path.
The executed prompt must match the saved snapshot even if its source file later
changes. Temporary prompt removal must not remove the saved snapshot.

The snapshot covers the prompt supplied by Soft Factory, not hidden system
instructions or additional context injected internally by an agent.

### Model, Provider, and Internal Commands

Provider means the selected coding agent, per explicit user clarification.
Read `run.agent` from `agent-sandbox.json` for implementation and `resume.agent`
for reviews/risk classification. Log the value and its source, such as
`Provider: claude (agent-sandbox.json: run.agent)`. Do not infer an API backend
such as Anthropic or OpenAI. If the agent setting is missing or unreadable,
mark Provider unavailable with a reason and continue execution.

Distinguish configured model selection from effective execution metadata.
Record the source of a known value: configuration, verified runtime event, or
supported session metadata. Agent names do not prove an effective model. Use
explicit unknown/unavailable values with a reason when defaults or runtime
details cannot be determined.

Preserve internal command events when an available sandbox/agent interface
exposes them. The same run file records the event source, observed time, command
information, and result/status when exposed. Associate events with their stage
and subagent/session identifiers when provided. Never fabricate missing events
or infer command execution merely because a command appears in prose.

If output is the only source, save it intact and identify structured internal
command capture as unavailable. Unsupported formats must not prevent basic
logging. If a structured interface exists, preserve available events even when
some event fields are unknown. No cross-provider guarantee or sandbox fork is
required by this first version.

### Compatibility and File Handling

Continue displaying terminal output during execution. Preserve the workflow's
stage order, prompts, report format, report paths, and failure behavior except
for the explicitly proposed log-persistence failure policy.

Create log directories with owner-only access and files with owner-only
read/write access. Ignore `logs/` in Git. Do not copy credentials files, full
environment variables, or entire configuration files into the logs. Required
prompts/output may contain sensitive task content and should remain exact local
snapshots; automatic content redaction is outside scope. Never add credential
values to diagnostic metadata.

## Code Style

Use lowercase package names, PascalCase exported identifiers, camelCase private
identifiers, and `gofmt` formatting. Keep persistence reusable under
`internal/executionlog/` and workflow orchestration in `cmd/factory/`. Wrap
underlying errors with `%w`, consistent with existing `saveReport`:

```go
if err := os.WriteFile(path, content, 0600); err != nil {
	return "", fmt.Errorf("save report %q: %w", path, err)
}
```

Example user-visible output: `Execution logs: logs/<run-id>.log`.

## Testing Strategy

The user subsequently requested automated tests. Test files cover the logging
package, sandbox subprocess integration, and CLI lifecycle using temporary
directories and controlled local subprocesses; no real agent or API is invoked.

Use the standard-library `testing` package, colocated `*_test.go` files, temporary
directories, and controlled subprocess fixtures. There is no coverage threshold.
Test observable behavior rather than duplicating implementation details.

Keep automated tests focused on important behavior, per the user's request:

- Storage: filename collisions never overwrite saved executions; prompts and
  output are saved; stage outcomes and pending stages remain readable.
- Concurrency: concurrent output chunks are not lost or corrupted.
- Logging failure: warn once, preserve saved output, and continue accepting
  subprocess output. Initialization failures do not stop workflow stages.
- Subprocess integration: report stdout/stderr share one pipe and preserve
  observed order; failed/interrupted commands retain partial output; the logged
  prompt matches the executed snapshot and its temporary file is cleaned up.
- Workflow: all four stages run in order on success, and genuine execution
  failures prevent later stages from starting.

Avoid separate tests for every formatting helper or metadata fallback. These
tests use real filesystem operations and controlled subprocess fixtures.

Run build, `go test ./...`, `go test -race ./...`, and vet for this implementation.
Future code changes should run build, tests, and vet unless instructed otherwise.
A spec-only change does not require running an agent or the Go test suite.

## Boundaries

- Always: preserve existing user changes; save available evidence before moving
  to the next stage; distinguish unknown values; return contextual errors;
  keep generated logs ignored; run required checks for code changes.
- Ask first: expand logging to standalone `review`; add dependencies; change
  the approved storage contract or persistence-failure policy; modify external
  sandbox/agent software; add retention, configuration options, or a viewer.
- Never: fabricate model/provider/command data; hide persistence errors; commit
  logs or credentials; change the task to obtain richer telemetry; publish
  changes or invoke real agent workflows as an automated test.

## Success Criteria

1. A successful default workflow produces exactly one unique log file and four
   identifiable successful stage sections with prompts and complete captured
   output, while terminal output and existing reports remain available.
2. Prompt snapshots survive temporary-file cleanup and match subprocess input
   for local tasks, task overrides, supporting context, reviews, and risk prompts.
3. Each attempted subprocess records its executable, unambiguously quoted arguments,
   working directory, timing, and exit outcome or startup error.
4. Provider records the selected agent from the correct sandbox configuration
   section. Configured models are recorded without claiming an effective model;
   unknown values carry an availability reason. API backends are not inferred.
5. Available internal command events and their exposed outcomes are persisted;
   unavailable structured capture is identified explicitly while raw output
   remains saved.
6. Nonzero exit, preparation failure, and handled interruption retain output
   already captured and identify the failed/interrupted stage and pending stages.
7. Logs are visible on disk during execution, concurrent run files cannot collide,
   concurrent writes cannot corrupt logger sections or output chunks, and saved
   text remains readable after an interrupted append. Prompts and output are
   ordinary multiline text, with no JSON/JSONL encoding or sidecar log files.
8. Logging failures issue one warning and do not stop current or later stages,
   change successful execution status, or replace the original workflow error.
9. Preflight errors after valid argument parsing have a run record; help and
   argument errors do not create execution history.
10. Generated logs are ignored, created with private permissions, and contain no
    added dumps of credentials, environment, or full configuration.
11. No cost/token tracking, new CLI command, viewer, or standalone-review logging
    is added; build, automated tests, race checks, and vet pass.

## Open Questions and Review Gate

The user approved the single-file plain-text format and explicitly authorized
implementation, initially without tests. The later request to add missing tests
supersedes that testing constraint. Assumptions above define the implemented
scope; real-agent manual verification remains separate from the fixture tests.

The sandbox interface limitation is documented under Current Behavior and
Evidence. Internal commands and effective model details printed at runtime
remain in OUTPUT; unsupported structured capture is explicitly unavailable.
Provider is the configured sandbox agent, as clarified by the user.
