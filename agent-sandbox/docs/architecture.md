# Architecture reference

This document maps the current implementation. [README.md](../README.md)
describes the CLI contract; consult the linked source when changing behavior.

## Package responsibilities

```text
cmd/agent-sandbox -> config
                 -> openrouter
                 -> sandbox -> agent -> docker
                            -> docker
                            -> git
                            -> github
                            -> utils
```

- [cmd/agent-sandbox](../cmd/agent-sandbox/root.go) uses Cobra for dispatch,
  flags, and usage errors. [config.go](../cmd/agent-sandbox/config.go) loads
  `./agent-sandbox.json` from the invocation directory. Explicit CLI flags
  override command-section values, which override shared root values. Shared
  execution bindings register `--apiKey <key>` and repeatable
  `--reviewers <username>` on all four commands, without shorthands. The loader
  validates the complete JSON file before applying CLI overrides; the schema
  still accepts `apiKey` only in sections and `reviewers` only in `run`.
  [run_tui.go](../cmd/agent-sandbox/run_tui.go) adds `run-tui` and `resume-tui`
  using the same execution and configuration as `run` and `resume`.
  [update.go](../cmd/agent-sandbox/update.go) runs the embedded
  [install.sh](../install.sh) with Bash to install the latest release, independently
  of sandbox execution and configuration.
  [models.go](../cmd/agent-sandbox/models.go) owns the model browser's terminal
  layout, provider selection, and asynchronous OpenRouter requests. Request
  identifiers prevent older responses from replacing a newer selection.
- [internal/clierror](../internal/clierror/error.go) carries structured error
  categories, summaries, details, recovery hints, and wrapped causes without
  printing or invoking commands. The CLI's
  [error_output.go](../cmd/agent-sandbox/error_output.go) renders plain-command
  failures to stderr with terminal-aware color and wrapping. Existing usage and
  exit-status handling remain in the CLI; TUI final output retains its format.
  Sandbox, Git, GitHub, and configuration code annotate the initial worktree,
  missing executable, and configuration cases where facts are available.
- [config](../config/config.go) exposes the public `Config` and `Section` JSON
  types and a `DefaultModel` accessor backed by `internal/agent`. The CLI owns
  file loading, validation, reviewer normalization, and flag merging.
- [internal/openrouter](../internal/openrouter/openrouter.go) fetches the public
  model catalog without authentication, filtering by model creator with
  `model_authors` and sorting by `newest`. It decodes model metadata and formats
  input/output prices in USD per million tokens. The CLI formats context limits
  and the `created` timestamp, which records when the model was added to OpenRouter.
- [internal/sandbox](../internal/sandbox/sandbox.go) orchestrates image preparation,
  worktree creation or reuse, container execution, and publication.
  [Runtime](../internal/sandbox/runtime.go) routes output and execution events
  without creating a separate TUI execution workflow.
- [internal/git](../internal/git/git.go) wraps the Git CLI. `Repo.At(dir)` addresses
  the same repository through a worktree.
- [internal/docker](../internal/docker/run.go) uses the Docker Engine API for
  builds, throwaway probes (`Output`), and attached execution (`Run`).
- [internal/agent](../internal/agent/agent.go) defines agent-specific commands,
  models, authentication, and container options. It depends on Docker option types.
  Effort resolves in `sandbox.NewOptions` to the shared `sandbox.DefaultEffort`
  constant (`high`) when empty, after CLI and JSON overrides are merged and
  leading and trailing whitespace is trimmed.
  `Args` and `APIKeyArgs` receive the resolved effort;
  each adapter builds its own argument without validating allowed levels or
  adding effort escaping.
- [internal/github](../internal/github/github.go) wraps host-side GitHub CLI
  authentication and pull request operations, including creation-time reviewer
  requests and read-only confirmation after an ambiguous create failure.
- [internal/utils](../internal/utils/utils.go) provides prompt-file loading.

## Execution and worktree ownership

[Run and Resume](../internal/sandbox/sandbox.go) resolve the repository and
worktree before sharing container execution through `Runtime.execute`.
`Run` creates and registers a worktree; `Resume` requires an existing valid
record and locks that worktree for the session. The TUI commands call these
same entry points.

[config.go](../internal/sandbox/config.go) uses `os.UserConfigDir()` for
`agent-sandbox/worktrees/<repo>-<hash>/<branch>` and `agent-sandbox/worktrees.jsonl`.
On Linux this normally means `~/.config`, honoring `XDG_CONFIG_HOME`.
The slug combines the repository basename with a short hash of its resolved path.
New runs refuse existing worktree paths.

[worktreestate.go](../internal/sandbox/worktreestate.go) records repository,
path, branch, base branch, and creation time. Worktree management selects only
registered sandbox worktrees; manually created worktrees are outside its scope.
A run records its worktree before starting the agent and rolls back creation
if registration fails. Delete commands rewrite the registry.
[worktreelock.go](../internal/sandbox/worktreelock.go) coordinates resume and deletion.

