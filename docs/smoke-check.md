# Sandbox smoke check

Executed on 2026-11-01 in the Software Factory / agent-sandbox container, against
`desktop-app/`. Read the root README and desktop application's AGENTS/README
before checking; also inspected the sandbox AGENTS/README and Dockerfile.
The initial `git status --porcelain=v1 --untracked-files=all` was empty: no
pre-existing staged, unstaged, or untracked files. No application behavior,
Light/Dark feature, dependencies, lockfiles, or configuration were changed.
Output below is summarized; download progress and generated asset lists are omitted.

Overall: PASS for the requested compiler/linker, native dependencies, frontend
build, and locked Linux Rust check/tests. Desktop integration remains BLOCKED
and was not run, as requested. The separate all-platform offline metadata probe
was BLOCKED by an uncached Android dependency; it does not invalidate the Linux
results.

## Executed environment checks

| Result | Command/check | Evidence |
| --- | --- | --- |
| PASS | `pwd`; `id` | `/workspace`; UID 1000, GID 1000, supplementary group 1000. All checks ran as this user without sudo or switching users. |
| PASS | Python `NamedTemporaryFile(dir='/workspace')`, write/read/unlink | Worktree writable; only the owned probe file was removed. |
| PASS | Read `/etc/os-release` | Debian GNU/Linux 12 (bookworm), x86_64. |
| PASS | `bash -c 'command -v cargo; cargo --version; rustc --version'` | Exit 0; Cargo at `/usr/local/cargo/bin/cargo`; Cargo/rustc 1.99.1. |
| PASS | Same command with `bash -lc` | Exit 0; same Cargo path and versions in a Bash login shell. |
| PASS | `sh -c 'command -v cargo; cargo --version'` | Exit 0; same Cargo path and version in the normal POSIX shell. |
| PASS | Selected home variables; directory access and temporary write probes | `HOME=/tmp/agent-sandbox-home`, `CARGO_HOME=/usr/local/cargo`, `RUSTUP_HOME=/usr/local/rustup`. All three readable, searchable, and writable by UID 1000. Home owned by 1000:1000; Cargo/Rustup directories owned by 0:0 but writable by this sandbox user. |
| PASS | `rustup show active-toolchain`; `rustup which rustc`; `rustc --print sysroot`; `rustc --print target-libdir` | Each exit 0. Active `1.99.1-x86_64-unknown-linux-gnu`; compiler under `/usr/local/rustup/toolchains/1.99.1-x86_64-unknown-linux-gnu/bin/`; libraries under the same sysroot. Cargo executable resolves to the Rustup proxy. |

`CARGO_TARGET_DIR` and `RUSTUP_TOOLCHAIN` were unset. Existing configured cache
paths were used with the agent-specific home; no root home or credential files
were needed. Project crate downloads also exercised writes to the Cargo cache.

## Tools and native libraries

Every tool invocation below exited 0 (PASS).

| Command | Observed version |
| --- | --- |
| `node --version` | 24.22.2 |
| `npm --version` | 11.19.1 |
| `rustc --version` | 1.99.1 (58cf8f9c3, 2026-10-13) |
| `cargo --version` | 1.99.1 (5f94df478, 2026-08-27) |
| `rustfmt --version` | 1.10.0-stable |
| `cargo clippy --version` | 0.1.99 |
| `git --version` | 2.39.5 |
| `curl --version` | 7.88.1, OpenSSL 3.0.22 |
| `pkg-config --version` | 1.8.1 |
| `cc --version`; `c++ --version` | GCC/G++ 12.2.0 (Debian 12.2.0-14+deb12u1) |
| `make --version` | GNU Make 4.3 |
| `ld --version`; `cc -print-prog-name=ld` | GNU Binutils linker 2.40; compiler selects `ld` |
| `file --version` | 5.44 |
| `wget --version` | 1.21.3 |

PASS: committed `package-lock.json` selects Vite 8.3.5, whose Node engine range is
`^20.19.0 || >=22.12.0`; Node 24.22.2 satisfies it. Rust 1.99.1 supports the
project's edition 2024, also demonstrated by the offline compilation below.

Each `pkg-config --modversion <name>` exited 0 (PASS):

| Name | Version |
| --- | --- |
| `gtk+-3.0` | 3.24.38 |
| `webkit2gtk-4.1` | 2.50.6 |
| `openssl` | 3.0.22 |
| `librsvg-2.0` | 2.54.7 |
| `ayatana-appindicator3-0.1` | 0.5.90 |

Each package was checked separately using
`dpkg-query -W -f='${db:Status-Status} ${Version}' <package>`;
each exited 0 and reported `installed` (PASS):

| Package | Installed version |
| --- | --- |
| `build-essential` | 12.9 |
| `ca-certificates` | 20250419~deb12u2 |
| `curl` | 7.88.1-10+deb12u15 |
| `file` | 1:5.44-3 |
| `git` | 1:2.39.5-0+deb12u3 |
| `libayatana-appindicator3-dev` | 0.5.92-1 |
| `libgtk-3-dev` | 3.24.38-2~deb12u3 |
| `librsvg2-dev` | 2.54.7+dfsg-1~deb12u1 |
| `libssl-dev` | 3.0.22-1~deb12u2 |
| `libwebkit2gtk-4.1-dev` | 2.50.6-1~deb12u2 |
| `libxdo-dev` | 1:3.20160805.1-5 |
| `pkg-config` | 1.8.1-1 |
| `wget` | 1.21.3-1+deb12u1 |

