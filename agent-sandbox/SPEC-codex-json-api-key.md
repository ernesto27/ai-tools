# Codex and Claude API keys from local JSON configuration

## Objective

Allow a user to run or resume Codex or Claude with a literal provider API key
in the invocation directory's `agent-sandbox.json`, even when the host has no
configuration for that agent. The key applies only to that container run and
must not replace or modify a subscription login on the host. Keep the agent
boundary ready for opencode and pi to add their own key handling later.

This is one capability: selecting a per-invocation credential source for an
agent run. No capability map is needed.

## Behavior and JSON contract

The existing commands remain:

```text
agent-sandbox run [-b <branch>] -a <codex|claude> (-q <query> | -f <prompt-file>)
agent-sandbox resume -b <branch> -a <codex|claude> (-q <query> | -f <prompt-file>)
go build ./...
go vet ./...
go test ./...
```

Both `run` and `resume` accept an optional, case-sensitive `api-key` JSON
string in their own sections. No `--api-key` flag is added. As with other
JSON fields, one section never supplies defaults to the other.

```json
{
  "run": {
    "agent": "claude",
    "api-key": "<Anthropic Console API key>",
    "query": "inspect this repository"
  },
  "resume": {
    "branch": "existing-branch",
    "agent": "codex",
    "api-key": "<OpenAI Platform API key>",
    "query": "continue the task"
  }
}
```

An omitted or empty `api-key` retains the current host-configuration behavior,
including its error when the required host directory is missing. A nonempty
key selects API billing for that invocation. It does not require or mount the
host's Codex or Claude credential files, run an agent login command, or alter
any host credentials. The key is not copied into the worktree, stored in
worktree state, embedded in an image, or placed in command arguments. `run`
and `resume` apply the same selection rule.

For this version, a nonempty key with `agent` set to `opencode` or `pi` is a
usage error naming the unsupported agent. The optional `APIKeyAgent` contract
keeps each supported agent's key setup in its own package.

The selected key-backed Codex process uses a dedicated provider configuration
whose `env_key` reads an environment variable scoped to its disposable
container. The provider targets the OpenAI Responses API and uses the selected
model. That configuration is supplied only to the key-backed invocation; a
normal mounted-login run keeps its existing provider and settings. No cached
Codex login is needed. A writable, isolated Codex home is available for normal
session files, but it does not mount the host credential directory or persist
API key authentication after the container exits.

The key-backed Claude process uses `ANTHROPIC_API_KEY` with its existing
`--print` invocation. It runs with a writable tmpfs home and does not mount
host `~/.claude` or `~/.claude.json`. `CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST=1`
prevents settings files from replacing the container's provider credentials.
`CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=0` explicitly disables Claude's subprocess
scrubbing, including when a custom base image sets it to `1`; this avoids
requiring `bubblewrap` inside the sandbox image. Consequently, Claude's tool
subprocesses may inherit the API key inside the container. The ordinary
mount-backed Claude path remains unchanged when the key is absent.

The JSON file contains a literal secret. The repository already ignores its
own `agent-sandbox.json`; users invoking the tool from other repositories must
keep their local file private. The key is never included in error messages,
help, agent prompts, command arguments, or normal logs. Docker container
environment is accessible to principals with Docker daemon access and to
processes inside the container. This exposure is an explicit limitation of
the selected environment-based credential mechanism.

## Project structure and tech stack

- Go 1.26, Cobra 1.10.2, Go `encoding/json`, and the existing Docker Engine API
  client; no new runtime dependency.
- `cmd/agent-sandbox/`: JSON schema validation and `run`/`resume` option flow.
- `internal/sandbox/`: credential selection before worktree creation and
  per-invocation container assembly.
- `internal/agent/`: agent-owned credential modes, Codex provider settings,
  and Claude environment settings.
- `internal/docker/`: existing container environment and tmpfs facilities.
- `README.md`: user-facing Go CLI documentation, including the API billing
  example and secret handling. `../bash/README.md` remains a reference for the
  original shell tool; its behavior is outside this change.

## Code style

Preserve the dependency direction `cmd` -> `sandbox` -> `agent` and `docker`.
Use explanatory comments where credential isolation or provider selection
would otherwise be surprising. Keep Go code formatted with `gofmt`; use
table-driven tests for related cases. For example:

```go
// A key-backed run uses the agent's isolated container configuration so it
// cannot replace the caller's cached subscription login.
if opts.APIKey != "" {
    keyed := opts.Agent.(agent.APIKeyAgent)
    runOpts = keyed.APIKeyContainer(opts.APIKey)
}
```

## Testing strategy

Use Go's `testing` package without Docker for the normal test suite. Add
table-driven cases for both JSON sections, absent and empty keys, wrong JSON
types, Codex and Claude key-only operation without host config, other agents
rejecting a nonempty key, and preservation of mounted-login behavior without
a key.
Inspect generated container options to prove the key-backed path has no host
credential mount, passes the key only to the selected container environment,
selects Codex's custom provider or Claude's `--print` mode, and does not put
the key in arguments or errors. For Claude, assert both host credential mounts
are absent and the subprocess scrub variable is explicitly `0`.
Verify a key in one JSON section cannot affect the other. Run `go test ./...`,
`go vet ./...`, and `go build ./...`. A live API smoke test is optional because
it requires Docker and a billable provider key.

## Boundaries

- **Always:** Validate the JSON and unsupported-agent combination before
  image builds or worktree creation; preserve the current exit-status
  protocol; avoid printing the secret; isolate key-backed agent credentials
  from the host login; document API billing and Docker environment exposure.
- **Ask first:** Add a dependency, introduce a CLI key flag, persist a
  key-backed login, or expand API key support to opencode or pi.
- **Never:** Commit a real key, pass it as a process argument, copy it into the
  worktree or state file, or write it into a host agent credential store.

## Success criteria

1. With `run.api-key` and no host Codex or Claude configuration, `run` reaches
   the selected agent's container configured for API-key requests.
2. With `resume.api-key` and no host Codex or Claude configuration, `resume`
   does the same for a recorded worktree.
3. A key-backed run leaves the host subscription login untouched and creates
   no persistent API-key login.
4. With no nonempty key, both commands retain their current mount-based
   authentication and missing-config error.
5. Wrong JSON types and a nonempty key for an unsupported agent fail with a
   usage error before any Docker or worktree side effect.
6. Tests and documentation cover the per-section behavior, secret placement,
   and API billing choice.
7. Claude key runs work without `bubblewrap`; the explicit scrub setting is
   `0` and is covered by a container-options test.

## Decisions and remaining verification

Docker environment exposure was accepted for this feature. A live Claude run
with a clean home remains useful to verify first-run behavior for the installed
Claude Code version; unit tests cannot establish that behavior.

## Relationship to earlier specifications

`SPEC-run-resume-json-config.md` excluded credentials from JSON. This feature
intentionally changes that boundary for Codex and Claude's `api-key` field. The
existing JSON lookup, validation, and section isolation rules still apply.
