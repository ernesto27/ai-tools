# agent-sandbox

`agent-sandbox` runs Codex, Claude Code, opencode, or pi inside a Docker
container and on its own Git *worktree*. The agent works on a separate branch
and in a separate directory, without modifying the working copy from which
the command was invoked.

## Installation

Releases are published for Linux x86_64. Install the latest release with:

```bash
curl -fsSL https://raw.githubusercontent.com/ernesto27/ai-tools/master/agent-sandbox/install.sh | bash
```

The installer downloads the latest `agent-sandbox` release, verifies its
SHA-256 checksum, and places the binary in `~/.local/bin`. If that directory
is not yet in `PATH`, the installer prints the line to add to your shell profile.
Releases are created with `agent-sandbox-v*` tags.

To update to the latest release:

```bash
agent-sandbox update
```

To build from this directory:

```bash
go build -o agent-sandbox ./cmd/agent-sandbox
```

## Requirements

- Docker running.
- Git and a Git repository: run the command from any directory inside the
  repository.
- An authenticated session for the selected agent on the host or, for Codex
  and Claude Code, an API key through `--apiKey` or `./agent-sandbox.json`.
  opencode and pi require the host session.

| Agent | Host configuration when not using `apiKey` |
| --- | --- |
| Codex | `~/.codex/` |
| Claude Code | `~/.claude/` and `~/.claude.json` |
| opencode | `~/.config/opencode/`, `~/.local/share/opencode/`, and `~/.local/state/opencode/` |
| pi | `~/.pi/agent/` |

Without `apiKey`, if any of these paths is missing, the command fails without
creating it. Run and authenticate the corresponding agent on the host first.
With an API key for Codex or Claude Code, the container uses a temporary home
directory and does not mount host credentials.

On each execution, `agent-sandbox` checks the selected agent's version inside
the image against npm. It builds the image if it does not exist and rebuilds
it if a newer version is found; the first run can therefore take some time
and requires access to npm.

## Usage

To list host dependencies:

```bash
agent-sandbox doctor
```

This checks whether `git`, `docker`, `gh` (optional, for PRs), and `code`
(optional, for `worktree-editor`) are in `PATH`.

Browse the public OpenRouter model catalog with `agent-sandbox models`.

```text
agent-sandbox run [-b <branch>] -a <codex|claude|opencode|pi> [--apiKey <key>] [--reviewers <username>]... [-m <model>] [--effort <value>] [-i <image>] [--image <file>]... [--file <path>]... [--hn] [-p] [--pr] [-c <commitMessage>] (-q <query> | -f <prompt-file>)
agent-sandbox resume -b <branch> -a <codex|claude|opencode|pi> [options] (-q <query> | -f <prompt-file>)
```

| Parameter | Description |
| --- | --- |
| `-b`, `--branch` | Optional. Branch for the separate worktree; generated if omitted. |
| `-a`, `--agent` | Required. One of `codex`, `claude`, `opencode`, or `pi`. |
| `--apiKey <key>` | Optional. Provider API key for Codex or Claude Code on `run`, `resume`, `run-tui`, and `resume-tui`; overrides the section's JSON key. `--apiKey=""` selects host authentication. No shorthand. |
| `--reviewers <username>` | Optional. Repeat for each GitHub username on `run`, `resume`, `run-tui`, and `resume-tui`. Replaces `run.reviewers`; `--reviewers=""` alone clears it. Validated but has no effect on resume. No shorthand. |
| `-m`, `--model` | Optional. Overrides the model selected by the agent. |
| `--effort` | Optional. Agent effort; defaults to `high`. Passed through without checking supported levels. |
| `-i`, `--baseImage` | Optional. Derives an image from a compatible base to make its toolchain available inside the sandbox. |
| `--hn` | Optional. Shares the host network with the container, without port restrictions. Disabled by default; see the risks under "Host networking". |
| `-p`, `--push` | The agent creates the commit inside the container. If it finishes successfully and leaves the worktree clean, the host runs `git push --set-upstream origin <branch>` without creating a PR. |
| `--pr` | Commit and publish a PR. `run` creates or reuses one; `resume` appends to an existing open PR. Includes `--push`; see "GitHub pull requests". |
| `-q`, `--query` | Instructions for the agent. |
| `-c`, `--commitMessage` | Optional. With `--push` or `--pr`, instructs the agent to use this exact message for the commit inside the container. |
| `-f`, `--filePrompt` | File whose contents are used as instructions for the agent, instead of `-q` or `--query`. |
| `--file <path>` | Optional and repeatable. Mounts any existing regular file read-only for every agent. Filenames are preserved under `/agent-sandbox-files/<number>/`; a filename-to-container-path list is added to the prompt. |
| `--image <file>` | Optional and repeatable. Attaches images to the initial prompt for Codex or Claude Code. Each path must be an existing regular file on the host; opencode and pi ignore it. Use `--` before the text prompt so Codex does not interpret it as another image. |

