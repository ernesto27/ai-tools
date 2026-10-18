# Your first run

Run Soft Factory from the Git project you want the agent to change.
Install [Soft Factory and Agent Sandbox](installation.md) first and authenticate your selected agent.

## Write a task

Create `task.txt` with a concrete request, such as:

```text
Fix the login redirect loop and verify the change with the existing tests.
```

## Configure the workflow

Create `software-factory.json`:

```json
{
  "documents": [],
  "agentSandbox": {
    "agent": "codex",
    "branch": "fix-login",
    "run": {"filePrompt": "task.txt"},
    "resume": {"push": false},
    "pr": false
  }
}
```

Breaking change: `agent-sandbox.json` is no longer consumed by Factory. If you
already have one, manually move its entire object into `agentSandbox`, preserving
your existing Factory settings and stage assignments.

The shared branch applies to both sections. Choose a new branch for each new task.
See [configuration](configuration.md) for agent options and supporting documents.

## Run the task

```bash
software-factory doctor
software-factory
```

Soft Factory implements the task, reviews code and security, classifies risk,
and writes a walkthrough. Reviews can correct files in the sandbox worktree.
Read the [reports](reports.md) and inspect the resulting changes before accepting them.

To work further on the same branch, use [continue](workflow.md#continue-an-existing-task).
