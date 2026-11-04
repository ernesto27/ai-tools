# Troubleshooting

## The app cannot find a CLI

Install `software-factory` and `agent-sandbox` separately. Launch the app from a terminal where both commands work. A desktop launcher or Finder may have a different `PATH`; see [Installation](./installation).

The **Add repository** screen uses `agent-sandbox doctor --json` for tool checks. Run `agent-sandbox doctor` in your terminal to inspect requirements and follow the [Agent Sandbox troubleshooting guide](https://ernesto27.github.io/ai-tools/troubleshooting).

## A folder cannot be added

Choose a local Git repository with **Browse**. The app checks whether the selected folder is a repository. If it is already saved, use its existing sidebar entry. A missing `software-factory.json` can be created from **Config** after adding the repository.

## No branches or runs appear

Branches in the sidebar come from Agent Sandbox's recorded worktrees. Check the registry using the same CLI and repository path:

```bash
agent-sandbox worktree-list --json --repo /path/to/repository
```

A normal Git branch without a recorded Agent Sandbox worktree may not appear. Choose **Refresh** in the branch's Runs screen to reload available run logs. See the [worktree guide](https://ernesto27.github.io/ai-tools/worktrees) for CLI-side management.

## Configuration cannot be loaded or saved

Check that the repository's `software-factory.json` is valid JSON and the file and directory are writable. Read the error shown in **Config** or **New run** before launching again. Use the [CLI configuration guide](https://ernesto27.github.io/ai-tools/soft-factory/configuration) to check field meanings.

## A run fails or is interrupted

Open its detail screen and read the failed stage's log. Fix the reported configuration, authentication, or environment problem, then choose **Resume** for the interrupted run or **Continue** for another implementation pass on the worktree. Missing output files may simply mean that a stage has not run or was disabled.

## Stop is unavailable after reopening

Live process controls belong to runs launched in the current desktop session. Reopening a recorded run provides its saved output, without restoring the previous session's Stop control. Closing the app alone does not stop a running workflow.