Without `-p`/`--push` or `--pr`, changes remain uncommitted in the worktree.
With either option, the agent commits before finishing. Provide exactly one
prompt source: `-q`/`--query`, `-f`/`--filePrompt`, or the `"query"`/`"filePrompt"`
value in JSON. Both sources cannot be used at once, and positional instructions
are not accepted. Without `--commitMessage`, the agent chooses a message based
on the actual changes.

To continue a registered worktree, use `agent-sandbox resume -b <branch>`.
The branch is the name shown by `worktree-list`. When combined with `--push`
or `--pr`, the agent must commit its changes before ending the session.

`run` and `resume` read `./agent-sandbox.json` if it exists in the directory
where they are invoked. The JSON root accepts `branch`, `agent`, `model`, `effort`, `baseImage`,
`push`, `pr`, `hn`, and `file` as values shared by both commands.
Each section accepts the long option names `branch`, `agent`, `model`, `effort`,
`baseImage`, `query`, `push`, `pr`, `hn`, `commitMessage`, `filePrompt`, and
`image` and `file` (arrays of paths), plus the `apiKey` field. Only `run`
accepts a JSON `reviewers` array. For `file`, CLI values replace the section
array, which replaces the root array; arrays are not combined, and a section's
empty array clears the root files. Explicit CLI image lists replace their
section JSON lists. For each supported field, an explicit command-line option
takes precedence, followed by the section value, then the root value.
A section's `false` or empty string also overrides the shared value.
The complete JSON file is validated even when CLI flags override its values.
Neither `apiKey` nor `reviewers` is accepted at the JSON root; `resume.reviewers`
is also invalid, even though resume accepts and ignores the CLI flag.

Multiword JSON keys and long CLI flags use camelCase: `baseImage`,
`commitMessage`, `filePrompt`, and `apiKey`.
The previous names `base-image`, `commit-message`, `file-prompt`, and `api-key`
are rejected. Rename them in existing configuration files and invocations.
Short flags (`-i`, `-c`, `-f`) and names such as `hn` and `pr` are unchanged.

```json
{
  "branch": "fix-login",
  "agent": "codex",
  "model": "gpt-6.1-sol",
  "effort": "high",
  "baseImage": "golang:1.26-alpine",
  "push": false,
  "pr": false,
  "hn": false,
  "run": {
    "model": "gpt-5.6-sol",
    "query": "run go version and do not change any files",
    "push": false,
    "reviewers": ["alice", "bob"]
  },
  "resume": {
    "query": "add a regression test"
  }
}
```

With this file, `agent-sandbox run` uses the `run` section; `resume` uses the
`resume` section. Both use the shared `fix-login` branch unless overridden
in their section or with `-b` / `--branch`.
You can also pass `-q "another task"` to override the JSON
query. Worktree management commands do not read this file.

### GitHub pull requests

`--pr` is available in `run` and `resume`, and can also be enabled with
`"pr": true` in the corresponding section of `agent-sandbox.json`. It requires
GitHub CLI (`gh`) installed and authenticated on the host for the server used
by `origin`, plus permission to push and create or edit the PR. The fetch and
push URLs for `origin` must identify the same GitHub repository, with a
single push destination. With `--push` and `--pr`, the commit is created inside
the container; push and GitHub operations run on the host. For `--pr`, the
container receives write access to the repository's Git metadata and uses
the Git identity configured on the host.

The PR base is the branch from which the worktree was created with `run`, and
is recorded for future `resume` invocations. That branch must exist on `origin`
and differ from the worktree branch. `--pr` is not supported when starting
from a *detached HEAD* or continuing older records without `base_branch`;
another base is not chosen automatically.

Create a worktree and publish its changes as a PR:

```bash
agent-sandbox run -b fix-login -a codex --pr -q "fix the login redirect loop"
```

Continue the task and append its session summary to the PR:

```bash
agent-sandbox resume -b fix-login -a codex --pr -q "add a regression test for the login redirect"
```

Publication requires a successful agent exit and a clean worktree.

With `run --pr`, the same agent session generates a title and checklist from the
full branch comparison. Without reviewable changes, nothing is published.
Otherwise, the host pushes and creates a PR ready for review or reuses an open
PR for the same head and base without changing its metadata.

`resume --pr` requires a matching open PR. It pushes and appends a checklist of
this session's committed changes, preserving earlier text, the title, draft
status, and reviewers. If the agent reports no net changes, the host pushes
without editing the description. The TUI commands behave the same way.

If PR creation or editing fails after pushing, the error is reported and the
branch remains published. Failed description updates are not saved or replayed.

