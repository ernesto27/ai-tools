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

No frontend test framework, test script, or coverage threshold is configured. Run `npm run build` and manually verify affected desktop workflows. For native changes, check repository selection, duplicate handling, CLI failures, and branch loading as applicable. Add Rust unit tests in `#[cfg(test)]` modules for new testable backend logic.

Do not create ad hoc frontend `.test.mjs` files such as `tests/runProgress.test.mjs` or `tests/useDesktopRuns.test.mjs`. Do not test frontend code by reading TypeScript source, transpiling it at runtime with `ts.transpileModule`, importing generated code through data URLs, or evaluating it in a VM with hand-written React or Tauri mocks. Use the build and manual verification described above unless the user explicitly requests a properly configured frontend test framework.

## Commit & Pull Request Guidelines

History uses short, descriptive subjects such as `Load sidebar repos and branches from disk`; follow that style. Keep commits focused. PRs should explain behavior changes, link relevant issues, and list validation performed. Include screenshots for UI changes and identify any new CLI, configuration, or platform requirements.
