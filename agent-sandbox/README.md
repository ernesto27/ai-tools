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
  and Claude Code, an API key in `./agent-sandbox.json`. opencode and pi require
  the host session.

| Agent | Host configuration when not using `api-key` |
| --- | --- |
| Codex | `~/.codex/` |
| Claude Code | `~/.claude/` and `~/.claude.json` |
| opencode | `~/.config/opencode/`, `~/.local/share/opencode/`, and `~/.local/state/opencode/` |
| pi | `~/.pi/agent/` |

Without `api-key`, if any of these paths is missing, the command fails without
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

```text
agent-sandbox run [-b <branch>] -a <codex|claude|opencode|pi> [-m <model>] [-i <image>] [--image <file>]... [--hn] [-p] [--pr] [-c <commit-message>] (-q <query> | -f <prompt-file>)
agent-sandbox resume -b <branch> -a <codex|claude|opencode|pi> [options] (-q <query> | -f <prompt-file>)
```

| Parameter | Description |
| --- | --- |
| `-b`, `--branch` | Optional. Branch for the separate worktree; generated if omitted. |
| `-a`, `--agent` | Required. One of `codex`, `claude`, `opencode`, or `pi`. |
| `-m`, `--model` | Optional. Overrides the model selected by the agent. |
| `-i`, `--base-image` | Optional. Derives an image from a compatible base to make its toolchain available inside the sandbox. |
| `--hn` | Optional. Shares the host network with the container, without port restrictions. Disabled by default; see the risks under "Host networking". |
| `-p`, `--push` | The agent creates the commit inside the container. If it finishes successfully and leaves the worktree clean, the host runs `git push --set-upstream origin <branch>` without creating a PR. |
| `--pr` | Optional. The agent commits inside the container; if it finishes successfully and leaves the worktree clean, the branch is published to `origin` and a GitHub pull request is created or reused. Does not require `--push`; see "GitHub pull requests". |
| `-q`, `--query` | Instructions for the agent. |
| `-c`, `--commit-message` | Optional. With `--push` or `--pr`, instructs the agent to use this exact message for the commit inside the container. |
| `-f`, `--file-prompt` | File whose contents are used as instructions for the agent, instead of `-q` or `--query`. |
| `--image <file>` | Optional and repeatable. Attaches images to the initial prompt for Codex or Claude Code. Each path must be an existing regular file on the host; opencode and pi ignore it. Use `--` before the text prompt so Codex does not interpret it as another image. |

Without `-p`/`--push` or `--pr`, changes remain uncommitted in the worktree.
With either option, the agent commits before finishing. Provide exactly one
prompt source: `-q`/`--query`, `-f`/`--file-prompt`, or the `"query"`/`"file-prompt"`
value in JSON. Both sources cannot be used at once, and positional instructions
are not accepted. Without `--commit-message`, the agent chooses a message based
on the actual changes.

To continue a registered worktree, use `agent-sandbox resume -b <branch>`.
The branch is the name shown by `worktree-list`. When combined with `--push`
or `--pr`, the agent must commit its changes before ending the session.

`run` and `resume` read `./agent-sandbox.json` if it exists in the directory
where they are invoked. The JSON root accepts `agent`, `model`, `base-image`,
`push`, `pr`, and `hn` as values shared by both commands.
Each section accepts the long option names `branch`, `agent`, `model`,
`base-image`, `query`, `push`, `pr`, `hn`, `commit-message`, `file-prompt`, and
`image`, plus the `api-key` field. `api-key` is available only in JSON: there is
no equivalent command-line option. For each field, an explicit command-line
option takes precedence, followed by the section value, then the root value.
A section's `false` or empty string also overrides the shared value.

```json
{
  "agent": "codex",
  "model": "gpt-6.1-sol",
  "base-image": "golang:1.26-alpine",
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
    "branch": "fix-login",
    "query": "add a regression test"
  }
}
```

With this file, `agent-sandbox run` uses the `run` section; `resume` uses the
`resume` section. You can also pass `-q "another task"` to override the JSON
query. Worktree management commands do not read this file.

### GitHub pull requests

`--pr` is available in `run` and `resume`, and can also be enabled with
`"pr": true` in the corresponding section of `agent-sandbox.json`. It requires
GitHub CLI (`gh`) installed and authenticated on the host for the server used
by `origin`, plus permission to push and create the PR. The fetch and push URLs
for `origin` must identify the same GitHub repository, and there must be a
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

Continue that worktree and update the PR branch:

```bash
agent-sandbox resume -b fix-login -a codex --pr -q "add a regression test for the login redirect"
```

With `--push` or `--pr`, if the agent exits with a nonzero status, publication
is skipped and changes remain in the worktree. If it finishes successfully but
leaves uncommitted changes, an error is reported and no push occurs. With a
clean worktree, the result is compared against the current base on `origin`.
Without differences to review, no push occurs and no PR is created. A clean
worktree with commits that introduce differences against the base can also
be published.

After the push, if an open PR already exists for the same repository, branch,
and base, its URL is displayed and its title, description, and draft status
are preserved. For a new PR, an additional invocation of the selected agent
generates the title and description from the full comparison; the PR is then
created ready for review, without marking it as a draft. That invocation also
consumes model usage. If PR generation or creation fails, the branch has already
been published; you can retry with `resume --pr`.

Combining `--pr` with `--push` follows this same flow, without duplicating the
commit or push. `--commit-message` controls the message given to the agent for
the commit, not the PR title.

### Host networking

`--hn` enables Docker's `host` network mode for the agent container.
It is disabled by default and can also be configured with `"hn": true` in
`run` or `resume` within `agent-sandbox.json`. To disable a value enabled in
JSON, pass `--hn=false`.

This mode shares the host network and reduces container isolation: the agent
can access local host services without a port allowlist. Use it when the task
needs that access. With `--pr`, the additional invocation that generates the
title and description uses the same network configuration.

### API keys for Codex and Claude Code

To use an API key instead of the host session, add `api-key` to the `run` or
`resume` section you will execute. For example, for Codex:

```json
{
  "run": {
    "agent": "codex",
    "api-key": "<OPENAI_API_KEY>",
    "query": "inspect this repository"
  }
}
```

For Claude Code:

```json
{
  "run": {
    "agent": "claude",
    "api-key": "<ANTHROPIC_API_KEY>",
    "query": "inspect this repository"
  }
}
```

### External base image

`--base-image` uses an image that already contains the project's runtime.
Alpine, Debian, Ubuntu, Fedora, RHEL 8/9, UBI 8/9, and Amazon Linux 2023 are
supported. For example, `golang:1.26-alpine` makes Go, `gofmt`, and `go test`
available inside the container:

```bash
agent-sandbox run -b fix-go-tests -a codex -i golang:1.26-alpine -q "run gofmt and go test ./..., then fix failures"
```

The first run creates a local derived image and installs the tools needed to
run the agents: Node.js, npm, Bash, Codex, Claude Code, opencode, pi, ripgrep,
CA certificates, curl, and Git. Later runs reuse that image for the same base
and check the selected agent's version on npm. If it differs from the installed
version, they rebuild the image with that version and retain the selected
base reference.

The base must provide `apk` (Alpine), `apt-get` (Debian/Ubuntu), `dnf`
(Fedora/RHEL/UBI/Amazon Linux), or `microdnf` (UBI minimal). Other bases fail
during the build. Alpine installs Node.js from its packages; other bases use
Node.js 22 from NodeSource.

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
