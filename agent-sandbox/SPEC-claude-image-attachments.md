# Claude image attachments

## Objective

Make `--image` deliver visual image input to Claude Code, using the same
user-facing flag, validation, sandbox mount layout, and ordering already used
by Codex. A user must be able to run:

```bash
agent-sandbox -a claude --image mockup.png -- "implement this UI"
```

The image must reach Claude as visual context, not merely as an inaccessible
host filename or text-only description. Claude Code documents that an image
path in its prompt is image input, and that multiple images can be used in one
conversation. <https://code.claude.com/docs/en/common-workflows>

## Commands

`--image <file>` remains an optional, repeatable root/run flag. No new
subcommand or Claude-specific flag is introduced.

For `-a claude`, each supplied image path must be validated before worktree
creation, just as it is for Codex:

- resolve it to an absolute host path;
- require an existing regular file;
- return the existing `UsageError` for a missing file, directory, or other
  non-regular path;
- preserve the order in which repeated `--image` flags were given.

Codex behavior, including its `-i` arguments and `--` terminator, must remain
unchanged. OpenCode and Pi continue accepting but ignoring `--image`, including
their current behavior for nonexistent image paths.

## Project structure and implementation

### Agent capability

Add an image-attachment capability to `internal/agent.Agent`, rather than
checking agent names independently in sandbox option parsing and Docker mount
construction. Codex and Claude report support; OpenCode and Pi report no
support. The capability controls both validation and image bind mounts.

### Input validation and bind mounts

In `internal/sandbox/options.go`, resolve image paths when the selected agent
reports image support. Keep `resolveImagePaths` and its current error wording.

In `internal/sandbox/sandbox.go`, mount images only for agents with that
capability. Reuse the current read-only mount convention and generated,
host-path-independent container names:

```text
/agent-sandbox-images/1.png
/agent-sandbox-images/2.jpg
```

The mount directory is outside the mounted Git worktree. Image files remain
read-only and are never copied into, staged from, or committed by the
worktree. A comment on the shared helper should describe the general sandbox
attachment rule, not Codex alone.

### Claude invocation

Keep Claude's existing CLI flags, model selection, permission mode, and
no-image prompt byte-for-byte unchanged. When image paths exist, construct the
single Claude prompt argument as:

```text
Analyze these attached images:
/agent-sandbox-images/1.png
/agent-sandbox-images/2.jpg

<the existing full prompt>
```

The paths must appear in image-flag order before the task prompt. This follows
Claude Code's documented image-path prompt workflow while avoiding a new,
undocumented CLI flag. The prompt passed here is already `Options.FullPrompt()`
so the existing sandbox house rules remain after the user task.

## Code style

Follow the repository's existing dependency direction: agent capability and
Claude prompt construction stay in `internal/agent`; path resolution and
Docker mounting stay in `internal/sandbox`. Use comments to explain why image
paths are generated and read-only rather than exposing host paths. Do not add
an agent-specific special case elsewhere once the capability exists.

## Testing strategy

Add focused unit coverage without Docker:

- Claude arguments without images are unchanged.
- Claude arguments with one and multiple images retain all existing flags and
  put the explicit image-path preamble before the full task prompt, in order.
- `NewOptions` resolves valid Claude image paths and rejects invalid Claude
  image paths with `UsageError`.
- OpenCode and Pi still leave `--image` input untouched and unvalidated.
- The shared image-mount test verifies Claude gets the same read-only mount
  paths as Codex, while unsupported agents receive none.
- Existing Codex argument and validation tests remain to guard against
  regressions.

Update `test-worktree.sh` so its image smoke test covers both supported agents
behind explicit opt-in environment variables. It must create a valid small PNG
rather than an empty temporary file, pass it to a Claude run using `--image`,
and ask Claude to use the visual input before it creates its sentinel file.
Keep the Codex image run opt-in as well. Do not add push coverage.

## Documentation

Update the Spanish `README.md`:

- describe `--image` as supported by Codex and Claude Code;
- say validation applies to those agents, while OpenCode and Pi ignore it;
- retain the `--` guidance needed for Codex;
- add or revise an example showing Claude with one or more images.

## Boundaries

This change does not add image support to OpenCode or Pi, create a general
attachment API beyond images, change credential mounts, alter Docker image
building, or change normal/resume worktree behavior. It does not inspect image
MIME types or decode image content before Docker; the established contract is
an existing regular file and the agent CLI handles supported image formats.

## Success criteria

1. The documented Claude command accepts one or more `--image` files and
   passes their generated container paths to Claude before the task prompt.
2. Claude receives those files through read-only mounts and can use them as
   visual context.
3. Invalid Claude image paths fail before a worktree or container is created.
4. Codex's current image invocation remains identical.
5. OpenCode and Pi retain their current ignore behavior.
6. Unit tests, `go vet ./...`, and the opt-in manual script paths pass.
