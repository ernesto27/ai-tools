# Soft Factory Desktop

A Tauri 2 desktop app for Software Factory and Agent Sandbox.

## Install a release

Download a `desktop-vX.Y.Z` release from
[GitHub Releases](https://github.com/ernesto27/ai-tools/releases).
Install `software-factory` and `agent-sandbox` separately first; the desktop
installers do not include them. Both commands must be available on the desktop
process's `PATH`. Check them from the terminal you will use to launch the app:

```bash
export PATH="$HOME/.local/bin:$PATH"
software-factory --version
agent-sandbox --version
```

The Software Factory public installer currently supports Linux only. Preparing
that CLI for macOS is separate from installing the desktop DMG.

| Computer | Download |
|---|---|
| Linux x64 | `soft-factory_X.Y.Z_linux_amd64.AppImage` |
| macOS Apple Silicon | `soft-factory_X.Y.Z_darwin_arm64.dmg` |

Download the matching `.sha256` file alongside the installer. In that directory,
verify it with `sha256sum -c FILE.sha256` on Linux or
`shasum -a 256 -c FILE.sha256` on macOS, replacing `FILE` with the full installer
filename.

### Linux

AppImages are built on GitHub's `ubuntu-latest` runner. The build environment
changes as GitHub updates that runner; compatibility with older distributions
depends on their runtime libraries and desktop environment. The initial release
targets x64 only.

For example, after downloading version 0.1.0:

```bash
chmod +x soft-factory_0.1.0_linux_amd64.AppImage
env PATH="$HOME/.local/bin:$PATH" ./soft-factory_0.1.0_linux_amd64.AppImage
```

If the system cannot mount AppImages through FUSE, use extraction mode:

```bash
APPIMAGE_EXTRACT_AND_RUN=1 ./soft-factory_0.1.0_linux_amd64.AppImage
```

### macOS

Requires an Apple Silicon (ARM64) Mac with macOS 11 or later. Open the DMG and drag
**Soft Factory** into **Applications**. Builds use an ad hoc signature without
Apple Developer credentials or notarization. After a blocked first launch, use
**System Settings > Privacy & Security > Open Anyway** and confirm opening the app.
See [Apple's instructions](https://support.apple.com/en-us/102445) if your macOS
version presents different controls.

Finder does not necessarily inherit your terminal's `PATH`. To make separately
installed CLIs available, launch the installed executable from a configured terminal:

```bash
env PATH="$HOME/.local/bin:$PATH" "/Applications/Soft Factory.app/Contents/MacOS/desktop-app"
```

## Recommended IDE Setup

- [VS Code](https://code.visualstudio.com/) + [Tauri](https://marketplace.visualstudio.com/items?itemName=tauri-apps.tauri-vscode) + [rust-analyzer](https://marketplace.visualstudio.com/items?itemName=rust-lang.rust-analyzer)

## Desktop integration tests (Linux)

Run the following commands from this directory. The suite requires Node.js
22.19 or newer, Rust, the [Tauri Linux prerequisites](https://v2.tauri.app/start/prerequisites/#linux),
and real `git`, `software-factory`, and `agent-sandbox` executables on `PATH`.

Install the test drivers and virtual display tools (Ubuntu/Debian):

```bash
cargo install tauri-driver --locked
sudo apt-get install webkit2gtk-driver xvfb xauth
export PATH="$HOME/.cargo/bin:$PATH"
```

Verify that both drivers can be found:

```bash
tauri-driver --help
WebKitWebDriver --help
```

Install the frontend and test dependencies:

```bash
npm ci
npm ci --prefix e2e
```

Run the suite using a virtual X11 display:

```bash
xvfb-run -a npm run test:desktop
```

With an existing X11 `DISPLAY`, you can run `npm run test:desktop` directly.
Keep ports 4444 and 4445 free. The suite builds and launches the actual Tauri
application automatically and checks screen navigation using temporary fixtures.
Driver logs and failure screenshots/HTML are saved under `e2e/artifacts/launch-*`.

If you see `spawnSync tauri-driver ENOENT`, install `tauri-driver` and ensure
`$HOME/.cargo/bin` is on `PATH` in the terminal running the tests. The npm
`Run npm audit for details` notice is unrelated to this missing executable.
