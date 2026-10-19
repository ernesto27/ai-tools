# Reports

Reports appear in the terminal. Default and `continue` workflows save code
review, security review, and risk classification output in their stage logs,
plus a run summary and the final walkthrough.

| Command | Saved output |
| --- | --- |
| `software-factory` | `logs/<sanitized-branch>/run-<UTC-timestamp>/summary.log`, stage `.log` files, and `05-reviewChanges.md` when the walkthrough runs successfully. |
| `software-factory continue` | The same files in `logs/<sanitized-branch>/continue-<UTC-timestamp>/` for `resume.branch`. |
| `software-factory resume <path>` | Appends stage logs, progress, and summary entries to the supplied invocation folder; saves the walkthrough there. |
| `software-factory review` | By default, risk-classification output in `docs/risk-classification-<datetime>.md` when that stage runs. When security review is enabled with `saveFindings: true`, execution logs and `summary.log` in `logs/<sanitized-branch>/review-<UTC-timestamp>/`. No final walkthrough. |

Code and security reviews also write temporary reports in the sandbox worktree
for risk classification to read. These are normally removed after risk
classification, or when that stage is disabled. By default, the standalone `review`
command does not archive code and security review output, so read those
findings in the terminal. Logged workflows also archive successful code and
security review evidence as `codeReview.txt` and `securityReview.txt` in the
invocation folder so `resume` can reuse it after worktree cleanup.

The `05-reviewChanges.md` walkthrough shows the changed code lines,
file-by-file explanations, findings, and flow diagrams. Terminal setup and
command output from that final stage are shown live but are not saved in the
Markdown file. The agent writes the report in its sandbox worktree; Soft Factory
copies it to the run logs and removes the worktree copy.
Findings in this report are informational. If the report cannot be generated
or saved, Soft Factory prints a warning and keeps the result of the earlier
stages. During `resume`, a walkthrough failure returns an error.

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
    │   └── 05-reviewChanges.md
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

Stages have position-prefixed log files (`01-implementation.log`,
`02-codeReview.log`, `03-securityReview.log`, `04-riskClassification.log`, and
`05-reviewChanges.log`); another attempt at a stage uses a numbered file
such as `01-implementation-2.log`. Each stage file includes the run details
and that stage's result. A stage that never starts has no file. The CLI prints
the directory path when the run starts and continues to show stage output
live. The final stage writes `05-reviewChanges.md` as the only Markdown file
in the run directory. The code review, security review, and risk classification
output stays in their stage logs. The standalone `review` command does not run
the final stage. It creates execution logs only when security review is enabled
with `stages.securityReview.saveFindings: true`.

The first four stage logs identify the configured agent and model and where
each value came from. A [stage override](configuration.md#stage-agents-and-models) names its
Factory setting, for example
`software-factory.json: stages.codeReview.model`; inherited values name their
setting in `software-factory.json: agentSandbox`. If no model is configured, the log says
`unavailable (no configured model)`.

Start with `summary.log` for a quick view of the stages that ran.
Each entry includes the execution status (`SUCCEEDED`, `FAILED`, or
`INTERRUPTED`), the recorded agent and model, duration, a short summary
of changes made during that stage,
and any execution error. If the agent's change summary cannot be collected,
the entry says `Change summary unavailable.` Disabled stages are listed as
skipped without an executed-stage entry. The final walkthrough also has an
execution-summary entry and stage log; its report is saved separately as Markdown. Its agent output remains live-only,
and its agent, model, and change summary may be unavailable.

`Agent:` and `Model:` appear between `Status:` and `Duration:`, with configured
names only. Each field is resolved independently: a stage override in
`software-factory.json`, then the `run` or `resume` section in
`software-factory.json: agentSandbox`, then its shared value. An explicitly empty sandbox section
value clears the shared value. Unset summary values are left empty; agent
defaults are not inferred from output. Each sandbox attempt replaces both
fields, and subprocess output cannot change them. Detailed logs retain
configuration provenance. Executed attempts append their own summary entries.

```text
1. IMPLEMENTATION
   Status:    SUCCEEDED
   Agent:     codex
   Model:     gpt-5
   Duration:  20 sec
   Changes:   Created hello.txt containing Hello, world!
```

Execution statuses describe whether the stage ran successfully. They are
separate from the review verdicts (`PASS`, `UNRESOLVED`, or `BLOCKED`); read the
review output to assess remaining findings.

With `stages.securityReview.saveFindings: true`, the security entry includes
full actionable findings and suggested fixes directly in `summary.log`, plus
checks and verification gaps. Each finding includes priority, affected location
when available, trigger, impact, and a suggested fix left unapplied. The ordinary
change summary separately says `No code changes made (saveFindings enabled).`
There is no two-line limit on findings. Reports must omit secret values.

A `PASS` verdict explicitly says `No actionable findings.` `UNRESOLVED` means
findings were intentionally left unapplied. `BLOCKED` identifies insufficient
review evidence or verification. A successful execution can have either of the
latter verdicts. Missing, unreadable, malformed, or incomplete final reports say
`Security findings unavailable.` and produce a visible warning; they never imply
a clean review. Actual execution errors remain in the same entry.

Factory collects a dedicated final report, rather than parsing terminal output
or echoed prompts. The detailed security log and latest `securityReview.txt`
archive remain available; reporting mode also keeps a uniquely named
`.soft-factory-stage-changes-<id>-review.txt` archive in the invocation directory
for each collected attempt. Risk classification receives the unapplied findings
as evidence and must not describe them as fixed.

Each executed attempt gets its own appended summary entry and fresh report.
Logged standalone review invocations also save `command.txt` so recovery keeps
implementation and the final walkthrough out of that workflow.
Resume preserves historical entries; an already successful security stage that
is skipped during recovery gets no new entry. Collection or summary-write
failures warn without enabling corrections or commits. Collected archives are
retained even if the summary cannot be saved; if archiving also fails, the
worktree report is retained when summary saving fails.

Each default or `continue` invocation creates a new directory, including after
a failure. `resume <path>` reopens an existing invocation instead: it appends
to the same stage log files and adds a `RESUMED` marker and new numbered
entries to `summary.log`.
Disabled stages keep the usual filename positions, so disabling code review
still leaves security review at `03-securityReview.log`.

Each completed stage attempt appends a line such as
`codeReview,SUCCEEDED` or `securityReview,FAILED` to `stage_status.txt`.
This includes the final walkthrough; disabled stages have no status entry.
[Recovery](workflow.md#resume-a-failed-or-interrupted-run) uses the last saved
entry, so a process killed before saving its result may leave only the previous
stage's status.

If log creation or writing fails in a default or `continue` run, Soft Factory
warns and keeps running. `resume` requires its existing status file and summary;
failure to open recovery logs stops that command.
Logs may be missing or incomplete; without a run log directory, the final
walkthrough cannot be saved. Stage logs include prompts and supporting context,
so keep the generated output private and out of commits.
