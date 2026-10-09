# Reports

Reports appear in the terminal. Default and `continue` workflows save code
review, security review, and risk classification output in their stage logs,
plus a run summary and the final walkthrough.

| Command | Saved output |
| --- | --- |
| `software-factory` | `logs/<sanitized-branch>/run-<UTC-timestamp>/summary.log`, stage `.log` files, and `05-review-changes.md` when the walkthrough runs successfully. |
| `software-factory continue` | The same files in `logs/<sanitized-branch>/continue-<UTC-timestamp>/` for `resume.branch`. |
| `software-factory review` | Risk-classification output in `docs/risk-classification-<datetime>.md` when that stage runs; no execution-log directory or final walkthrough. |

Code and security reviews also write temporary reports in the sandbox worktree
for risk classification to read. These are normally removed after risk
classification, or when that stage is disabled. The standalone `review`
command does not archive code and security review output, so read those
findings in the terminal.

The `05-review-changes.md` walkthrough shows the changed code lines,
file-by-file explanations, findings, and flow diagrams. Terminal setup and
command output from that final stage are shown live but are not saved in the
Markdown file. The agent writes the report in its sandbox worktree; Soft Factory
copies it to the run logs and removes the worktree copy.
Findings in this report are informational. If the report cannot be generated
or saved, Soft Factory prints a warning and keeps the result of the earlier
stages.

Review reports include their status (**PASS**, **UNRESOLVED**, or **BLOCKED**), corrections, remaining findings, and verification gaps. Read the reports before accepting the changes: a successful command exit does not guarantee that every finding was resolved.

Local configuration, credentials, task files, and generated reports are ignored by Git. Keep sensitive content out of commits.

## Execution logs

Default workflows save logs in `logs/<sanitized-branch>/run-<UTC-timestamp>/`,
using `run.branch`. `continue` uses `resume.branch` and creates
`logs/<sanitized-branch>/continue-<UTC-timestamp>/`. When commands select the
same branch, their invocations share a branch folder. The parent is created
when needed, including for a continue without earlier logs. The log root is
relative to the CLI working directory. Existing flat log folders stay untouched.

ASCII letters, digits, `.`, `_`, and `-` remain in branch folder names; all
other characters become `-`. For example, `feature/logs` maps to the single
folder `feature-logs`. The timestamp is UTC with nanosecond precision, using
`2006-01-02_15-04-05.000000000`. An existing child name gets a suffix of `-2`,
`-3`, and so on, keeping every invocation's files separate.

```text
logs/
└── feature-logs/
    ├── run-2026-10-08_19-38-48.007046183/
    │   ├── summary.log
    │   ├── 01-implementation.log
    │   └── 05-review-changes.md
    ├── continue-2026-10-08_20-09-49.425026374/
    │   ├── summary.log
    │   └── ...
    └── continue-2026-10-08_21-15-30.123456789/
        ├── summary.log
        └── ...
```

The CLI prints the invocation child path, for example:

```text
Execution logs: logs/feature-logs/continue-2026-10-08_20-09-49.425026374
```

The first four stages have position-prefixed files (`01-implementation.log`,
`02-code-review.log`, `03-security-review.log`, and
`04-risk-classification.log`); another attempt at a stage uses a numbered file
such as `01-implementation-2.log`. Each stage file includes the run details
and that stage's result. A stage that never starts has no file. The CLI prints
the directory path when the run starts and continues to show stage output
live. The final stage writes `05-review-changes.md` as the only Markdown file
in the run directory. The code review, security review, and risk classification
output stays in their stage logs. The standalone `review` command does not run
the final stage or create these logs.

Start with `summary.log` for a quick view of the first four stages that ran.
Each entry includes the execution status (`SUCCEEDED`, `FAILED`, or
`INTERRUPTED`), duration, a short summary of changes made during that stage,
and any execution error. If the agent's change summary cannot be collected,
the entry says `Change summary unavailable.` Disabled stages are listed as
skipped without an executed-stage entry. The final walkthrough has its own
Markdown file and no execution-summary entry.

Execution statuses describe whether the stage ran successfully. They are
separate from the review verdicts (`PASS`, `UNRESOLVED`, or `BLOCKED`); read the
review output to assess remaining findings.

Each default or `continue` invocation creates a new directory, including after
a failure.
Disabled stages keep the usual filename positions, so disabling code review
still leaves security review at `03-security-review.log`.

If log creation or writing fails, Soft Factory warns and keeps running.
Logs may be missing or incomplete; without a run log directory, the final
walkthrough cannot be saved. Stage logs include prompts and supporting context,
so keep the generated output private and out of commits.
