# Workflow

Run commands from your project folder, where `software-factory.json` lives.

| What you want | Command |
| --- | --- |
| Start a new task | `software-factory` |
| Work further on an existing task | `software-factory continue` |
| Recover a failed or interrupted run | `software-factory resume "/path/to/run-folder"` |

## Start a task

```bash
software-factory
```

Soft Factory loads your task and supporting documents, then runs five stages:

1. Implement the task in a sandbox worktree.
2. Review code and apply corrections.
3. Review security and apply corrections.
4. Classify risk as **LOW**, **MEDIUM**, **HIGH**, or **UNKNOWN**.
5. Write a read-only walkthrough of the changes.

Code and security reviews allow up to three rounds of review, corrections,
and verification. With [saveFindings enabled](configuration.md#save-security-findings-without-corrections),
security review makes one pass and suggests fixes without applying them.

Read the [reports](reports.md) and inspect the changes before accepting them.
You can [choose agents and models](configuration.md#stage-agents-and-models)
or [skip stages](configuration.md#disabling-stages) in your configuration.

## Continue an existing task

```bash
software-factory continue
```

This runs implementation and the later enabled stages again in an existing
sandbox worktree. It creates a new log folder and prints its path.

The default command uses `run.branch`; `continue` uses `resume.branch`.
Both fall back to the shared `agentSandbox.branch`. Use the branch from your
original task. To list available worktrees:

```bash
agent-sandbox worktree-list
```

Keep the task in `run.filePrompt`. For a Jira task, pass `--jira` again:

```bash
software-factory continue --jira 'https://your-company.atlassian.net/browse/ENG-123'
```

## Resume a failed or interrupted run

Copy the log folder path printed by the failed run:

```bash
software-factory resume "/path/to/run-folder"
```

Run this from the original project folder. Set `resume.branch` (or the shared
`branch`) to that run's sandbox branch. Recovery reads your current
configuration, supporting documents, and task from `run.filePrompt`.
For Jira tasks, pass `--jira` again.

Recovery starts from the last saved stage:

- A successful stage advances to the next one.
- A failed or interrupted stage is retried.
- An empty status file starts at implementation.
- A successful final walkthrough means no work remains.

Recovery reuses the same logs and summary. Keep `summary.log`,
`stage_status.txt`, and archived review reports together. Missing status or
required review evidence prevents recovery. Use `continue` to start a fresh
run in the existing worktree if those files are unavailable.

See [execution logs](reports.md#execution-logs) for file locations and details.

## When a stage fails

An error before the final walkthrough stops the workflow. A walkthrough error
prints a warning for default and `continue` runs; during recovery, it returns
an error so you can retry.

Review verdicts such as **UNRESOLVED** or **BLOCKED** can still appear in a
successful run. Check the reports for findings and verification gaps.
