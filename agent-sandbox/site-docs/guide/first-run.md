# Your first run

This example creates a branch, asks the agent for a small change, and leaves
the result for you to inspect. Complete [installation](installation.md) and
[authentication](authentication.md) first.

## 1. Start from your project

```bash
cd /path/to/your/repository
git status
```

Choose the starting branch you want the agent to work from. A new worktree
starts from your current commit; uncommitted files in your working copy are
not copied into it. Commit any changes the agent needs before starting.

## 2. Give the agent a  task

```bash
agent-sandbox run -b improve-readme -a codex \
  -q "Add a short getting-started section to README.md using the project's existing setup commands."
```

Replace `codex` with `claude`, `opencode`, or `pi` if that is your configured
agent. Pick a branch name that is not already in use, or omit `-b` to generate one.

The first run may take longer because the tool builds a Docker image and
installs the agents. Every run needs npm access to check the selected agent's
version. Without `--push` or `--pr`, the prompt tells the agent to leave changes
unstaged and uncommitted.

## 3. Find and review the result

```bash
agent-sandbox worktree-list
agent-sandbox worktree-editor -b improve-readme
```

The second command opens the worktree in VS Code. If you use another editor,
open the path shown by `worktree-list`.

## 4. Ask for a follow-up

From your original repository directory:

```bash
agent-sandbox resume -b improve-readme -a codex \
  -q "Make the getting-started section shorter and verify the commands against this repository."
```

`resume` starts another agent invocation in the same worktree, preserving its
files and commits. It does not promise to restore the previous conversation.

Once satisfied, either commit the changes yourself in the worktree and bring
them into your usual Git workflow, or [publish a branch or PR](publishing.md).
Keep the worktree until its changes are safely retained.