## Authentication and container mounts

`containerOptions` in [sandbox.go](../internal/sandbox/sandbox.go) selects one
of two credential modes:

- Host authentication calls `Agent.Container(home)` to validate and bind mount
  existing configuration. Missing required paths fail rather than being created.
  [Codex](../internal/agent/codex.go) mounts `.codex`; [Claude](../internal/agent/claude.go)
  mounts `.claude` and `.claude.json`; [opencode](../internal/agent/opencode.go)
  mounts configuration, data, and state; [pi](../internal/agent/pi.go) mounts `.pi/agent`.
- `--apiKey` or section JSON `apiKey` authentication requires `APIKeyAgent`,
  implemented by Codex and Claude. The key is supplied as `OPENAI_API_KEY` or `ANTHROPIC_API_KEY` in
  the container environment. These runs use a tmpfs home and do not mount host
  agent credentials. An explicit flag overrides the selected section, including
  `--apiKey=""`, which selects host authentication and its required mounts.
  Nonempty keys for other agents remain usage errors. Keys are not added to
  prompts, worktree records, help, or transcripts. The `--api-key` alias is rejected.

The container runs as the invoking UID/GID with the worktree at `/workspace`.
It also mounts the repository's common Git directory at its original absolute
path and receives Git directory settings and the host's commit identity.
That metadata mount is writable, with nested read-only mounts protecting its
shared `config` and existing `config.worktree` files for the main and current
worktrees. [gitconfig.go](../internal/sandbox/gitconfig.go) validates these files
before container creation; the shared config is required. Missing optional
worktree config files are not created or protected. Git commits and history
remain available, but the separate worktree does not isolate other shared Git
metadata or inherited Git environment variables. Supported image attachments
receive separate read-only mounts. Generic `--file` attachments are mounted for
all agents outside the worktree under numbered directories that preserve
basenames. [files.go](../internal/sandbox/files.go) supplies the same container
paths to mounts and the initial prompt manifest.
`HostNetwork` enables Docker host networking only when requested.

## Commit and publication flow

[Options.FullPrompt()](../internal/sandbox/options.go) appends mode-specific
instructions. Without `--push` or `--pr`, it forbids staging, committing, and
pushing. With either option, it instructs the agent to stage and commit related
changes inside the container, using `CommitMessage` exactly when supplied or
choosing a short message from the actual changes. It forbids agent-side push
and PR creation and constrains Git commands and metadata changes.

[publishResult](../internal/sandbox/pullrequest.go) skips publication after a
nonzero agent exit. Both publishing modes require a clean worktree; the host
does not create the commit. `--push` alone pushes the recorded branch to `origin`.

For `--pr`, preflight validates GitHub authentication, origin URLs, and a named
recorded base branch. Execution fetches a base snapshot before the agent runs.
The same agent session writes title and body to the untracked
`.agent-sandbox-pr.json` artifact after committing, using the complete branch
comparison. [prcontent.go](../internal/sandbox/prcontent.go) validates and
consumes that artifact before publication. The host checks for reviewable
differences against the snapshot, pushes, and reuses an open PR for the same
head and base or creates a PR ready for review. Combining `--push` and `--pr`
uses this single PR publication flow.

Repeatable CLI `--reviewers` values replace `run.reviewers` entirely; omitted
flags use the JSON list. A single `--reviewers=""` clears it. All other empty or
whitespace-only entries are usage errors. Both input paths trim usernames,
validate letters, digits, and hyphens with an initial letter or digit, and
deduplicate case-insensitively, preserving first occurrence order and spelling.
Mentions, emails, teams, comma lists, and option-like strings are rejected.
`run-tui` uses the same configuration. `resume` and `resume-tui` validate CLI
reviewer syntax, then discard valid values before execution; they add no effective
reviewer configuration and still reject `resume.reviewers` in JSON.
Reviewer options pass through `Options` to host publication only for new PR
creation with `--pr`; existing PRs are not modified. Push-only runs, absent
`--pr`, no reviewable changes, agent failures, and cancellation do not request
reviews. `Resume` also clears reviewer options at its entry point, and reviewers
are not stored in worktree records or prompts. Resume with `--pr` continues to
require an existing open PR and preserves its reviewers.
After a failed create call with reviewers, recovery also checks outstanding
review requests before reporting success; missing requests remain an error.

## Image lifecycle and cleanup

[embed.go](../embed.go) embeds [Dockerfile](../Dockerfile) so the binary can
build the default image without a checkout. [baseimage.go](../internal/sandbox/baseimage.go)
uses the same embedded Dockerfile with `BASE_IMAGE` for external bases and
selects their derived images.
Default and derived images install Poppler PDF tools at build time, without
a runtime probe or rebuild fallback for missing PDF tools. Derived tags hash
the Dockerfile template.
The same Dockerfile installs distribution Chromium, fonts, FFmpeg for video
recording, and Puppeteer Core
25.15.0 under `/opt/agent-sandbox-browser` (Node >=22.12.0). Base-image package
repositories must provide Chromium, fonts, and FFmpeg. The `agent-sandbox-browser` helper
takes a URL, PNG output path, and optional required selector; custom assertions
can import `launch` from `/opt/agent-sandbox-browser/browser.cjs`. Each browser
uses a temporary writable home, removed when the browser disconnects. Chromium's
inner sandbox is disabled within the existing Docker isolation; no additional
container privileges are requested. Browser tools are installed only at build
time, without runtime probes or rebuild fallbacks for older images. Rebuild an
existing default image to pick up the updated Dockerfile.

