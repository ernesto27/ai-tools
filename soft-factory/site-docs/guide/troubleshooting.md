# Troubleshooting

## A dependency is missing

Run `software-factory doctor`. It exits with status 1 if `agent-sandbox`,
`git`, or `docker` is missing from `PATH`. Install the missing dependency or
add its directory to `PATH`. Doctor checks executable availability only;
use `docker info` to verify Docker daemon access.

## Configuration cannot be loaded

Run from the project folder containing `software-factory.json` and
`agent-sandbox.json`. Rename older `config.json` files and use the
[documented camelCase properties](configuration.md).

## The sandbox branch cannot be found

For `review` or `continue`, set `resume.branch` to an existing sandbox branch.
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

## The final walkthrough is missing

The standalone `review` command does not run the walkthrough. Default and
`continue` workflows warn if walkthrough generation or saving fails, keeping
the result of earlier stages. Check terminal warnings and [reports](reports.md).
