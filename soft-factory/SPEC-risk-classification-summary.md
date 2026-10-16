# Spec: Risk classification in the execution summary

Status: Draft — awaiting human review.

## Objective

Let CLI users read the actual classification in `summary.log` without opening
the risk-classification stage log. Replace that stage's `Changes` field with
`Classification`, containing only the returned classification label.

This is one capability: presenting the risk-classification result in the
existing execution summary.

## Assumptions

- Keep execution status, agent, model, duration, and execution errors.
- Display the classifier's actual label without an explanation or mapping to
  a different scale. The bundled skill currently uses `LOW`, `MEDIUM`, `HIGH`,
  and `UNKNOWN`. The user's `Small` example describes displaying a result;
  it does not request changing the bundled taxonomy. If a custom skill defines
  a label such as `Small`, display that returned label.
- Use `Classification unavailable.` when no usable classification was collected.
  This fallback is distinct from a classifier explicitly returning `UNKNOWN`.

## Required Behavior

1. Every executed risk-classification summary entry contains a single
   `Classification:` field with the result from that stage attempt.
2. Omit `Changes:` from risk-classification entries, including failed or
   interrupted attempts. Do not request a code-change summary from this stage.
3. Preserve its existing metadata and error reporting. Execution success and
   classification remain separate: `SUCCEEDED` is not a risk verdict.
4. Show only the classification label. Explanations, findings, and verification
   gaps stay in the existing detailed stage output.
5. Missing, empty, unreadable, or malformed result data uses the unavailable
   fallback. Do not infer a label from incidental words in prompts, echoed
   instructions, or unrelated output. A missing result does not itself change
   execution status or stop the workflow.
6. Associate results with the current attempt. Retries and `resume` must not
   reuse an earlier attempt's classification. Existing summary entries remain
   untouched; newly appended entries follow this format.
7. Apply this behavior wherever execution summaries are recorded: default,
   `continue`, and `resume` workflows. The standalone `review` command retains
   its existing report behavior because it does not create `summary.log`.
8. Preserve result artifacts until their summary entry is saved successfully,
   consistent with the existing stage-report lifecycle.

## Structured Result Contract

The risk-classification agent writes a dedicated JSON file at a path supplied
by Soft Factory in its prompt. The summary reads this structured result; it
must not extract a classification from the narrative report or terminal logs.

The complete result format is:

```json
{"classification":"LOW"}
```

For a custom skill returning `Small`, the same contract is:

```json
{"classification":"Small"}
```

- Soft Factory supplies a unique result filename for each executed attempt,
  such as `.soft-factory-risk-result-<unique-id>.json`, and the exact sandbox
  output path. The prompt requires a plain JSON object without Markdown fences.
- The agent performs its assessment, writes the final label into this file,
  and keeps its explanation in the existing detailed stage output. Writing
  this reporting artifact is explicitly allowed by the prompt's read-only
  restriction; implementation files must remain unchanged.
- After the agent exits, Soft Factory collects the file through the sandbox
  worktree reporting mechanism and decodes it using Go's `encoding/json`.
  This is structured JSON decoding, not parsing a prose report.
- Accept exactly one JSON object whose `classification` is a nonempty string
  after trimming surrounding whitespace. Reject missing, null, non-string,
  multiline, or control-character-containing labels, non-object JSON, and
  trailing content. Do not impose the bundled taxonomy on custom labels.
- A missing, unreadable, or invalid file produces `Classification unavailable.`
  with no fallback to searching narrative output. Preserve the stage's actual
  execution status and any execution error.
- Use only regular files and preserve the existing reporting-path protections.
  Ignore this temporary artifact when assessing task changes. Clean it up only
  after its summary entry has been saved successfully.
- Generate this artifact for logged workflows. The standalone `review` command
  keeps its existing report behavior and does not require this new artifact.

Example (field padding may be adjusted for alignment):

```text
4. RISK CLASSIFICATION
   Status:         SUCCEEDED
   Agent:          codex
   Model:          gpt-6.1-sol
   Duration:       53 sec
   Classification: LOW
```

