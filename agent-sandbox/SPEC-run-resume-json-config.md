# Run and resume JSON configuration

## Objective

Let a user keep per-command defaults in `agent-sandbox.json` in the directory
from which they invoke `agent-sandbox`. This reduces repeated flags while
preserving command-line control for a single invocation. Make new sessions
explicit with `agent-sandbox run`, matching `agent-sandbox resume`.

This is one CLI capability: both commands use the same config lookup and
precedence rules, and the `run` verb gives the JSON section an unambiguous
command name. No capability map is needed.

## Commands and behavior

```text
agent-sandbox run [-b <branch>] -a <agent> [flags] (-q <query> | -f <prompt-file>)
agent-sandbox resume -b <branch> -a <agent> [flags] (-q <query> | -f <prompt-file>)
agent-sandbox worktree-list
agent-sandbox worktree-delete -b <branch> [--force]
agent-sandbox worktree-delete-all [--yes]
agent-sandbox worktree-editor -b <branch>
```

- `agent-sandbox run` is the only new-session entry point. The root command
  dispatches subcommands and shows help; `agent-sandbox -a codex "task"` no
  longer starts a session. It reports a usage error (exit 2). `run` keeps the
  current new-session behavior, including generated branches and the requirement
  for exactly one prompt source.
- `resume` keeps its current behavior, including its required branch and prompt.
- `-q` and `--query` supply the agent instruction as a string on either
  command. Positional instructions are rejected. The existing
  `-f`/`--file-prompt` form remains available. `-p` and `--push` commit and
  push the branch.
- On `run` and `resume`, read only `./agent-sandbox.json`, relative to the
  process's current working directory. Do not search parent directories, the
  repository root, a worktree, or a user config directory. An absent file is
  normal; the commands then use their existing flag defaults and validations.
- The worktree management commands do not read or validate the file. Their
  behavior and flags stay as they are.

### JSON schema

The top-level value is an object with optional `run` and `resume` objects.
Each command object may contain any subset of that command's long flag names:
`branch`, `agent`, `model`, `base-image`, `query`, `push`,
`commit-message`, `file-prompt`, and `image`. String flags take JSON strings,
`push` takes a JSON boolean, and `image` takes an array of JSON strings.
Empty objects and omitted keys use the existing CLI defaults. The file
contains flag values, not positional arguments.

```json
{
  "run": {
    "agent": "codex",
    "model": "gpt-5.6-sol",
    "base-image": "golang:1.26-alpine",
    "query": "run go version and do not change any files",
    "push": false,
    "image": []
  },
  "resume": {
    "branch": "existing-branch",
    "agent": "codex",
    "model": "gpt-5.6-sol",
    "query": "continue the task",
    "push": false
  }
}
```

### Precedence and validation

1. Start with the current built-in defaults.
2. Apply only the selected command's JSON section, if present.
3. Apply explicitly supplied command-line flags. A CLI flag wins even when its
   value is an empty string or `false`. For `image`, explicitly supplied CLI
   values replace the entire JSON array instead of appending to it.
4. Validate the merged values using the existing command and sandbox rules
   before creating a worktree or container.

`run` never inherits `resume` values and `resume` never inherits `run` values.
Exactly one of `-q`/`--query`, `-f`/`--file-prompt`, or their JSON defaults must
provide the agent instruction after merging. A command-line prompt source
overrides `query` or `file-prompt` from JSON. Two prompt sources explicitly
supplied on the command line remain a usage error. Positional text is always
a usage error. A JSON `commit-message`
does not satisfy the prompt requirement: it only names the Git commit made
when `--push` is used. If the commit message is omitted, the resolved agent
prompt supplies its default.
Paths in `file-prompt` and `image` keep their current interpretation relative
to the invocation directory. The config must not be copied into the worktree
or passed to the container as an agent config file.

Malformed JSON, unknown top-level or command keys, and values of the wrong
JSON type are usage errors (exit 2) with the filename and offending field in
the message. An existing file that cannot be read is an error, not an absent
config. Missing required merged values continue to use the current usage-error
protocol and synopsis. Invalid config must fail before image builds, worktree
creation, or container startup. `--help` and `--version` do not need a valid
config file.