Combining `--pr` with `--push` follows this same flow, without duplicating the
commit or push. `--commitMessage` controls the message given to the agent for
the commit, not the PR title.

### Host networking

`--hn` enables Docker's `host` network mode for the agent container.
It is disabled by default and can also be configured with `"hn": true` in
`run` or `resume` within `agent-sandbox.json`. To disable a value enabled in
JSON, pass `--hn=false`.

This mode shares the host network and reduces container isolation: the agent
can access local host services without a port allowlist. Use it when the task
needs that access.

### Request PR reviewers

All four execution commands accept repeatable `--reviewers <username>`, without
a shorthand. On `run` and `run-tui`, an explicit list replaces `run.reviewers`
entirely. Omit the flag to use the JSON list. Use `--reviewers=""` alone to clear
configured reviewers; empty or whitespace-only entries in any other list are
usage errors, including an empty value combined with a username.

```bash
agent-sandbox run -a claude --pr --reviewers alice --reviewers bob -q "fix the login redirect"
agent-sandbox run -a codex --apiKey="" --reviewers="" -q "inspect this repository"
```

Usernames are trimmed and deduplicated case-insensitively, preserving the first
occurrence's order and spelling. Use letters, digits, and hyphens, starting with
a letter or digit. Mentions, email addresses, teams, comma-separated lists, and
option-like strings are rejected before sandbox execution.

Reviewers are requested only when a run creates a new PR with `--pr`. Reused PRs,
push-only runs, runs without `--pr`, no reviewable changes, failed agents, and
cancellation do not request reviewers. Create recovery checks existing requests
without re-requesting them. `resume` and `resume-tui` validate reviewer flag syntax,
then ignore valid values, including clearing: they preserve existing reviewers.
With `--pr`, resume still requires an existing open PR.

### API keys for Codex and Claude Code

All four execution commands accept `--apiKey <key>`, without a shorthand.
It overrides the selected section's JSON `apiKey`. Omit the flag to use that
section's key and existing defaults. Pass `--apiKey=""` to clear a configured
key and use host authentication, including its required configuration mounts.
Nonempty keys are supported only for Codex and Claude Code; other agents report
a usage error. Keys enter the container environment with a disposable home;
they are not added to prompts, worktree records, help, or transcript output.
A `--apiKey` value is part of the host process arguments for the whole run, so
other local users and processes can read it from the process list (for example
`ps`), and a literal value may be saved in shell history. Referencing an
environment variable avoids the history entry but not the process list; on
shared hosts, prefer the JSON `apiKey` shown below, and keep `agent-sandbox.json`
readable only by you (for example, `chmod 600 agent-sandbox.json`) and out of
Git (for example, listed in `.gitignore`).

```bash
agent-sandbox run -a codex --apiKey "$OPENAI_API_KEY" -q "inspect this repository"
agent-sandbox resume -b fix-login -a codex --apiKey "$OPENAI_API_KEY" --pr -q "add a regression test"
```

Alternatively, add `apiKey` to the `run` or `resume` JSON section you will execute.
For example, for Codex:

```json
{
  "run": {
    "agent": "codex",
    "apiKey": "<OPENAI_API_KEY>",
    "query": "inspect this repository"
  }
}
```

For Claude Code:

```json
{
  "run": {
    "agent": "claude",
    "apiKey": "<ANTHROPIC_API_KEY>",
    "query": "inspect this repository"
  }
}
```

### External base image

`--baseImage` uses an image that already contains the project's runtime.
Alpine, Debian, Ubuntu, Fedora, RHEL 8/9, UBI 8/9, and Amazon Linux 2023 are
supported. For example, `golang:1.26-alpine` makes Go, `gofmt`, and `go test`
available inside the container:

```bash
agent-sandbox run -b fix-go-tests -a codex -i golang:1.26-alpine -q "run gofmt and go test ./..., then fix failures"
```

The first run creates a local derived image and installs the tools needed to
run the agents: Node.js, npm, Bash, Codex, Claude Code, opencode, pi, ripgrep,
CA certificates, curl, Git, and Poppler PDF tools (`pdftotext` and `pdfinfo`). Later runs reuse that image for the same base
and check the selected agent's version on npm. If it differs from the installed
version, they rebuild the image with that version and retain the selected
base reference.

The base must provide `apk` (Alpine), `apt-get` (Debian/Ubuntu), `dnf`
(Fedora/RHEL/UBI/Amazon Linux), or `microdnf` (UBI minimal). Other bases fail
during the build. A single Dockerfile serves both default and derived images.
When both Node.js and npm are already available, they are reused. Otherwise,
Alpine installs them from its packages; other bases use Node.js 22 from NodeSource.