## Tech Stack

Go 1.26.0 or later, existing Cobra CLI, and existing `agent-sandbox` integration.
No new dependencies are required.

## Commands

Run from the repository root:

```sh
go build -o /tmp/software-factory ./cmd/factory
go test ./...
go vet ./...
gofmt -w cmd internal
```

For a manually configured environment with `agent-sandbox` available:

```sh
go run ./cmd/factory
go run ./cmd/factory continue
go run ./cmd/factory resume <existing-invocation-directory>
```

Live workflow commands execute agents and are optional manual verification;
automated tests should use controlled fixtures.

## Project Structure

- `internal/executionlog/log.go`: summary formatting and stage-result lifecycle.
- `internal/executionlog/*_test.go`: summary and result-lifecycle regression tests.
- `internal/sandbox/runner.go`: risk-classification execution and result collection.
- `internal/sandbox/*_test.go`: controlled sandbox and prompt integration tests.
- `internal/prompts/risk-classification.txt`: classification prompt contract.
- `internal/prompts/testdata/`: prompt snapshots, updated if the contract changes.
- `skills/risk-classification/SKILL.md`: existing classification taxonomy.
- `site-docs/guide/reports.md`: user documentation for the summary fields.

Implementation planning will choose the smallest necessary subset of files.

## Code Style

Use idiomatic Go, `gofmt`, explicit return values, and contextual errors with
`%w`. Keep reusable behavior under `internal/`. Follow the existing formatting
style, for example:

```go
if _, err := io.WriteString(r.summary, entry+"\n"); err != nil {
	return fmt.Errorf("write stage summary: %w", err)
}
```

## Testing Strategy

Use standard-library `testing` tests beside the affected packages, temporary
files, and controlled subprocess fixtures. No numerical coverage threshold.

- Cover all bundled classification labels and a custom label such as `Small`.
- Assert that risk entries show classification only and omit `Changes:`.
- Cover missing, empty, unreadable, and malformed result data, including an
  explicit `UNKNOWN` result versus the unavailable fallback.
- Cover the JSON contract: valid object, missing field, wrong value type,
  whitespace-only label, multiline label, non-object JSON, trailing content,
  and misleading classification words in narrative output. Assert that only
  the dedicated JSON artifact supplies the summary value.
- Verify failed and interrupted stage status/error behavior is preserved.
- Verify repeated attempts and resumed runs use their own result.
- Verify result retention when saving the summary fails.
- Verify other stages keep their existing change summaries and skipped-stage
  behavior remains unchanged.
- Update prompt snapshots if result instructions change; verify result collection
  through the sandbox boundary using fixtures rather than a live agent.
- Update reports documentation to match the final behavior and run its required
  site build checks during implementation.

## Boundaries

- **Always:** Keep the classification read-only with respect to implementation
  files; preserve existing status/error semantics and artifact handling; update
  affected documentation; run the build, tests, and vet before submitting code.
- **Ask first:** Changing the classification taxonomy, adding dependencies,
  changing configuration or CI, or expanding this change into other summaries.
- **Never:** Invent a verdict when collection fails, expose credentials, commit
  generated logs, or remove failing tests to make checks pass.

Out of scope: changing risk-assessment logic, adding classification explanations
to the summary, changing other stages' fields, rewriting historical summaries,
and changing standalone review report storage.

## Success Criteria

- A classification decoded from the stage's dedicated JSON artifact appears
  in the corresponding summary entry; narrative reports and logs are never
  parsed to obtain it.
- Risk-classification entries contain no `Changes:` line or explanation text.
- Existing status, agent, model, duration, and error information remains present.
- Unavailable results are explicit and never presented as `UNKNOWN` verdicts.
- Default, continue, and resume summaries behave consistently without stale results.
- Other stage summaries and standalone review behavior remain compatible.
- Relevant regression tests, `go test ./...`, `go vet ./...`, and the CLI build pass.

## Open Questions

No blocking questions. Human review should confirm the actual-label behavior
and the proposed `Classification unavailable.` fallback before planning.
