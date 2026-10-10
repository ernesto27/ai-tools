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

Create `agent-sandbox.json` in the directory where you will invoke the command.
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
`apiKey` is supported only in the `run` and `resume` JSON sections.

In this mode, the key is supplied to the container as the provider's environment
variable. The container uses a temporary home and does not mount host agent
credentials.

::: warning Keep your key out of Git
Replace the placeholder locally and add `agent-sandbox.json` to your
project's `.gitignore` before storing a real key. Do not include keys in
prompts, screenshots, or shared logs.
:::

Continue with [your first run](first-run.md).