On RHEL, the image must have enabled repositories and, where applicable, a
valid subscription: the sandbox does not mount host credentials. It does not
enable EPEL or CRB/CodeReady Builder, and does not support `yum`; if a package
such as `ripgrep` is missing, the build fails with the package manager's error.

```bash
docker image ls 'agent-sandbox-base-*'
docker image rm <IMAGE_ID>
```

### Models

When `--model` is omitted, Codex uses `gpt-6.1-sol` and Claude Code uses
`claude-opus-5-5`. opencode and pi let their own configuration select the model.
Values of `--model` are passed directly to the agent: for example, `gpt-5.6-sol`
for Codex, `sonnet` for Claude Code, and `provider/model` for opencode or pi.

### Effort

All four agents default to `high`. Set `--effort` per invocation, or set the
global `effort` string in `./agent-sandbox.json` for a shared default:

```json
{
  "effort": "low"
}
```

Precedence is explicit CLI flag, then the `run` or `resume` section, then the
global JSON value, then `high`. Leading and trailing whitespace is trimmed;
an empty or whitespace-only value resets effort to `high`.
The same rules apply to `run-tui` and `resume-tui`.

```bash
agent-sandbox run -a codex --effort low -q "inspect this repository"
```

The value is passed to Codex's `model_reasoning_effort`, Claude's `--effort`,
opencode's `--variant`, or pi's `--thinking`. Supported levels depend on the
agent and model; the agent reports unsupported values and its exit status is
propagated. The sandbox does not validate levels or modify host agent settings.

## Examples

Create a worktree for Codex and leave its changes ready for review:

```bash
agent-sandbox run -a codex -m gpt-5.6-sol -q "fix the login redirect loop"
```

Run Claude Code and publish the branch when it finishes:

```bash
agent-sandbox run -b add-test -a claude -m sonnet -p -q "add a regression test for the login redirect"
```

Continue a registered worktree by name:

```bash
agent-sandbox resume -b fix-login -a codex -q "add a regression test for the login redirect"
```

Let opencode resolve its configured model:

```bash
agent-sandbox run -b update-copy -a opencode -q "update the empty-state copy"
```

Run Codex with a Go image:

```bash
agent-sandbox run -b fix-go-tests -a codex -i golang:1.26-alpine -q "run go test ./... and fix failures"
```

Read the prompt from a file:

```bash
agent-sandbox run -b prompt-file-test -a codex -f prompt.md
```

Attach one or more images to the initial prompt for Codex or Claude Code:

```bash
agent-sandbox run -a claude \
  --image "/home/user/Pictures/mockup.png" \
  --image "/home/user/Pictures/reference.png" \
  -q "compare these screenshots and implement the resulting UI"
```

Mount reference files for any agent, and refer to their names in the prompt:

```bash
agent-sandbox run -a claude \
  --file "/home/user/Documents/spec.pdf" \
  --file "/home/user/Documents/sample.csv" \
  -q 'read "spec.pdf" and "sample.csv", then implement the requested feature'
```

The prompt lists each file's container path. Duplicate filenames have separate
numbered directories; use their container paths to distinguish them. Paths are
resolved from the invocation directory, including JSON paths. `--file` is also
available on `resume`, `run-tui`, and `resume-tui`. It does not replace `-f` /
`--filePrompt`, which reads the instructions themselves from a file.

Default and derived images include Poppler tools for PDF text extraction and
inspection. PDF tools are installed when images are built; there is no runtime
probe or automatic rebuild for missing PDF tools.
Files are read when requested by the prompt; they are not automatically converted.

## Manage worktrees created by the sandbox

The command creates worktrees at
`~/.config/agent-sandbox/worktrees/<repo>-<hash>/<branch>` and records those it
created in `~/.config/agent-sandbox/worktrees.jsonl`. The repository identifier
prevents collisions between branches with the same name in different
repositories. The following subcommands only list and delete those entries;
they do not affect manually created worktrees.

```bash
# Show registered worktrees for the current repository.
agent-sandbox worktree-list

# Open a registered worktree in VS Code.
agent-sandbox worktree-editor -b fix-login

# Delete a clean worktree and its branch.
agent-sandbox worktree-delete --branch fix-login

# Also allow deletion of its uncommitted changes.
agent-sandbox worktree-delete -b fix-login --force

# Show all worktrees, ask for confirmation, and delete them and their branches.
agent-sandbox worktree-delete-all

# Skip confirmation for bulk deletion.
agent-sandbox worktree-delete-all --yes
```

`worktree-delete` refuses to discard changes without `--force`.
`worktree-delete-all` always deletes uncommitted changes after confirmation
(or immediately with `--yes`). Do not run deletion commands from the worktree
you want to remove.
`worktree-editor` requires `-b` and opens the branch's worktree in VS Code.
