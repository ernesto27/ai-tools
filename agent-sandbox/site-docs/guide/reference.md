# Commands and flags

## Commands

| Command | Purpose |
| --- | --- |
| `run` | Start a new task with plain output. |
| `resume` | Continue a recorded task; requires `-b`. |
| `run-tui` | Start a task in the terminal UI. |
| `resume-tui` | Continue a task in the terminal UI; requires `-b`. |
| `doctor` | Check host dependency availability in `PATH`. |
| `update` | Install the latest release binary. |
| `worktree-list` | List registered worktrees for the current repository. |
| `worktree-editor -b <branch>` | Open a recorded worktree in VS Code. |
| `worktree-delete -b <branch> [--force]` | Remove a worktree and its local branch. |
| `worktree-delete-all [--yes]` | Remove all registered tasks for the current repository, including uncommitted work. |

## Execution flags

These flags apply to `run`, `resume`, and their TUI equivalents:

| Flag | Meaning |
| --- | --- |
| `-b`, `--branch` | Task branch. Generated for new tasks if omitted; required for resumes. |
| `-a`, `--agent` | `codex`, `claude`, `opencode`, or `pi`; required unless supplied by JSON. |
| `-m`, `--model` | Override the model selected for the agent. |
| `-q`, `--query` | Task instructions. |
| `-f`, `--filePrompt` | Read task instructions from a file, instead of `-q`. |
| `-i`, `--baseImage` | Build the sandbox image from a compatible external base. |
| `--image` | Attach an image to Codex or Claude Code; repeat for multiple files. |
| `-p`, `--push` | Ask the agent to commit, then push from the host. |
| `--pr` | Commit and publish from the host. Initial runs create or reuse a PR; resumes require an open PR and append a session checklist. |
| `-c`, `--commitMessage` | Exact commit message to use with `--push` or `--pr`. |
| `--hn` | Share the host's network with the container. |

Exactly one prompt source must be supplied through flags or JSON.
`apiKey` and `reviewers` are [JSON-only settings](configuration.md).

## Help and version

```bash
agent-sandbox --help
agent-sandbox run --help
agent-sandbox --version
```

## Exit status

| Status | Meaning |
| --- | --- |
| `0` | Successful execution. |
| `1` | General tool error. |
| `2` | Invalid command usage. |
| `130` | Execution cancelled. |
| Agent's nonzero status | Agent failure, propagated by the CLI. |

Agent status values can overlap these numbers; read the accompanying error
message to understand the cause. In publishing modes, a failed agent is not
published.
