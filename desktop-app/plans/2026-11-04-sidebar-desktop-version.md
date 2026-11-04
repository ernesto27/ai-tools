# Plan: Sidebar desktop version

- **Date:** 2026-11-04
- **Domain(s):** Frontend
- **Author:** plan-from-spec (reviewed with the user)
- **Status:** Draft — scope confirmed; implementation pending

## 1. Summary

Show the desktop version at the bottom of the sidebar, below Settings, so users can identify the installed release. Read the value from the root `package.json` and display it as `v{version}` using small, muted, left-aligned text. The current value renders as `v0.1.1`.

## 2. Scope

### In scope

- Add a static version label to the existing sidebar footer on every screen.
- Import the version from `package.json` at build time.
- Validate the frontend build and manually inspect the real desktop UI.

### Out of scope / non-goals

- Updating version numbers, creating tags, or publishing releases.
- Update checks, links, click actions, copy buttons, or additional version metadata.
- Native commands, permissions, dependencies, configuration changes, or routing changes.
- Automated test changes or adding a frontend test framework.

## 3. Resolved decisions

| # | Question | Decision |
|---|----------|----------|
| 1 | Which version source should the label use? | Root `package.json`, as explicitly requested. |
| 2 | What should the version look like? | `v0.1.1`, dynamically rendered as `v{version}`, small, muted, and left-aligned. |
| 3 | Where should the label appear? | In the sidebar's fixed footer, directly below Settings. |
| 4 | Should the label be interactive? | Plain, non-clickable text, confirmed in the scope read-back. |
| 5 | What verification is required? | `npm run build` and manual visual verification only, explicitly selected by the user. |
| 6 | Should releases or version values change? | No; version changes, tags, and publishing are outside scope. |
| 7 | Does the label vary across screens or themes? | Visible across screens, using existing theme-aware muted styling; verify both themes. |

## 4. Components & state

Only `src/components/Sidebar.tsx` needs an implementation change. Import the named `version` field from `../../package.json` alongside existing imports. Render it in a paragraph immediately after the Settings `NavItem`, inside the footer whose existing classes are `mt-5 flex shrink-0 flex-col gap-2 border-t border-line pt-3`.

Use `px-2 text-left text-xs text-muted` to align the label with the navigation item's left padding and reuse existing typography and theme tokens. Keep it outside the scrollable repository list so repository scrolling does not scroll the label away.

Data flow: root `package.json.version` → static JSON import → Sidebar paragraph → visible `v{version}` text. The existing `resolveJsonModule: true` in `tsconfig.json` supports the import. No component props, state, effects, runtime file reads, API calls, or new dependencies are needed.

## 5. Interfaces & contracts

- **Input:** root `package.json` contains a string `version`, currently `0.1.1`.
- **Output:** one non-interactive paragraph containing `v` followed by the imported version.
- **Public interfaces:** Sidebar props and navigation behavior remain as currently defined.
- **Accessibility and language:** ordinary readable text with no tab stop or control role. The version token contains no translatable prose and is identical in English and Spanish.
- **Errors:** malformed or missing JSON/imports are build failures; there is no runtime fetch, loading state, or fallback label.
- **Release relationship:** `.github/workflows/desktop-release.yml` in the parent repository already checks that package and native versions equal the numeric suffix of `desktop-vMAJOR.MINOR.PATCH`. Preserve that check. The label represents the package version embedded in the build; it does not query Git tags or the latest published release.

## 6. Behavior & states

- Render the label synchronously when Sidebar renders, including before repositories load and when none are configured.
- Keep it visible when navigating among Home, Add repository, Config, Branch runs, New run, Run detail, and Settings.
- Use theme-aware muted text in light and dark themes and retain the same value when switching language.
- Keep Settings clickable and repository scrolling functional.
- A subsequent build using an updated `package.json.version` picks up the new value without editing the label.
- No loading, empty, error, authorization, or persistence state is introduced.

## 7. Implementation tasks

### Task 1 — Render the package version below Settings

- **Why:** Make the desktop release identifiable from the persistent sidebar.
- **Files & changes:** Edit `src/components/Sidebar.tsx` at the import block and footer, with these exact changes:

```diff
 import { Icon } from "./Icon";
+import { version } from "../../package.json";
```

```diff
           <Icon name="settings" className="shrink-0" />
           {t("settings")}
         </NavItem>
+        <p className="px-2 text-left text-xs text-muted">v{version}</p>
       </div>
       {repoMenu && (
```

- **Depends on:** None.

### Task 2 — Verify the build and desktop presentation

- **Why:** Confirm JSON import compatibility and that the label fits the existing sidebar.
- **Files & changes:** No additional source or test files. Run the following from `desktop-app`:

```bash
npm run build
npm run tauri dev
```

- In the actual desktop app, verify the cases in section 8. Save light and dark screenshots for review or any eventual PR, following repository guidance.
- Do not start workflows or agents to check the label. Existing navigation and an initial Run detail view are sufficient.
- Record actual results and any unavailable desktop prerequisites when reporting implementation completion; do not describe unperformed visual checks as passed.
- **Depends on:** Task 1.

## 8. Testing

- **Unit tests:** Omitted. This change has no independent logic beyond displaying an imported string, the repository has no frontend unit-test framework, and the user explicitly chose build and manual verification only. Do not add source-inspection tests, runtime transpilation, mocks, or a new framework.
- **Automated integration tests:** Omitted by the user's explicit verification choice. Do not modify `e2e/` for this feature.
- **Build verification:** `npm run build` must complete successfully, checking TypeScript and the production frontend bundle.
- **Manual integration verification:** Launch the real Tauri desktop app with its real backend and installed CLI prerequisites. Compare the displayed label with root `package.json.version`; confirm one label below Settings on all existing screens, in both themes, and after a language change. Check readability and alignment at the normal window size and at the smallest allowed window size. Expand enough repositories/branches to scroll the repository list and confirm the footer remains visible without overlap. Verify the label is plain text with no click action or keyboard tab stop and Settings still opens normally. Use existing local data; do not create project fixtures or run agents for these checks. Close the development app when finished.

## 9. Acceptance criteria

1. The sidebar displays exactly `v` plus root `package.json.version` (currently `v0.1.1`). Validate by comparing the file value with the desktop UI.
2. The value is imported, with no hardcoded version literal in Sidebar. Validate the implementation diff and successful build.
3. The label appears directly below Settings in the fixed footer, small, muted, left-aligned, and non-interactive. Validate visually and with keyboard navigation.
4. Navigation, scrolling, both themes, language changes, and supported window sizes retain a readable, visible label. Validate manually in the desktop app.
5. `npm run build` passes. No version values, release tags, native code, dependencies, or automated tests change. Validate build output and the final diff.

## 10. Risks & open items

- No unresolved product or implementation decisions.
- The label contains the package version at build time. Local untagged builds can share a version with a release; Git provenance and development-build suffixes are outside scope.
- Unit and automated integration tests are intentionally omitted for the reasons in section 8. Manual desktop checks require the platform's Tauri development prerequisites and real CLI executables described in `README.md`; unavailable prerequisites must be reported as a validation limitation.
