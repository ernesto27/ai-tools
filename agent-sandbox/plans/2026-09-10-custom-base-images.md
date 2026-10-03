# Specification: Custom Base Images for Agent Sandbox

- **Date:** 2026-09-10
- **Domain(s):** CLI, Docker/container runtime, infrastructure
- **Status:** Implemented; RPM live validation in progress
- **Supersedes:** `2026-09-09-alpine-base-image-codex.md`, `2026-09-10-debian-ubuntu-custom-images.md`, and `2026-09-10-rpm-custom-images.md`

## 1. Goal

The Go binary can run any supported agent (`codex`, `claude`, `opencode`, or
`pi`) in an isolated worktree while using a user-selected Docker base image.
The base supplies the project's language/runtime toolchain; agent-sandbox adds
the coding-agent runtime and common CLI tools.

```text
agent-sandbox <branch> --agent <codex|claude|opencode|pi> \
  --base-image <image-reference> [--model <model>] [--push] <prompt>
```

Omitting `--base-image` preserves the existing embedded-image behavior and npm
freshness check. The image name, tag, labels, and OCI metadata are never used
to infer its operating system.

## 2. Supported images and package managers

| Family | Detection | Node source | Examples |
|---|---|---|---|
| Alpine | `apk` | Alpine packages | `golang:1.26-alpine` |
| Debian / Ubuntu | `apt-get` | NodeSource Node.js 22 | `python:3.13-bookworm`, .NET Jammy |
| Fedora / RHEL / UBI / Amazon Linux 2023 | `dnf` | NodeSource signed Node.js 22 RPM repo | `fedora:42`, RHEL 8/9, UBI 8/9, AL2023 |
| UBI minimal | `microdnf` | NodeSource signed Node.js 22 RPM repo | UBI 8/9 minimal |

The build probes in this exact order: `apk`, `apt-get`, `dnf`, `microdnf`.
Every successful branch installs Bash, CA certificates, curl, Git, ripgrep,
Node.js/npm, Codex, Claude Code, opencode, and pi.

### Exclusions and constraints

- No legacy `yum`, Amazon Linux 2, zypper, pacman, distroless images, or
  arbitrary user-provided bootstrap scripts.
- No `--base-os` option.
- RHEL images must already have enabled repositories and any required valid
  subscription. Host entitlement certificates are never mounted.
- The sandbox does not automatically enable EPEL, CRB/CodeReady Builder, or
  other supplemental repositories. Missing required packages fail the build.
- NodeSource RPM builds support only `x86_64` and `aarch64`.

## 3. Derived-image lifecycle

```text
--base-image <reference>
            │
            ▼
SHA-256(reference + generated Dockerfile)
            │
            ▼
agent-sandbox-base-<hash> cached?
       │ yes                    │ no
       ▼                        ▼
  reuse local image       build from selected base
                                  │
             ┌────────────────────┼────────────────────┐
             ▼                    ▼                    ▼
           apk                apt-get             dnf/microdnf
             └────────────────────┴────────────────────┘
                                  │
                                  ▼
                  common tools + Node.js 22 + all agents
                                  │
                                  ▼
                       run selected agent in worktree
```

`internal/sandbox/baseimage.go` embeds the generated Dockerfile. Its SHA-256
key keeps arbitrary image-reference characters out of Docker tags and makes a
bootstrap-definition change select a new derived cache image.

An existing derived image is deliberately reused without a forced pull or npm
version check. Removing its `agent-sandbox-base-*` Docker image explicitly
forces a rebuild. This is distinct from the normal embedded-image path.

## 4. Runtime contract

```text
host worktree ───────────── bind mount ───────────► /workspace
host agent configuration ── bind mount ───────────► agent-specific config
base image PATH ─────────── merged environment ───► agent shell

custom image: SHELL=/bin/bash
              XDG_CACHE_HOME=/tmp/agent-sandbox-cache
              HOME=/tmp/agent-sandbox-home  (Codex only)
```

The run configuration merges environment values, preserving a base runtime's
`PATH` (for example `/usr/local/go/bin`). The generated Dockerfile also appends
the build-time `PATH` to `/etc/profile`, so login shells retain it. Credentials
are bind-mounted from existing host agent configuration; no credential is
passed as an environment variable. Codex alone receives a temporary writable
home to avoid caches being written under `/` by the host numeric UID/GID.

## 5. Required outcomes

| Condition | Outcome |
|---|---|
| Derived cache exists | Reuse it, then create/run the isolated worktree. |
| Derived cache is absent | Build it before creating the worktree. |
| Pull, install, or npm failure | Propagate Docker's error; do not create a worktree. |
| No supported package manager | Fail with `apk, apt-get, dnf, or microdnf is required`. |
| Unsupported RPM architecture | Fail with an explicit architecture error. |
| Required RPM package/repository absent | Propagate package-manager error; add no repositories beyond NodeSource. |
| No `--base-image` | Preserve current all-agent image behavior. |

## 6. Documentation and validation

`README.md` and `../bash/README.md` document the command, OS support, cache
policy, and RHEL constraints. `data.txt` contains manual runs for Alpine,
Debian/Ubuntu, Fedora, UBI minimal, and Amazon Linux 2023.

No new automated test files are in scope by user decision. Required validation:

1. Alpine Go image with Codex and Claude: `go version`.
2. Debian/Ubuntu language base with Codex or Claude: runtime version.
3. Fedora or Amazon Linux 2023 with Codex: `node --version`.
4. UBI minimal with Claude: `node --version`.
5. A subscription-enabled RHEL 8/9 image, where available.
6. Repeated identical run: cached image is reused.
7. Unsupported base: build fails before worktree creation.

Repository checks:

```bash
gofmt -w internal/sandbox/baseimage.go
go test ./...
go vet ./...
go build ./...
```

## 7. Current implementation status

- `--base-image`, all-agent derived images, and `apk`/`apt-get`/`dnf`/`microdnf`
  branches are implemented in the Go binary only. The Bash implementation is
  deprecated and intentionally unchanged.
- The Go format, test, vet, and build checks pass.
- Fedora live validation reached its repository-metadata refresh and needs to
  complete before RPM support is considered fully validated.
- UBI minimal, Amazon Linux 2023, and configured RHEL 8/9 remain manual
  validation targets.
- Future work: additional package-manager families, configurable/mirrored Node
  sources, and automated Docker integration tests.
