# Tauri + React + Typescript

This template should help get you started developing with Tauri, React and Typescript in Vite.

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
