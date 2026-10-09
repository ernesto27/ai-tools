# Installation

## What you need

The published binary and installer support **Linux x86_64**.

- Git, and a Git repository to work in.
- Docker installed, running, and accessible to your user.
- An authenticated agent on your host, or an API key for Codex or Claude Code.


## Install the latest release

```bash
curl -fsSL https://raw.githubusercontent.com/ernesto27/ai-tools/master/agent-sandbox/install.sh | bash
```

## Check your setup

```bash
agent-sandbox --version
agent-sandbox doctor
```

`doctor` checks whether `git`, `docker`, `gh`, and `code` exist in `PATH`.
It succeeds even if dependencies are missing and does **not** check Docker
service health, authentication, or permissions. `docker info` checks whether
your user can reach Docker.

## Update

```bash
agent-sandbox update
```

This runs the installer again. Updating the CLI and updating the container's
agent are separate: each run or resume checks the selected agent against npm
and rebuilds its image if needed.

## Build from source

With the Go version required by `go.mod` installed, run from the
`agent-sandbox` directory of the repository:

```bash
go build -o agent-sandbox ./cmd/agent-sandbox
./agent-sandbox --help
```

Next, [choose an authentication mode](authentication.md).