No requested compilation tool or native development package was missing.

## Offline compiler, linker, and test proof

PASS, both commands exited 0 as UID 1000:

```sh
# Python tempfile.mkdtemp(prefix='tauri-rust-smoke-') created this owned directory.
cargo init --lib --name sandbox_rust_smoke --vcs none --edition 2024 /tmp/tauri-rust-smoke-rdl7ibaw
cargo test --offline --manifest-path /tmp/tauri-rust-smoke-rdl7ibaw/Cargo.toml
```

The dependency-free library compiled and linked its test executable, which ran:
`1 passed; 0 failed`; doc-tests: `0 passed; 0 failed`. Confirmed no `.git` was
created. A `finally` block removed only this owned temporary directory, including
its build output, and confirmed removal. No toolchain or system packages were
installed interactively.

## Project checks

Commands ran from `/workspace/desktop-app`, preserving both committed lockfiles.

| Result | Command | Exit/result |
| --- | --- | --- |
| PASS | `npm ci` | 0; installed 154 packages, audited 155; reported 0 vulnerabilities. |
| PASS | `npm run build` | 0; TypeScript checking and Vite 8.3.5 production build succeeded; 300 modules transformed. |
| PASS | `cargo check --locked --manifest-path src-tauri/Cargo.toml` | 0; completed the Linux `dev` check in 1m 17s with the locked dependencies. Registry access and native build scripts succeeded. |
| PASS | `cargo test --locked --manifest-path src-tauri/Cargo.toml` | 0; compiled and linked the Linux test targets in 1m 36s. Library: 38 passed, 0 failed; binary: 0 tests; doc-tests: 0 tests. |
| PASS | `cargo metadata --locked --offline --filter-platform x86_64-unknown-linux-gnu --format-version 1 --manifest-path src-tauri/Cargo.toml` | 0; 367 resolved packages; highest declared Rust requirement 1.90, including Tauri 2.12.5. Installed Rust 1.99.1 satisfies it. |

An additional, unfiltered `cargo metadata --locked --offline --format-version 1
--manifest-path src-tauri/Cargo.toml` probe was BLOCKED (exit 101):
`failed to download android_system_properties v0.1.6` because offline mode
prohibited an HTTP request. The downstream JSON parser then exited 1 on empty
input. This was a cache miss for another platform, not a network outage,
permissions problem, compiler error, or test failure. The Linux-filtered probe
above passed without downloading other-platform dependencies. Full all-platform
offline readiness is not established; populate those caches online if needed.

PASS: SHA-256 comparison after dependency installation and builds
confirmed the lockfiles still matched the initial worktree:

```text
desktop-app/package-lock.json
f57a0ae0334200bc7a16241b1723f1dc76f476b940add88080e17679684dec5b
desktop-app/src-tauri/Cargo.lock
fd6c7f8b06022252130a8b71ddb397b074f61426ddc2ae5e02f28ab9a07a78d1
```

There were no network/registry failures, missing compilation packages,
permission failures, compiler errors, or test failures in the requested builds.
The separate metadata cache miss and desktop runtime blockers are identified
individually above and below; neither is reported as a passing check.

## Desktop prerequisites and checks not run

Availability was checked individually with selected-variable inspection and
Python `shutil.which`, rather than launching workflows:

| Result | Prerequisite | Evidence |
| --- | --- | --- |
| BLOCKED | `DISPLAY` | Unset. |
| BLOCKED | `software-factory` | No executable on PATH. |
| BLOCKED | `agent-sandbox` | No executable on PATH. |
| BLOCKED | `tauri-driver` | No executable on PATH. |
| BLOCKED | `WebKitWebDriver` | No executable on PATH. |
| BLOCKED | Optional virtual display | Neither `Xvfb` nor `xvfb-run` on PATH. |

NOT RUN (desktop suite BLOCKED): `npm run test:desktop`, desktop application
launch, packaging, and UI interaction checks. The full desktop integration suite
was explicitly excluded from this smoke check. It needs a working display and
all four real executables above, in addition to compilation dependencies; a
successful build or browser mock would not establish desktop functionality.
No frontend mocks, screenshots, or video were created; this task changes no UI.

## Next steps and image lifecycle

No Dockerfile correction is indicated by the observed compiler and native
dependency checks, so no Dockerfile or `baseImage` configuration was edited.
If desktop integration is needed later, provision a display and the four real
executables in the appropriate toolchain image, then run the real suite.

Editing a Dockerfile does **not** change this running container. Any future image
fix must be built and tagged with a **new image tag**, and the host's selected
`baseImage` must be updated to that reference: agent-sandbox caches derived
images by base reference. Start a fresh sandbox and rerun shell/cache permissions,
offline Rust compilation/link/test, package/library probes, `npm ci`, frontend
build, and locked project Cargo check/test before claiming the rebuilt image
passes. Desktop validation additionally requires rechecking the display and real
executables. No image rebuild, deployment, or host configuration change was
performed here.
