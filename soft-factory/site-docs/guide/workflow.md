# Workflow

1. Load the local task or Jira issue and supporting documents.
2. Implement in the sandbox worktree (default and `continue`).
3. Review code and apply corrections.
4. Review security and apply corrections.
5. Classify the final risk as **LOW**, **MEDIUM**, **HIGH**, or **UNKNOWN**.
6. Explain the final changes (default and `continue`).

The standalone `review` command starts at code review and finishes at risk
classification. [Reports](reports.md) document results and verification gaps.

Each code or security review follows **review → corrections when needed → verification**, with up to three rounds. Failures before the final walkthrough stop the workflow. A walkthrough failure warns and does not change the exit status.

## Review existing changes

Skip implementation and review the changes in the sandbox worktree selected by `resume.branch`:

```bash
software-factory review
```

This command can modify files to correct review findings.

## Continue an existing task

```bash
software-factory continue
```

This runs implementation and subsequent stages against the existing sandbox
worktree selected by `resume.branch`. The default command selects `run.branch`.
Every stage uses the branch selected for that command, even when the two
configured branches differ. List available worktrees with
`agent-sandbox worktree-list`.

`continue` starts a new workflow invocation in
`logs/<sanitized-branch>/continue-<UTC-timestamp>/`. Default invocations use a
`run-<UTC-timestamp>` child under the same branch folder when `run.branch` and
`resume.branch` select the same branch. A continue creates the branch folder
if needed and keeps its files separate from all earlier invocations. See
[execution logs](reports.md#execution-logs) for examples. It runs implementation
again rather than resuming at the last failed stage. To run
only reviews and risk classification, use `software-factory review` instead.

Keep the original task in `run.file-prompt`, or supply `--jira` again when
continuing a Jira task. Supporting documents and enabled stages still apply.
See [configuration](configuration.md#disabling-stages) to skip selected stages.
