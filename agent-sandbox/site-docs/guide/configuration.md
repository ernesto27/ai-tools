# Configuration

`run`, `resume`, `run-tui`, and `resume-tui` read `./agent-sandbox.json` from
the directory where you invoke the command. The tool does not search parent
directories for it. Worktree management commands do not read this file.

## Save defaults for your project

```json
{
  "branch": "fix-go-tests",
  "agent": "codex",
  "effort": "high",
  "baseImage": "golang:1.26-alpine",
  "push": false,
  "pr": false,
  "run": {
    "query": "run go test ./... and fix failures"
  },
  "resume": {
    "query": "add a regression test for the fix"
  }
}
```

With this configuration, `agent-sandbox run` uses the shared `fix-go-tests`
branch and the saved agent, image, and prompt. `agent-sandbox resume` then
continues that branch using its section. A section's `branch` or an explicit
`-b` / `--branch` flag overrides the shared branch.

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
| Shared root | `branch`, `agent`, `model`, `effort`, `baseImage`, `push`, `pr`, `hn`, `file` |
| `run` and `resume` | `branch`, `agent`, `model`, `effort`, `baseImage`, `query`, `filePrompt`, `push`, `pr`, `hn`, `commitMessage`, `image`, `file`, `apiKey` |
| `run` only | `reviewers` |

Use strings for text fields, booleans for `push`, `pr`, and `hn`, and arrays
of strings for `image`, `file`, and `reviewers`. Unknown fields, `null`, and wrong
types are rejected. Both command sections are validated even when only one
is being used.

`apiKey` and `reviewers` are JSON-only fields. See [authentication](authentication.md)
for keys and [publishing](publishing.md#request-reviewers) for reviewers.

## Share reference files

Use `file` to mount reference files for any agent:

```json
{
  "file": ["/path/to/spec.pdf", "/path/to/sample.csv"],
  "resume": {
    "file": []
  }
}
```

An explicit CLI `--file` list replaces the section's array, which replaces
the root array. Arrays are not combined; an empty section array clears the
root files. CLI `--image` lists also replace their section arrays.
Relative paths in either array resolve from the invocation directory.
See [reference-file attachments](environment.md#mount-reference-files) for
container paths and PDF tools.

## Set effort globally

Set the root-level `effort` string to share a default across all agents and
all four execution commands:

```json
{
  "effort": "low"
}
```

An explicit `--effort` overrides a section's `effort`, which overrides the root
value. Leading and trailing whitespace is trimmed. Omitted, empty, or
whitespace-only effort resolves to `high`. Values are passed through without
checking allowed levels. The agent
reports unsupported values for its selected model.
