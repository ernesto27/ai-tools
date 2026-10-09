# Reports

Reports appear in the terminal. The default workflow saves the code review,
security review, and risk classification output in their `.log` files, and
only the final walkthrough as `logs/<run>/05-review-changes.md`.
The standalone `review` command saves its three reports as timestamped
Markdown files in `docs/` because it has no run log directory.

The default workflow's `05-review-changes.md` shows the changed code lines,
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

The default workflow saves logs in `logs/<branch>-<UTC-datetime>/`, using the
sandbox worktree branch from `agent-sandbox.json` (`run.branch`). Branch slashes
become hyphens, so `feature/logs` appears as `feature-logs`. The
first four stages have position-prefixed files (`01-implementation.log`,
`02-code-review.log`, `03-security-review.log`, and
`04-risk-classification.log`); another attempt at a stage uses a numbered file
such as `01-implementation-2.log`. Each stage file includes the run details
and that stage's result. A stage that never starts has no file. The CLI prints
the directory path when the run starts and continues to show stage output
live. The final stage writes `05-review-changes.md` as the only Markdown file
in the default run directory. The code review, security review, and risk classification
output stays in their stage logs. The standalone `review` command does not run
the final stage or create these logs; its three reports remain in `docs/`.
