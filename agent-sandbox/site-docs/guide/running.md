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

| Key | Action |
| --- | --- |
| Tab | Change focus |
| Arrow keys, Page Up, Page Down | Scroll |
| `q` or Ctrl+C | Cancel execution and wait for cleanup |


