# Troubleshooting

## A dependency is missing

Run `software-factory doctor`. It exits with status 1 if `agent-sandbox`,
`git`, or `docker` is missing from `PATH`. Install the missing dependency or
add its directory to `PATH`. Doctor checks executable availability only;
use `docker info` to verify Docker daemon access.

## Configuration cannot be loaded

Run from the project root containing `software-factory.json`. Use the
[documented camelCase properties](configuration.md).

The `agentSandbox` property is required; use `{}` when you need no sandbox
settings. Settings travel through CLI flags; no sandbox configuration JSON is
generated. `apiKey` and `reviewers` in either section are temporarily ignored.
Authenticate the agent on the host. Agent Sandbox reports sandbox validation
failures; see [transport details](configuration.md#sandbox-settings-transport).

## The sandbox branch cannot be found

For `continue`, set `resume.branch` to an existing sandbox branch.
Use `agent-sandbox worktree-list` to list them. In the default workflow,
disabling implementation still requires the `run.branch` worktree to exist.

## A custom skill cannot be found

Put the skill at `.agents/skills/<name>/SKILL.md` or
`.claude/skills/<name>/SKILL.md` relative to `software-factory.json`.
Configure the directory name, rather than a path. Missing custom skills for
enabled stages stop the workflow before implementation.

## Google Docs cannot be loaded

Use a service-account key named `service_account.json` in the project folder.
Share the document or its folder with the account's `client_email` and enable
the Drive API. Folder names must identify a single accessible folder.
See [Google Drive setup](google-drive.md).

## Jira credentials or issue access fail

Set `JIRA_EMAIL` and `JIRA_API_TOKEN` in the environment or local `.env`.
Check the [issue URL](jira.md) and that the account can read the issue.

## A review reports BLOCKED or UNRESOLVED

Read the stage output for unavailable reviewer/fixer subagents, verification
gaps, or remaining findings. Reviews allow up to three rounds.
A successful command exit does not guarantee every finding was resolved.

## A workflow failed or was interrupted

Inspect the stage log and `summary.log` in the printed execution-log directory.
Set `resume.branch` to that run's branch, then use
`software-factory resume <path>` with the invocation directory to recover from
the last saved stage. Keep the task in `run.filePrompt`, or pass `--jira` again
for a Jira task. Recovery needs the original summary, stage status, and any
earlier review evidence needed for risk classification.

If those files are unavailable, use `software-factory continue` to run
implementation and subsequent enabled stages in a new invocation.
See [resuming a run](workflow.md#resume-a-failed-or-interrupted-run) and
[execution logs](reports.md#execution-logs).

## The final walkthrough is missing

Default and `continue` workflows warn if walkthrough generation or saving fails, keeping
the result of earlier stages. During `resume`, a walkthrough failure returns
an error so recovery can be retried. Check whether `reviewChanges` is disabled,
then check terminal warnings and [reports](reports.md). If execution logging
is unavailable, the workflow can proceed but cannot save the final walkthrough.
