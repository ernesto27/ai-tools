# Reports

After a run, open `summary.log` to see each stage's result. Then read
`05-reviewChanges.md` to understand the changes, or check the stage logs for
more detail. Output also appears in the terminal.

## Where files are saved {#execution-logs}

Soft Factory prints the log folder when a run starts. It lives under:

```text
~/.config/software-factory/logs/<project>-<id>/<branch>/
```

Each run creates a subfolder with its date and time:

| Command | Folder |
| --- | --- |
| `software-factory` | `run-<timestamp>` |
| `software-factory continue` | `continue-<timestamp>` |
| `software-factory resume <path>` | Reuses the supplied folder and appends new results. |

## Which file to open

| File | Contents |
| --- | --- |
| `summary.log` | Each stage's status, duration, agent, model, and summary. |
| `01-implementation.log` | Implementation details. |
| `02-codeReview.log` | Code review and corrections. |
| `03-securityReview.log` | Security review and findings. |
| `04-riskClassification.log` | Risk classification. |
| `05-reviewChanges.md` | File-by-file explanations, changed lines, and diagrams. |
| `stage_status.txt` | Saved progress for recovery with `resume`. |

Skipped stages create no files. The final walkthrough is saved when it
completes successfully; its `.log` file contains execution details.

## How to read the result

The execution status tells you whether a stage completed:

- `SUCCEEDED`: completed successfully.
- `FAILED`: failed.
- `INTERRUPTED`: was interrupted.

Reviews also have their own verdict:

| Verdict | Meaning |
| --- | --- |
| `PASS` | No actionable findings remain. |
| `UNRESOLVED` | Some findings remain unresolved. |
| `BLOCKED` | More information or verification is needed to complete the review. |

A stage can complete successfully and still leave findings. Read the reviews
before accepting the changes.

With [`saveFindings: true`](configuration.md#save-security-findings-without-corrections),
security findings and suggested fixes appear in `summary.log` without being
applied. If you see `Security findings unavailable.`, check the warnings and
security log.

## Recover a run

To continue from the last saved stage:

```bash
software-factory resume "/path/to/run-folder"
```

Keep the complete folder, including `summary.log`, `stage_status.txt`, and the
archived `codeReview.txt` and `securityReview.txt` reports when present.
See the [recovery guide](workflow.md#resume-a-failed-or-interrupted-run).

If the final walkthrough fails, default and `continue` runs warn and keep the
earlier results. During `resume`, the failure returns an error so you can retry.

## GitHub summary {#github-pull-request-summaries}

After a successful run with logs, Soft Factory tries to append the summary to
the branch's existing pull request. It uses `gh`, host authentication, and the
`origin` remote. It does not create a PR and works independently of `push` and `pr`.

If you see `Warning: append PR execution summary`, the run's result is
preserved. You can read the local summary in `summary.log`.

Logs include the task and context documents. Keep their contents private and
out of commits.
