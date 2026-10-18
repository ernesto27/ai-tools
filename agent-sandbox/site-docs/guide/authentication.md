# Authentication

Choose either your agent's existing host session or, for Codex and Claude
Code, an API key. `agent-sandbox` does not provide model access itself; you
use your own provider account.

## Use your existing host session

Install and authenticate the selected agent on your host first, following
that agent's login instructions. Then select it with `-a`:

```bash
agent-sandbox run -a claude -q "inspect this repository and summarize its structure"
```

The tool validates and mounts these existing paths:

| Agent | Required host paths |
| --- | --- |
| `codex` | `~/.codex/` |
| `claude` | `~/.claude/` and `~/.claude.json` |
| `opencode` | `~/.config/opencode/`, `~/.local/share/opencode/`, `~/.local/state/opencode/` |
| `pi` | `~/.pi/agent/` |

Missing paths cause an error; the tool does not create them for you.
opencode and pi support only this mode.

## Use an API key

Pass `--apiKey <key>` on `run`, `resume`, `run-tui`, or `resume-tui`:

```bash
agent-sandbox run -a codex --apiKey "$OPENAI_API_KEY" -q "inspect this repository"
agent-sandbox resume -b fix-login -a codex --apiKey "$OPENAI_API_KEY" --pr -q "add a regression test"
```

The flag has no shorthand and overrides the selected section's JSON `apiKey`.
Omit it to use that section's value and existing defaults. Use `--apiKey=""`
to clear a configured key and select host authentication with the required
configuration mounts above. Use the exact camelCase name; `--api-key` is rejected.
Nonempty keys for opencode or pi produce a usage error.

A key passed on the command line is part of the `agent-sandbox` process
arguments for the whole run, so other local users and processes can read it
from the process list (for example `ps` or `/proc/<pid>/cmdline`). A key typed
literally may also be saved in shell history; referencing an environment
variable such as `"$OPENAI_API_KEY"` avoids that, but the expanded value is
still visible in the process list. On shared hosts, prefer the JSON `apiKey`
below, and keep `agent-sandbox.json` readable only by you (for example,
`chmod 600 agent-sandbox.json`) and out of Git (for example, listed in
`.gitignore`).

Alternatively, create `agent-sandbox.json` in the invocation directory.
For Codex:

```json
{
  "run": {
    "agent": "codex",
    "apiKey": "<YOUR_OPENAI_API_KEY>",
    "query": "inspect this repository and summarize its structure"
  }
}
```

Then run:

```bash
agent-sandbox run
```

For Claude Code, use `"agent": "claude"` and your Anthropic API key instead.
In JSON, `apiKey` is supported only in the `run` and `resume` sections.

In this mode, the key is supplied to the container as the provider's environment
variable. The container uses a temporary home and does not mount host agent
credentials. Keys are not added to prompts, worktree records, help, or
transcript output.

::: warning Keep your key out of Git
Replace the placeholder locally and add `agent-sandbox.json` to your
project's `.gitignore` before storing a real key. Do not include keys in
prompts, screenshots, or shared logs.
:::

Continue with [your first run](first-run.md).
