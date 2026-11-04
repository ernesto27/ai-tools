# Installation

Soft Factory Desktop is a graphical interface for Software Factory and Agent Sandbox. Install both CLIs separately and make them available on the desktop process's `PATH`.

## Prepare the CLIs

Follow the [Software Factory installation guide](https://ernesto27.github.io/ai-tools/soft-factory/installation) and the [Agent Sandbox installation guide](https://ernesto27.github.io/ai-tools/installation). Complete the agent authentication and Docker setup described there before starting a workflow.

Check the commands in the terminal you will use to launch the app:

```bash
software-factory --version
agent-sandbox --version
agent-sandbox doctor
```

## Download the desktop app

Choose a `desktop-vX.Y.Z` release from [GitHub Releases](https://github.com/ernesto27/ai-tools/releases).

| Computer | Package |
| --- | --- |
| Linux x64 | `soft-factory_X.Y.Z_linux_amd64.AppImage` |
| macOS Apple Silicon | `soft-factory_X.Y.Z_darwin_arm64.dmg` |

### Linux

From the folder containing your download, move the AppImage to `~/Applications` and make it executable. Replace `X.Y.Z` with the downloaded version:

```bash
mkdir -p "$HOME/Applications"
mv soft-factory_X.Y.Z_linux_amd64.AppImage "$HOME/Applications/"
chmod +x "$HOME/Applications/soft-factory_X.Y.Z_linux_amd64.AppImage"
```

You can then launch the app from any directory:

```bash
"$HOME/Applications/soft-factory_X.Y.Z_linux_amd64.AppImage"
```

### macOS

Requires an Apple Silicon Mac with macOS 11 or later. Open the DMG and drag **Soft Factory** into **Applications**. Builds use an ad hoc signature without notarization. If the first launch is blocked, use **System Settings → Privacy & Security → Open Anyway**.

The Software Factory public installer currently supports Linux only. Preparing that CLI for macOS is a separate step from installing the desktop DMG.

Finder may not inherit your terminal's `PATH`. Launch from a configured terminal to make the CLIs available:

```bash
env PATH="$HOME/.local/bin:$PATH" "/Applications/Soft Factory.app/Contents/MacOS/desktop-app"
```

## Next step

[Add a repository and start your first run](./first-run).
