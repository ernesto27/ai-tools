# Spec: Multiple task files in Run and Continue

Status: Approved by the user; implementation requested.

## Objective

Let desktop users supply more than one task file when starting a Run or Continue
workflow. A plus icon adds a file-path row; each row provides an editable path,
Browse button, and remove icon. The displayed order is the order supplied to the
CLI through `agentSandbox.run.filePrompt`.

The confirmed intent covers Run and Continue. Resume, folder, inline task, and
Jira inputs retain their existing behavior.

## Context and assumptions

- Commit `573a633` adds multiple prompt files to agent-sandbox; `b946f79` adds
  multiple task prompt files to software-factory.
- The current desktop type, parser, and task input still treat `filePrompt` as
  a string. All must support arrays for this feature to work end to end.
- Follow the CLI's new schema: even one file is an array. Legacy strings are
  rejected with a configuration validation error, not automatically migrated.
- Start with one empty row when no files are configured. Removing the final row
  leaves one empty row so a user can always enter or browse for a file.
- Omit whitespace-only rows when serializing; preserve nonblank path strings,
  including spaces within them, repeated paths, and their relative order.
- No application-imposed file-count limit is introduced.
- Browse continues to select one file for its own row. Adding many files through
  a single picker selection and drag-and-drop are outside this change.

## Tech stack

Existing Tauri 2, React 19, TypeScript 6, Tailwind CSS 4, and react-i18next.
Use existing UI and icon patterns and the Tauri dialog plugin. No new dependency.

## User behavior and data contract

1. In the File task source, render one path row per configured file, in order.
2. Provide a plus icon with an accessible localized name such as “Add task file”.
   Clicking it appends an empty row and moves keyboard focus to its path input.
3. Each row has an editable path, its own Browse button, and a remove icon with
   an accessible localized name identifying its row.
4. Browse replaces only the chosen row. Canceling the picker changes nothing;
   dialog errors remain visible and do not discard other rows.
5. Removing a row preserves all remaining values and their order. Focus moves to
   an adjacent path input, or the retained empty input after removing the last row.
6. Preserve file entries when switching task-source tabs and when switching
   between Run and Continue for the same repository. Repository changes load that
   repository's configuration rather than retaining another repository's paths.
7. Existing loading, error, and busy-state restrictions continue to apply.
8. Configuration parsing accepts a missing `filePrompt`, an empty array, or an
   array containing only strings. Reject strings, null, and mixed-type arrays.
9. Preview and saved configuration use `filePrompt: ["task.txt", "context.md"]`.
   A single entry uses `["task.txt"]`; when no nonblank entries remain, omit the
   property. Preserve unrelated configuration properties.
10. Run and Continue use the same ordered array. Continue keeps its existing
    resume branch handling. Do not concatenate file contents in the desktop;
    the CLI owns reading and joining them, including reporting unreadable files.
11. New controls and labels support English and Argentine Spanish, keyboard
    operation, light/dark themes, and the existing responsive layout.

## Commands

Run from the desktop-app repository directory:

```bash
npm run dev
npm run build
npm ci --prefix e2e
xvfb-run -a npm run test:desktop
```

For native code changes, also run:

```bash
cargo fmt --manifest-path src-tauri/Cargo.toml -- --check
cargo test --manifest-path src-tauri/Cargo.toml
```

The desktop integration suite requires the installed real `software-factory`,
`agent-sandbox`, `tauri-driver`, and `WebKitWebDriver` executables. Browser-only
development cannot verify native dialogs or configuration persistence.

## Project structure

- `src/components/RunTaskInput.tsx`: shared task source inputs and file controls.
- `src/screens/NewRun.tsx`: Run/Continue form state and configuration loading.
- `src/config.ts`: configuration types, validation, and task serialization.
- `src/locales/en.json`, `src/locales/es-AR.json`: localized labels and errors.
- `src/components/ui.tsx`: existing UI primitives to reuse.
- `e2e/navigationChecks.js` and `e2e/navigationFixture.js`: real desktop checks
  and exclusively owned repository/configuration fixtures.
- `src-tauri/src/factory_config.rs`: existing native persistence boundary;
  change only if implementation reveals a requirement for array handling there.

## Code style

Follow the existing two-space indentation, double quotes, semicolons, explicit
TypeScript types, camelCase functions, and PascalCase React components. Keep
strict TypeScript checks passing. For example, current form state updates use:

```tsx
onChange={(source, value) => {
  setTaskValues((current) => ({ ...current, [source]: value }));
  setSaveError(null);
}}
```

Adapt types to represent files as an array rather than encoding multiple paths
in one string. Reuse existing UI primitives and icon conventions.

## Testing strategy

- Run `npm run build` for type and production build verification.
- Extend the real desktop integration suite to cover multiple configured files,
  adding/editing/removing rows, empty-row serialization, duplicates and ordering,
  task-source switches, Run/Continue switches, and configuration previews.
- Cover missing/empty arrays and invalid legacy/mixed values using real fixture
  configuration files loaded by the native backend. Verify malformed input
  produces a visible configuration error rather than crashing.
- Manually verify row-specific native Browse, cancellation, dialog failure where
  reproducible, focus behavior, narrow layouts, both languages, and both themes.
- Verify real configuration persistence through the native path without starting
  agents as part of basic navigation tests. Do not trigger a workflow merely to
  assert a preview. Report any persistence check that requires a separate run.
- Fixtures use temporary app/registry data and exclusively owned repository
  directories; clean up all owned data. Native commands must remain real.
- No frontend unit-test framework or coverage threshold is added. Do not create
  source-reading tests, runtime TypeScript transpilation tests, or hand-written
  React/Tauri mocks. Add Rust unit tests if new testable native logic is needed.

## Boundaries

- **Always:** Preserve ordering, duplicates, unrelated configuration, existing
  branch semantics, localization, and accessibility; validate the array schema;
  run relevant build/checks and report validation limits.
- **Ask first:** Add dependencies, expand to other task sources or Resume UI,
  introduce automatic legacy configuration migration, or change CLI semantics.
- **Never:** Commit secrets, modify unrelated repository work, replace native
  commands with mocks, start agents in basic navigation checks, or release/tag as
  part of this feature.

## Success criteria

- [ ] Run and Continue both show configured file arrays without validation errors.
- [ ] Plus appends a row; each Browse affects only its row; remove preserves others.
- [ ] Removing all files leaves one usable empty row and omits `filePrompt`.
- [ ] Preview and persisted JSON use arrays for one or more files, in shown order.
- [ ] Duplicate paths survive editing and serialization; blank rows are omitted.
- [ ] Switching task source or Run/Continue preserves entered file values.
- [ ] Invalid schemas show a configuration error; unrelated fields remain intact.
- [ ] Labels, focus, keyboard operation, themes, and narrow layouts are verified.
- [ ] Build passes and relevant real desktop checks pass, or blockers are reported.
- [ ] Resume and other task inputs retain their existing behavior.

## Open questions

No blocking questions. Human review must confirm the assumptions above,
particularly rejecting legacy string configuration and the final empty-row behavior.

## Implementation validation

- Implemented file-array parsing and serialization, ordered editable rows with
  stable identities, plus/remove icons, row-specific Browse, focus handling, and
  English/Argentine Spanish labels.
- Added real desktop integration checks for row operations, duplicates, blanks,
  source/mode switches, and valid/invalid schema fixtures.
- `npm run build` passes. The existing Vite bundle-size warning remains.
- `xvfb-run -a npm run test:desktop` stops at its prerequisite check because
  `xdotool` is not installed. UI, native Browse, and persistence checks have not
  been executed; no agents or workflows were started for verification.
