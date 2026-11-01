# Repository Guidelines

## Project Structure & Module Organization

Soft Factory is a Tauri 2 desktop app with a React 19, TypeScript, and Tailwind CSS frontend.

- `src/screens/` contains application screens; `src/components/` contains shared components and UI primitives.
- `src/App.tsx` coordinates navigation; `src/router.ts` defines route types; `src/data.ts` supplies mock data still used by several screens.
- `src/index.css` defines shared styles. Static assets live in `public/`; desktop icons live in `src-tauri/icons/`.
- `src-tauri/src/lib.rs` implements native commands, repository persistence, and CLI integration. Tauri settings and permissions live in `src-tauri/tauri.conf.json` and `src-tauri/capabilities/`.

## Build, Test, and Development Commands

Run frontend commands from this directory:

- `npm ci`: install dependencies using the committed lockfile.
- `npm run dev`: start Vite on port 1420 for browser development.
- `npm run tauri dev`: launch the desktop app with the Rust backend; requires Rust and platform Tauri prerequisites.
- `npm run build`: type-check TypeScript and produce frontend assets in `dist/`.
- `npm run preview`: preview the built frontend.
- `npm run tauri build`: build desktop bundles.
- `cargo test --manifest-path src-tauri/Cargo.toml`: run Rust tests when present.

Native repository workflows require `agent-sandbox` on `PATH`; browser previews cannot exercise Tauri commands.

## Coding Style & Naming Conventions

Match existing TypeScript style: two-space indentation, double quotes, semicolons, functional React components, and explicit prop types. Use PascalCase for component files and components, camelCase for functions and variables, and snake_case for Rust functions and commands. Keep TypeScript strict checks passing. Reuse `src/components/ui.tsx` primitives and existing Tailwind styling. No frontend formatter or linter is configured; use `cargo fmt --manifest-path src-tauri/Cargo.toml` for Rust.

## Testing Guidelines

No frontend unit-test framework or coverage threshold is configured. Run `npm run build` and manually verify affected desktop workflows beyond the navigation and text checks described below. For native changes, check repository selection, duplicate handling, CLI failures, and branch loading as applicable. Add Rust unit tests in `#[cfg(test)]` modules for new testable backend logic.

The Linux desktop integration suite lives in `e2e/` and uses Node's standard test
runner with WebdriverIO against the actual built Tauri application. Install its
locked dependencies with `npm ci --prefix e2e`, then run `npm run test:desktop`
with an X11 display or `xvfb-run -a npm run test:desktop`. It requires installed real
`software-factory`, `agent-sandbox`, `tauri-driver`, and `WebKitWebDriver` executables.
Keep desktop UI checks in this real integration suite; do not replace native
commands with mocks. See `e2e/launch.spec.js` for setup and
`e2e/navigationChecks.js` for current coverage.

The suite checks all screens, including New run's Run/Continue/Resume modes.
Use real Git worktrees and prepared configuration/registry/log files read by the
native backend. Keep test registry/app data temporary, create project logs only
in an exclusively owned fixture directory, and remove all owned data in teardown.
Run detail coverage opens its initial view only; do not start workflows or agents
as part of basic navigation tests.

Do not create ad hoc frontend `.test.mjs` files such as `tests/runProgress.test.mjs` or `tests/useDesktopRuns.test.mjs`. Do not test frontend code by reading TypeScript source, transpiling it at runtime with `ts.transpileModule`, importing generated code through data URLs, or evaluating it in a VM with hand-written React or Tauri mocks. Use the build and manual verification described above unless the user explicitly requests a properly configured frontend test framework.

## Commit & Pull Request Guidelines

Prefix every commit subject with `desktop:` and use a short, descriptive subject, such as `desktop: Load sidebar repos and branches from disk`. Keep commits focused. PRs should explain behavior changes, link relevant issues, and list validation performed. Include screenshots for UI changes and identify any new CLI, configuration, or platform requirements.