Image preparation compares the selected agent's installed version with
`npm view <package> version` on every run or resume and rebuilds on a mismatch,
pinning its `BuildArg` and retaining the external base when applicable.

[docker.Run](../internal/docker/run.go) registers the next-exit wait before
starting the container, drains output, and stops and waits on cancellation
using a context derived with `context.WithoutCancel`. Deferred removal also
covers attach/start failures and unexpected wait failures. Preserve these
orderings so an agent cannot outlive the command and hold its worktree open.

## Automatic verification evidence

[verification.go](../internal/sandbox/verification.go) contains instructions
appended by `Options.FullPrompt` for all agents and plain/TUI run/resume commands.
The agent attempts task-appropriate checks, creates
`/workspace/.agent-sandbox-verification/`, and saves a `summary.md` containing
checks, commands, exit codes, results, and limitations; one `verification.log`
when there is useful output; and useful final UI screenshots and a demo video. It creates a local
`.gitignore` containing `*` to exclude all evidence, including that ignore file,
from normal staging. The prompt forbids staging, committing, or uploading evidence
and asks the agent to update evidence on resume rather than accumulating copies.
Separate exit-code files, per-command or empty logs, duplicate screenshots, and
intermediate screenshots are not requested. Temporary verification scripts and
intermediate artifacts created by the current execution are removed after checks;
project files and user-provided files are left untouched.
For web UI changes, the prompt requests retained `desktop.png` and `mobile.png`
at 1280x800 and 390x844 viewports in the requested theme, plus a short `demo.webm`
showing the changed functionality through relevant clicks, typing, and navigation.
The agent finalizes the recording before closing the browser, retains the final
video alongside the screenshots during cleanup, and removes incomplete recordings
created during the current execution. On resume, it updates the video with the
other evidence rather than accumulating copies. Recording uses an API supported
by the installed Puppeteer and Chromium versions, with FFmpeg available in newly
built images. The agent describes what the video demonstrates in `summary.md`
and reports saved screenshot and video paths, or explains unavailable capture
in the summary.

Verification storage, checking, and reporting are agent-driven. The host does not
create directories, parse reports, or add publication checks. Failed or unavailable
verification must not prevent completion or publication; existing agent-exit,
cancellation, and clean-worktree behavior remains unchanged. Browser tools are
one verification option; backend, infrastructure, documentation, and other tasks
use relevant project checks and report missing prerequisites as limitations.

## Claude output

[Claude arguments](../internal/agent/claude.go) request print mode with
`--output-format stream-json --verbose --include-partial-messages` for both
credential modes and all four execution commands. CLI presentation in
[agent_output.go](../cmd/agent-sandbox/agent_output.go) supplies a stdout formatter
through `Runtime.FormatStdout` and `RunOptions.FormatStdout`. Docker's non-TTY
pump wraps only agent stdout and flushes the formatter after draining the
attachment; stderr and image/version probes retain their existing routing.
The formatter buffers an incomplete line, then uses Go's `json.Indent` to display
each event with two-space indentation without removing fields or changing their
order, numbers, or string escapes. Non-JSON lines pass through unchanged. A
trailing event without a newline is flushed at EOF. Partial and completed events
are both retained. The TUI displays the formatted event text through its existing
terminal decoder and records it in the transcript.
Sandbox preparation and publication messages still accompany agent output, so
the entire command output is not a pure JSONL stream. Stdin and terminal ownership
retain their existing behavior. Exit status and publication depend on the
container result, not on JSON event contents.

## Exit status

[fail()](../cmd/agent-sandbox/main.go) maps `UsageError` to a usage synopsis and
exit 2, cancellation to exit 130, `StatusError` to its status, and other errors
to exit 1. Successful command dispatch returns the agent's own exit code.
Route Cobra flag and argument errors through the existing hooks. Check required
flags in `RunE` rather than `MarkFlagRequired`, which bypasses those hooks.

## Adding an agent

Implement `Agent` under [internal/agent](../internal/agent/agent.go) and add it
to `registry`, whose order determines CLI display order. Add the matching
version argument and npm install to the shared [Dockerfile](../Dockerfile).
`Container` validates and mounts host configuration. Implement `APIKeyAgent`
only if the agent supports isolated key authentication. `DefaultModel()`
returning an empty string leaves model selection to the agent; Codex and Claude
read defaults from [providers.json](../internal/agent/providers.json).