## Tech stack

Go 1.26, Cobra 1.10.2 and pflag for command parsing, and the Go standard
library's `encoding/json` for the config file. No new runtime dependency is
required.

## Project structure

- `cmd/agent-sandbox/`: command dispatch, flag binding, config loading and
  merging, usage handling, and CLI tests.
- `internal/sandbox/`: existing option validation and run/resume orchestration;
  retain its current responsibility for agent lookup and prompt-file content.
- `README.md`: user-facing Spanish CLI and JSON usage documentation.
- `test-worktree.sh`: opt-in manual lifecycle check using isolated invocation
  directories so a developer's own `agent-sandbox.json` cannot affect it.
- `../bash/README.md`: reference for the original shell CLI when updating
  user-visible behavior; the Go feature does not change the shell script.

## Code style

Keep the existing dependency direction (`cmd` to `sandbox` to leaf packages).
Use the project's explanatory comments at decision points. Use standard
`gofmt` formatting and precise, lower-case JSON keys matching long flags.
For example, a comment should explain why JSON defaults cannot mark a CLI
flag as explicitly supplied:

```go
// JSON defaults change bound values without marking a flag as changed, so an
// explicit false or empty string from the command line still wins.
if cmd.Flags().Changed(key) {
    continue
}
```

## Testing strategy

Use Go's `testing` package in `cmd/agent-sandbox`, with temporary directories
and no Docker dependency. Where one behavior has several cases, use a named
table of inputs and expected results with `t.Run`. Exercise config merging,
prompt validation, and resolved options without a production runner hook added
solely for tests. Cover:

- explicit `run` dispatch and rejection of the former root invocation;
- missing config, and lookup from the invocation directory only;
- separate `run`/`resume` sections and every supported value type;
- CLI overrides for strings, `--push=false`, and replacement of `image`;
- a JSON `query` reaching the agent instruction, while `commit-message`
  remains separate and cannot supply a missing prompt;
- `-q` overriding JSON `query` or `file-prompt`, while positional text and
  two explicit CLI prompt sources remain invalid;
- malformed, unreadable, wrongly typed, and unknown config fields;
- worktree commands ignoring even an invalid config file;
- help and version working with an invalid config file.

Run `go test ./...`, `go vet ./...`, and `go build ./...`. Unit tests should
exercise parsing and merging without starting an agent or Docker. The manual
`test-worktree.sh` uses `run` defaults from temporary JSON, `resume` defaults
with a CLI `-q` override, and explicit `run` for other sessions. Its full run
requires Docker and host agent authentication.

## Boundaries

- **Always:** Preserve the CLI exit-status protocol, validate before side
  effects, keep the config optional, and document the breaking `run` syntax
  and the `-q`/`--query` flag while retaining `-p`/`--push`.
- **Ask first:** Any new dependency, alternate config location or filename,
  additional JSON section, or change to worktree management commands.
- **Never:** Read credentials from JSON, pass JSON contents to the container,
  silently accept misspelled keys, or start a session from the root command.

## Success criteria

1. In a directory containing the example config, `agent-sandbox run` uses
   its `run` defaults and JSON query, and `agent-sandbox resume` uses its
   `resume` defaults and JSON query.
2. Explicit CLI flags override the corresponding JSON values, including
   `--push=false` and `-q "different task"`; neither command receives defaults
   from the other section.
3. Without the file, both commands work with CLI flags as they do today.
4. A malformed or unusable file fails before creating a worktree or container;
   worktree management commands continue to operate independently of it.
5. The former root invocation fails with exit 2, and help, examples, tests,
   and the Spanish README present the explicit `run` form.
6. `commit-message` alone never starts an agent session; the resolved agent
   prompt supplies the default commit message when `--push` is used without
   `--commit-message`.

## Decisions

Unknown JSON keys and wrong value types fail validation. A command-line
prompt source overrides a JSON query source. Positional instructions are
rejected. An explicitly supplied CLI
`--image` replaces the JSON image list.
