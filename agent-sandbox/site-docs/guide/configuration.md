# Configuration

`run`, `resume`, `run-tui`, and `resume-tui` read `./agent-sandbox.json` from
the directory where you invoke the command. The tool does not search parent
directories for it. Worktree management commands do not read this file.

## Save defaults for your project

```json
{
  "agent": "codex",
  "baseImage": "golang:1.26-alpine",
  "push": false,
  "pr": false,
  "run": {
    "query": "run go test ./... and fix failures"
  },
  "resume": {
    "branch": "fix-go-tests",
    "query": "add a regression test for the fix"
  }
}
```

With this configuration, `agent-sandbox run -b fix-go-tests` supplies the
branch explicitly and uses the saved agent, image, and prompt.
`agent-sandbox resume` then continues that branch using its section.

## Understand precedence

Values are resolved in this order, from highest priority to lowest:

1. Explicit CLI flags.
2. The `run` or `resume` section.
3. Shared root values.
4. Built-in defaults.

A section's `false` or empty string overrides the shared value too. For example:

```bash
agent-sandbox run --pr=false -q "inspect the failing tests"
```

This disables `pr` even if the JSON enables it. Also pass `--push=false` if
your configuration enables `push` and you want to leave changes uncommitted.

## Supported fields

Multiword JSON keys and long CLI flags use camelCase: `baseImage`,
`commitMessage`, and `filePrompt`. For example, use `--baseImage` on the CLI.
The JSON-only API key field is `apiKey`.
Rename the old `base-image`, `commit-message`, `file-prompt`, and `api-key`
names in existing files and invocations; they are rejected, with no aliases.
Short flags (`-i`, `-c`, `-f`) and names such as `hn` and `pr` are unchanged.

| Location | Fields |
| --- | --- |
| Shared root | `agent`, `model`, `baseImage`, `push`, `pr`, `hn` |
| `run` and `resume` | `branch`, `agent`, `model`, `baseImage`, `query`, `filePrompt`, `push`, `pr`, `hn`, `commitMessage`, `image`, `apiKey` |
| `run` only | `reviewers` |

Use strings for text fields, booleans for `push`, `pr`, and `hn`, and arrays
of strings for `image` and `reviewers`. Unknown fields, `null`, and wrong
types are rejected. Both command sections are validated even when only one
is being used.

`apiKey` and `reviewers` are JSON-only fields. See [authentication](authentication.md)
for keys and [publishing](publishing.md#request-reviewers) for reviewers.
