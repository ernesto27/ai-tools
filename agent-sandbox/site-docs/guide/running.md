# Prompts and terminal UI

## Give a clear task

A useful prompt names the desired behavior, relevant constraints, and how to
verify the result:

```bash
agent-sandbox run -b fix-login -a codex \
  -q "Fix the login redirect loop. Keep the existing API unchanged, add a regression test, and run the relevant tests."
```

The tool adds instructions for handling Git and publication. It also tells
the agent to make decisions without asking questions, so include information
it needs up front.

## Use a prompt file

For longer tasks, put the instructions in a file:

```bash
agent-sandbox run -a claude -f prompt.md
```

Provide exactly one prompt source: `-q`, `-f`, or the corresponding JSON
setting. Positional prompts and combining `-q` with `-f` are rejected.
An explicit CLI prompt source replaces a prompt source from JSON.

## Choose an agent and model

```bash
agent-sandbox run -a claude -m sonnet -q "add a regression test for the login redirect"
agent-sandbox run -a opencode -q "update the empty-state copy"
agent-sandbox run -a pi -q "inspect the test failures and fix their cause"
```

`--model` is passed to the chosen agent. The current defaults are
`gpt-6.1-sol` for Codex and `claude-opus-5-5` for Claude Code. opencode and pi
use their own configuration when you omit it. Model availability depends on
your provider account.

## Choose effort

All agents default to `high`. Override it for a single task:

```bash
agent-sandbox run -a codex --effort low -q "inspect this repository"
```

You can also set a shared `effort` in [JSON configuration](configuration.md#set-effort-globally).
The value goes to Codex's `model_reasoning_effort`, Claude's `--effort`,
opencode's `--variant`, or pi's `--thinking`. Supported values depend on the
agent and model; unsupported values are reported by the agent with its normal
exit status.

## Read the compact audit feed

Codex and Claude Code automatically show a compact audit feed in `run`,
`resume`, `run-tui`, and `resume-tui`. It groups commands, tool activity, file
changes, and agent messages into timestamped entries. No extra flag is needed.
opencode and pi keep their existing output format.

For example, a Codex task might show:

```text
codex / fix-login
---------------------------------------------
10:32:02  > COMMAND
            rg redirect auth/

10:32:02  + RESULT
            rg redirect auth/
            exit 0 / 0.1s

10:32:04  > TOOL
            Context7 > query-docs

10:32:05  + TOOL
            Context7 > query-docs
            completed / 1.2s

10:32:07  * FILE
            auth/login.go
            patch applied

10:32:12  > COMMAND
            go test ./...

10:32:13  ! FAILED
            go test ./...
            exit 1 / 0.8s
            TestLoginRedirect
            Expected /home
            Received /login

---------------------------------------------
```

The header identifies the agent and actual worktree branch, including a
branch generated when you omit `-b`.

| Label | Meaning |
| --- | --- |
| `> COMMAND` | A command started. |
| `+ RESULT` | A command completed. Codex includes its numeric exit code when supplied. |
| `> TOOL` / `+ TOOL` | A tool started or completed. Claude file-edit tools also begin with `> TOOL`. |
| `* FILE` | A reported file change completed. |
| `! FAILED` | A command, tool, or agent reported a failure. Available failure output follows. |
| `AGENT` / `THINKING` | Agent text or reasoning exposed by the agent's event stream. |
| `+ SEARCH` / `PLAN` | Search activity or a task checklist exposed by Codex. |
| `? EVENT` | A notice, such as a retry or an unsupported event type. |
| `! OUTPUT` | Output could not be decoded; the diagnostic and original line follow. |

Successful command and tool output is hidden to keep the feed compact; failure
output is included when available. A file entry reports the path and change
status, rather than displaying a full diff. Review the worktree for the actual
changes.

Timestamps use the host's local time. Durations measure the interval between
receiving matching start and completion events on the host; they are omitted
when no matching start was received. Claude command results show a status
such as `completed` or `failed`, rather than inferring an exit code from text.
Claude's token-level updates are omitted, and a final result matching the
last assistant text is suppressed.

A closing separator marks the end of the feed, including cancellation. Check
the command's exit status and publication messages to determine the task's
outcome; individual failed commands can be followed by a successful retry.

## Use the terminal UI

Use `run-tui` or `resume-tui` for a terminal view with execution output and
details:

```bash
agent-sandbox run-tui -b fix-login -a codex -q "fix the login redirect loop"
agent-sandbox resume-tui -b fix-login -a codex -q "add a regression test"
```

These commands use the same flags and `run`/`resume` configuration as their
plain-output equivalents. They require an interactive terminal for both
input and output; use `run` or `resume` in scripts or when redirecting output.

The output panel shows the same [compact audit feed](#read-the-compact-audit-feed)
for Codex and Claude Code, with colors for activity, notices, and failures.
The TUI transcript records that formatted text. Logs normally live under
`~/.config/agent-sandbox/logs/`, honoring `XDG_CONFIG_HOME`; they contain the
formatted feed rather than the original agent JSON event stream.

| Key | Action |
| --- | --- |
| Tab | Change focus |
| Arrow keys, Page Up, Page Down | Scroll |
| `q` or Ctrl+C | Cancel execution and wait for cleanup |


