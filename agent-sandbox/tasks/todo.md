# Tasks: macOS release installation

Status: Draft for review before implementation.
Spec: [macOS releases](../SPEC-macos-releases.md).
Plan: [implementation plan](plan.md), approved 2026-10-13.

- [ ] Task 1: Support macOS in the shared installer.
  - Acceptance: Map Linux x86_64, Darwin x86_64, and Darwin arm64 to their
    specified assets. Use available SHA-256 tools and Bash-3.2-compatible
    constructs. Preserve cleanup, stable release selection, INSTALL_DIR,
    checksum-before-install ordering, and PATH guidance.
  - Verify: `bash -n install.sh`; table-driven installer fixtures cover supported
    and unsupported platforms, both checksum tools, malformed/mismatched
    checksums, preservation of an existing binary on verification failure,
    and numeric selection of stable project tags. Run `go test ./...`.
  - Files: `install.sh`, a module-root installer test file.

- [ ] Task 2: Build and verify candidates for all three release targets.
  - Acceptance: Native Linux amd64, macOS amd64, and macOS arm64 jobs assert
    host architecture, run existing checks, build versioned binaries, execute
    candidate help/version checks, and upload each archive and checksum.
    Publication requires all jobs and publishes exactly the six required files.
  - Verify: Validate workflow YAML and review dependencies, platform mapping,
    archive names, and job permissions. Run `go build ./...`, `go vet ./...`,
    and `go test ./...`. Native checks must pass in GitHub Actions when run.
  - Files: `../.github/workflows/agent-sandbox-release.yml`.

- [ ] Task 3: Check public curl installation after publication.
  - Acceptance: Clean native jobs for all three targets execute the documented
    public curl installer, assert installed command resolution, and run
    `agent-sandbox -h` plus an exact current-tag version assertion. Use stock
    macOS Bash/checksum tools, bounded timeouts, and propagated failures.
  - Verify: Validate workflow syntax and ordering locally. Confirm a tagged
    GitHub Actions run passes the actual public-download checks; mark that
    external evidence pending until it exists. Do not tag or publish as part
    of implementing this task without a separate user release instruction.
  - Files: `../.github/workflows/agent-sandbox-release.yml`.

- [ ] Task 4: Align installation documentation and update help.
  - Acceptance: Describe all three targets consistently. Explain that the
    installer creates `~/.local/bin`; show macOS zsh users how to save the PATH
    export in `~/.zshrc`. Correct the update command's Linux-only help and the
    stale release description in agent guidance. No automatic profile edits.
  - Verify: Compare documented commands with installer and CLI behavior; run
    Go checks and the documented site build command after reading the site
    documentation skill. Review the final diff before merge.
  - Files: `README.md`, `site-docs/guide/installation.md`,
    `cmd/agent-sandbox/update.go`, `AGENTS.md`.

Implementation order: 1 → 2 → 3 → 4. Keep unrelated working-tree changes intact.
Local completion and tagged-release CI evidence must be reported separately.
