# Workflow

1. Load the local task or Jira issue and supporting documents.
2. Implement in the sandbox worktree (default and `continue`).
3. Review code and apply corrections.
4. Review security and apply corrections.
5. Classify the final risk as **LOW**, **MEDIUM**, **HIGH**, or **UNKNOWN**.
6. Explain the final changes (default and `continue`).

[Reports](reports.md) document results and verification gaps.

Choose agents and models independently for each stage with
[`stages` in configuration](configuration.md#stage-agents-and-models).
Omitted settings inherit the defaults from `software-factory.json: agentSandbox`.

Each code or security review follows **review → corrections when needed → verification**,
with up to three rounds. Failures before the final walkthrough stop the
workflow. A walkthrough failure warns and does not change the exit status
for default and `continue` runs; during `resume`, it returns an error.

## Continue an existing task

```bash
software-factory continue
```

This runs implementation and subsequent stages against the existing sandbox
worktree selected by `resume.branch`. The default command selects `run.branch`.
Every stage uses the branch selected for that command, even when the two
configured branches differ. List available worktrees with
`agent-sandbox worktree-list`.

`continue` starts a new invocation under
`~/.config/software-factory/logs/<project-folder>-<id>/<sanitized-branch>/`,
named `continue-YYYY-MM-DD_HH-MM-SS` in local time. Default invocations use
`run-YYYY-MM-DD_HH-MM-SS`. The project name comes from the directory where
Factory is invoked; its path ID separates same-named projects. A continue
creates missing folders and keeps its files separate from earlier invocations.
See [execution logs](reports.md#execution-logs) for identity and collision
details. It runs implementation again rather than resuming at the last failed stage.

Keep the original task in `run.filePrompt`, or supply `--jira` again when
continuing a Jira task. Supporting documents and enabled stages still apply.
See [configuration](configuration.md#disabling-stages) to skip selected stages.

## Resume a failed or interrupted run

To recover from the last saved stage, pass the invocation folder printed by
that run. The ID below is illustrative; copy the path printed by your run:

```bash
software-factory resume "$HOME/.config/software-factory/logs/soft-factory-a1b2c3d4e5f6/fix-login/run-2026-10-26_17-47-23"
```

Run this from the original project directory, with `resume.branch` set to the
sandbox branch used by that run. The path selects the logs; it does not restore
the branch or configuration. Soft Factory reads the current configuration,
supporting documents, and task from `run.filePrompt`. For a Jira task, pass
`--jira` again:

```bash
software-factory resume "$HOME/.config/software-factory/logs/soft-factory-a1b2c3d4e5f6/eng-123/run-2026-10-26_17-47-23" --jira 'https://your-company.atlassian.net/browse/ENG-123'
```

Existing project-local log directories are still valid recovery inputs.
They are not moved automatically. For example:

```bash
software-factory resume logs/fix-login/run-2026-10-08_19-38-48.007046183
```

The last entry in `stage_status.txt` determines where recovery starts. A
`SUCCEEDED` entry advances to the next stage; a `FAILED` or `INTERRUPTED`
entry retries that stage. An empty status file starts at implementation.
If the final walkthrough is already recorded as successful, the command
returns without running stages. Current `stages.<name>.disabled` settings still apply.

Unlike `continue`, `resume` reuses the same invocation directory and appends
to its stage logs and summary. Keep `summary.log`, `stage_status.txt`, and the
archived `codeReview.txt` and `securityReview.txt` reports together. When earlier
reviews are skipped during recovery, their archived evidence is restored for
risk classification. Missing required evidence stops recovery.

A missing status file prevents recovery, including for older log folders that
predate it. Use `continue` to start a fresh invocation. See [reports](reports.md#execution-logs)
for the saved files.
