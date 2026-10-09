# Troubleshooting

## The shell cannot find `agent-sandbox`

The installer uses `~/.local/bin` by default. Add it to `PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
agent-sandbox --version
```

Add the export to your shell profile to keep it across terminal sessions.

## Docker is installed, but the run fails

```bash
agent-sandbox doctor
docker info
```

`doctor` checks executable availability only. If `docker info` fails, start
Docker or correct your user's access using your Docker installation's guidance.

## Authentication paths are missing

Run and authenticate your selected agent on the host first. Check every path
listed on the [authentication page](authentication.md), including
`~/.claude.json` for Claude Code. Do not create empty files to bypass the check.
For Codex or Claude, API-key authentication is another supported option.

## The first run is slow, or image preparation fails

The first run may build an image. Every run checks npm, and an agent update
can trigger a rebuild. Check network access, the Docker build output, and the
base image's package repositories. If your project needs Go or another runtime,
choose an [external base image](environment.md).

## The command rejects my prompt or JSON

Use `-q "task"` or `-f prompt.md`, not positional text, and do not pass both.
Check that your JSON uses supported fields and types, with no `null` values.
Both `run` and `resume` sections are validated.

Run from the directory containing your intended `agent-sandbox.json`; config
is not searched in parent directories. If JSON enables publication, use
`--push=false --pr=false` for a run that should leave changes uncommitted.

## The branch already exists

Use a different branch name for a new task, or omit `-b`. For a recorded task,
use `resume` instead of `run`:

```bash
agent-sandbox worktree-list
agent-sandbox resume -b fix-login -a codex -q "continue the fix"
```

## A resume cannot find my task, or says it is busy

Run from the same repository and use the branch listed by `worktree-list`.
Only registered sandbox worktrees can be resumed. If the task is busy, wait
for its running session to finish or cancel that session and wait for cleanup.

## The terminal UI fails or disappears

Use an interactive terminal for `run-tui` and `resume-tui`. For redirection
or scripts, use `run` or `resume`. Check the log path in the error message;
TUI transcripts normally live under `~/.config/agent-sandbox/logs/`, honoring
`XDG_CONFIG_HOME`.

## Push or PR creation fails

Check the agent's exit status and inspect the worktree first. Publishing
requires committed changes and a clean worktree; it is skipped after agent
failure. Ask the agent to finish with `resume --push` or `resume --pr`.

For PRs, check `gh auth status`, origin URLs, and push permissions. The recorded
base branch must exist on `origin`; detached starts and legacy records without
a base cannot publish PRs. No differences against the base means no PR is created.

If an error says the push succeeded but PR creation failed, the remote branch
is already published. Correct the reported problem and retry `resume --pr`.
Reviewer requests apply to new PRs from `run`, so resolve missing reviewer
requests separately on GitHub if creation partially succeeded.

## I cannot delete a worktree

Run from your original repository, outside the worktree being removed. Wait
for any session using it to finish. Review or save uncommitted changes before
trying again. Use `--force` only when you intend to discard them; deletion
also removes the local branch.

## Report an issue

Include the CLI version, selected agent, command with secrets removed, exit
status, and relevant error output in a
[GitHub issue](https://github.com/ernesto27/ai-tools/issues).
Review logs for keys, private code, and account information before sharing.
