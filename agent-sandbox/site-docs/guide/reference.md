# Commands and flags

## Commands

| Command | Purpose |
| --- | --- |
| `run` | Start a new task with plain output. |
| `resume` | Continue a recorded task; requires `-b`. |
| `run-tui` | Start a task in the terminal UI. |
| `resume-tui` | Continue a task in the terminal UI; requires `-b`. |
| `doctor [--json]` | Check host dependency availability in `PATH`; optionally return JSON. |
| `update` | Install the latest release binary. |
| `worktree-list` | List registered worktrees for the current repository. |
| `worktree-editor -b <branch>` | Open a recorded worktree in VS Code. |
| `worktree-delete -b <branch> [--force]` | Remove a worktree and its local branch. |
| `worktree-delete-all [--yes]` | Remove all registered tasks for the current repository, including uncommitted work. |

Codex and Claude Code show a [compact audit feed](running.md#read-the-compact-audit-feed)
in all four execution commands. Plain commands support redirection; TUI commands
require interactive terminal input and output.

## Doctor output

`doctor --json` writes one JSON array to stdout with exactly `name` (string)
and `installed` (boolean) per dependency, ordered `git`, `docker`, `gh`, `code`.
It includes missing tools with `installed: false` and exits successfully even
when tools are missing. It checks PATH only, without running tools or checking
service health. Output and usage errors retain the normal CLI error handling.
Without `--json`, or with `--json=false`, output is the colored text table.

## Execution flags

These flags apply to `run`, `resume`, and their TUI equivalents:

| Flag | Meaning |
| --- | --- |
| `-b`, `--branch` | Task branch. Generated for new tasks if omitted; required for resumes. |
| `-a`, `--agent` | `codex`, `claude`, `opencode`, or `pi`; required unless supplied by JSON. |
| `--apiKey <key>` | Provider API key for Codex or Claude Code; overrides the section's JSON key. `--apiKey=""` selects host authentication. No shorthand. |
| `--reviewers <username>` | Repeat for each GitHub username. Replaces `run.reviewers`; `--reviewers=""` alone clears it. Validated but ignored on resume. No shorthand. |
| `-m`, `--model` | Override the model selected for the agent. |
| `--effort` | Agent effort. Defaults to `high`; the agent checks supported levels. |
| `-q`, `--query` | Task instructions. |
| `-f`, `--filePrompt` | Read task instructions from a file, instead of `-q`. |
| `-i`, `--baseImage` | Build the sandbox image from a compatible external base. |
| `--image` | Attach an image to Codex or Claude Code; repeat for multiple files. |
| `--file` | Mount a regular file read-only for any agent; repeat for multiple files. See [reference files](environment.md#mount-reference-files). |
| `-p`, `--push` | Ask the agent to commit, then push from the host. |
| `--pr` | Commit and publish from the host. Initial runs create or reuse a PR; resumes require an open PR and append a session checklist. |
| `-c`, `--commitMessage` | Exact commit message to use with `--push` or `--pr`. |
| `--hn` | Share the host's network with the container. |

Exactly one prompt source must be supplied through flags or JSON.
Flags override [configuration](configuration.md), including explicit empty values.
Use camelCase `--apiKey`; `--api-key` is unsupported. Keys use the existing
[authentication modes](authentication.md) and are not added to prompts, worktree
records, help, or transcripts. A `--apiKey` value is visible to other local
users and processes through the process list for the whole run, and a literal
value may be saved in shell history; on shared hosts, prefer the section's JSON
`apiKey`, and keep `agent-sandbox.json` readable only by you (for example,
`chmod 600 agent-sandbox.json`) and out of Git (for example, listed in
`.gitignore`). Reviewer usernames are trimmed, validated, and
deduplicated case-insensitively; comma lists and mixed empty entries are usage
errors. See [reviewer rules](publishing.md#request-reviewers).

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
