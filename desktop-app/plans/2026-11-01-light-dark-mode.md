# Plan: Light and Dark Mode

- **Date:** 2026-11-01
- **Domain(s):** Frontend; desktop integration verification
- **Author:** plan-from-spec
- **Status:** Approved

## 1. Summary

Add a two-state Light/Dark toggle below the app name at the top of the sidebar. Keep it visible while repositories scroll. Apply the selected appearance immediately throughout the app, preserving the current dark palette and using a white background with dark text in Light mode. Persist explicit choices locally across restarts. This document is a plan only; no application changes are included.

## 2. Scope

### In scope

- One accessible Light/Dark switch, shared across all routes and repositories.
- Shared semantic color tokens, status colors, focus styles, native controls inside the webview, and Markdown content.
- Local preference persistence, startup initialization, and system appearance tracking before an explicit choice.
- Real desktop integration checks, build validation, and manual visual verification.

### Out of scope / non-goals

- A visible System option or reset-to-System action.
- Native window decorations, file pickers, OS dialogs, and the native background before the webview loads.
- Per-repository preferences, account sync, custom palettes, theme animations, or a UI redesign.
- New packages, Rust commands, settings screens, authentication, or a frontend test framework.

## 3. Resolved decisions

| # | Question | Decision |
|---|----------|----------|
| 1 | Which appearance options? | User revised the initial three-option request to a two-state Light/Dark toggle. |
| 2 | Where is the control? | Always at the top of the sidebar; below the brand and outside the repository scroller. |
| 3 | Should the theme cover all screens and survive restarts? | Yes, explicitly confirmed. |
| 4 | What light palette? | White (`#ffffff`) background and dark text, per the latest clarification; retain the current dark palette, layout, typography, spacing, and component shapes. |
| 5 | Does this include native window appearance? | App content only; native dialogs and window chrome retain system behavior. |
| 6 | What happens before a saved choice exists? | User selected System by default, then requested creation of the file instead of answering the follow-up. **Interpretation for review:** follow OS changes until the first toggle; afterward persist an explicit Light/Dark choice with no visible System reset. |
| 7 | How is the preference stored? | Implementation decision: webview localStorage, key `soft-factory.theme`, raw values `light` or `dark`. No new backend dependencies. |
| 8 | What happens with missing, invalid, or inaccessible storage? | Implementation decision: missing/invalid values follow System; read failure also follows System. A write failure keeps the immediate choice in memory and logs a warning; persistence cannot be guaranteed in that exceptional case. |
| 9 | How is this tested? | Repository rules: build plus real desktop integration and manual verification; no ad hoc frontend unit harness or source-evaluation tests. |

Questions 1–5 are resolved by user responses. Item 6 is intentionally distinguished from a confirmed answer. Items 7–9 are recorded implementation decisions, not additional user answers. The user requested the draft file before the skill's final confirmation checkpoint.

## 4. Components & state

- `index.html`: synchronous, small startup script applies `data-theme` and `color-scheme` before the React bundle renders. Startup CSS follows that attribute instead of forcing dark content.
- `src/theme.ts`: types, persistence validation, and one DOM application function.
- `src/hooks/useTheme.ts`: mount once in `App`; own the explicit preference and OS media-query subscription. Clean up the subscription under React StrictMode.
- `src/components/ThemeToggle.tsx`: native button with `role="switch"`, stable accessible name `Dark mode`, and `aria-checked` indicating dark mode. Visible Light/Dark labels and a moving thumb expose the state without relying on color alone.
- `src/components/Sidebar.tsx`: receives the theme and callback; fixed header contains brand and toggle, while navigation entries scroll independently.
- `src/index.css`: retain dark `@theme` tokens, add a light attribute override, and remove the body-level forced dark color scheme.
- Existing `STATUS_COLOR` values already reference CSS variables, so status dots adapt without changes to `src/data.ts`.

Documentation checked: [React useEffect](https://react.dev/reference/react/useEffect) for subscription cleanup and [Tailwind theme variables](https://tailwindcss.com/docs/theme) for semantic CSS tokens. Context7 tools were unavailable; official documentation was used directly.

## 5. Interfaces & contracts

```ts
export type Theme = "light" | "dark";
export const THEME_KEY = "soft-factory.theme";
export function readThemePreference(): Theme | null;
export function applyTheme(theme: Theme): void;
// Hook result:
// { theme: Theme; toggleTheme: () => void }
// ThemeToggle props:
// { theme: Theme; onToggle: () => void }
```

- `null` preference means implicit System mode; it is never written as a storage value.
- The toggle always switches the currently resolved theme and makes the new choice explicit.
- `html[data-theme]` always contains `light` or `dark`; `html.style.colorScheme` matches it.
- Storage errors do not interrupt navigation, repository workflows, or theme switching.
- Preference scope is this app's webview profile on this device, not the repository.

## 6. Behavior & states

| State/event | Result |
|-------------|--------|
| Startup, saved `light` or `dark` | Apply that choice before React renders. |
| Startup, missing/invalid/unreadable storage | Resolve `prefers-color-scheme: dark`; otherwise Light. |
| System changes, no explicit choice | Immediately update the resolved theme. |
| Toggle from Light | Apply and save Dark. |
| Toggle from Dark | Apply and save Light. |
| System changes after toggle | Keep the explicit choice. |
| Navigation, active run, or repository changes | Preserve theme and existing route/form/run state. |
| Write fails | Keep choice for this session; log warning; later startup resolves from whatever storage remains. |
| Restart | Restore saved explicit choice. |

No cross-window synchronization is needed for the current single-window configuration. No changes to CLI contracts or permissions are needed.

## 7. Implementation tasks

### Task 1 — Define preference handling and startup appearance

- **Why:** Establish one preference contract and avoid an incorrect web-content palette during startup.
- **Files & changes:**

`src/theme.ts` (new, full contents):

```ts
export type Theme = "light" | "dark";

export const THEME_KEY = "soft-factory.theme";

export function readThemePreference(): Theme | null {
  try {
    const value = window.localStorage.getItem(THEME_KEY);
    return value === "light" || value === "dark" ? value : null;
  } catch {
    return null;
  }
}

export function applyTheme(theme: Theme): void {
  document.documentElement.dataset.theme = theme;
  document.documentElement.style.colorScheme = theme;
}
```

`index.html` (replace its existing inline background style, before the bundle):

```diff
-    <style>html,body{background:#111110}</style>
+    <script>
+      (() => {
+        let saved = null;
+        try { saved = localStorage.getItem("soft-factory.theme"); } catch {}
+        const theme = saved === "light" || saved === "dark"
+          ? saved
+          : window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
+        document.documentElement.dataset.theme = theme;
+        document.documentElement.style.colorScheme = theme;
+      })();
+    </script>
+    <style>
+      html,body{background:#111110}
+      html[data-theme="light"],html[data-theme="light"] body{background:#ffffff}
+    </style>
```

Keep startup validation in sync with `readThemePreference`; the deliberate tiny duplication avoids making the first appearance wait on the module graph. Current Tauri CSP is null; a future restrictive CSP must account for this inline script.

- **Depends on:** —

### Task 2 — Add the light palette

- **Why:** All existing semantic utilities and inline status tokens must update together.
- **Files & changes:** `src/index.css`, after the existing `@theme` block and before `@layer base`:

```css
:root[data-theme="light"] {
  --color-canvas: #ffffff;
  --color-ink: #242424;
  --color-ink-strong: #111110;
  --color-muted: #626262;
  --color-faint: #737373;
  --color-line: #dedede;
  --color-field-line: #b3b3b3;
  --color-field: #ffffff;
  --color-panel: #f5f5f5;
  --color-selected: #e8e8e8;
  --color-hover: #f0f0f0;
  --color-ok: #247346;
  --color-bad: #b33328;
  --color-warn: #875800;
  --color-info: #285db0;
}
```

In the existing base `body` rule:

```diff
-    color-scheme: dark;
```

Retain all existing dark values, typography, sizing, focus rules, and configuration styles. The existing neutral SVG select chevron remains usable in both themes; inspect it during visual QA. Validate light text, focus, and status contrast on canvas/panel/selected surfaces, adjusting only light palette values if necessary.

- **Depends on:** 1

### Task 3 — Own theme state and render the switch

- **Why:** Keep route-independent state and clean OS subscriptions.
- **Files & changes:**

`src/hooks/useTheme.ts` (new, full contents):

```ts
import { useEffect, useState } from "react";
import { applyTheme, readThemePreference, THEME_KEY, type Theme } from "../theme";

export function useTheme() {
  const [preference, setPreference] = useState<Theme | null>(readThemePreference);
  const [systemTheme, setSystemTheme] = useState<Theme>(() =>
    window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light",
  );
  const theme = preference ?? systemTheme;

  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const update = () => setSystemTheme(media.matches ? "dark" : "light");
    media.addEventListener("change", update);
    update();
    return () => media.removeEventListener("change", update);
  }, []);

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  const toggleTheme = () => {
    const next: Theme = theme === "dark" ? "light" : "dark";
    applyTheme(next);
    setPreference(next);
    try {
      window.localStorage.setItem(THEME_KEY, next);
    } catch (error) {
      console.warn("Could not save appearance preference", error);
    }
  };

  return { theme, toggleTheme };
}
```

`src/components/ThemeToggle.tsx` (new, full contents):

```tsx
import type { Theme } from "../theme";
import { cx } from "./ui";

interface Props {
  theme: Theme;
  onToggle: () => void;
}

export function ThemeToggle({ theme, onToggle }: Props) {
  const dark = theme === "dark";
  return (
    <button
      type="button"
      role="switch"
      aria-label="Dark mode"
      aria-checked={dark}
      onClick={onToggle}
      className="flex min-h-9 w-full items-center justify-between gap-2 rounded-md px-2 text-sm hover:bg-hover"
    >
      <span className={dark ? "text-muted" : "text-ink"}>Light</span>
      <span aria-hidden="true" className="flex h-5 w-9 shrink-0 items-center rounded-full border border-field-line bg-selected p-0.5">
        <span className={cx("size-3 rounded-full bg-ink", dark && "translate-x-4")} />
      </span>
      <span className={dark ? "text-ink" : "text-muted"}>Dark</span>
    </button>
  );
}
```

`src/App.tsx`, import block and beginning of `App`:

```diff
+import { useTheme } from "./hooks/useTheme";
 
 function App() {
+  const { theme, toggleTheme } = useTheme();
```

Add `theme={theme} onToggleTheme={toggleTheme}` to the existing `<Sidebar ... />` invocation; retain every existing prop.

`src/components/Sidebar.tsx`, imports, props, and signature:

```diff
+import type { Theme } from "../theme";
+import { ThemeToggle } from "./ThemeToggle";
 
 interface Props {
+  theme: Theme;
+  onToggleTheme: () => void;
```

Add `theme, onToggleTheme` to the existing destructured Sidebar argument. At the opening navigation markup, replace:

```diff
-    <nav aria-label="Main" className="flex w-nav flex-none flex-col gap-0.5 overflow-y-auto border-r border-line px-4 py-7 text-base">
-      <span className="px-2 pb-7 font-mono text-md font-medium">soft·factory</span>
+    <nav aria-label="Main" className="flex w-nav flex-none flex-col overflow-hidden border-r border-line px-4 py-7 text-base">
+      <div className="flex shrink-0 flex-col gap-3 pb-5">
+        <span className="px-2 font-mono text-md font-medium">soft·factory</span>
+        <ThemeToggle theme={theme} onToggle={onToggleTheme} />
+      </div>
+      <div className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto">
```

Close this new scroller `</div>` immediately after the existing `removeError` paragraph, before `{repoMenu && (...)}`. Keep the context menu outside the scroller. Existing navigation and repository actions remain unchanged.

- **Depends on:** 1, 2

### Task 4 — Extend the real desktop suite

- **Why:** Exercise the actual React app in Tauri with real storage and existing native workflows.
- **Files & changes:**

`e2e/themeChecks.js` (new, full contents):

```js
import assert from "node:assert/strict";
import { join } from "node:path";

export async function runThemeChecks(browser, artifacts) {
  const key = "soft-factory.theme";
  const original = await browser.execute((name) => localStorage.getItem(name), key);
  const state = () => browser.execute(() => ({
    theme: document.documentElement.dataset.theme,
    background: getComputedStyle(document.body).backgroundColor,
    scheme: getComputedStyle(document.documentElement).colorScheme,
  }));
  const ready = async () => {
    await (await browser.$('nav [role="switch"][aria-label="Dark mode"]'))
      .waitForDisplayed({ timeout: 15_000 });
  };
  const expectTheme = async (theme) => {
    await browser.waitUntil(async () => (await state()).theme === theme,
      { timeout: 5_000 });
    assert.equal((await state()).scheme, theme);
    const toggle = await browser.$('nav [role="switch"][aria-label="Dark mode"]');
    assert.equal(await toggle.getAttribute("aria-checked"), String(theme === "dark"));
    assert.equal((await state()).background,
      theme === "dark" ? "rgb(17, 17, 16)" : "rgb(255, 255, 255)");
  };
  try {
    await ready();
    await browser.execute((name) => localStorage.setItem(name, "dark"), key);
    await browser.refresh();
    await ready();
    await expectTheme("dark");
    await browser.saveScreenshot(join(artifacts, "theme-dark.png"));

    await (await browser.$('nav [role="switch"][aria-label="Dark mode"]')).click();
    await expectTheme("light");
    assert.equal(await browser.execute((name) => localStorage.getItem(name), key), "light");
    await browser.saveScreenshot(join(artifacts, "theme-light.png"));
    await browser.refresh();
    await ready();
    await expectTheme("light");

    await (await browser.$('nav [role="switch"][aria-label="Dark mode"]')).click();
    await expectTheme("dark");
    assert.equal(await browser.execute((name) => localStorage.getItem(name), key), "dark");
    await browser.refresh();
    await ready();
    await expectTheme("dark");

    for (const invalid of [null, "invalid"]) {
      await browser.execute((name, value) => {
        if (value === null) localStorage.removeItem(name);
        else localStorage.setItem(name, value);
      }, key, invalid);
      await browser.refresh();
      await ready();
      const expected = await browser.execute(() =>
        matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
      await expectTheme(expected);
    }
  } finally {
    await browser.execute((name, value) => {
      if (value === null) localStorage.removeItem(name);
      else localStorage.setItem(name, value);
    }, key, original);
    await browser.refresh();
    await ready();
  }
}
```

`e2e/launch.spec.js`, import block:

```diff
+import { runThemeChecks } from "./themeChecks.js";
```

Immediately before its existing `await runNavigationChecks(...)` call:

```js
await runThemeChecks(browser, artifacts);
```

Inside `e2e/navigationChecks.js`, in the existing `screen` helper immediately after `await check();`, add the following to exercise both palettes on every existing screen without restarting or mocking native commands:

```js
const toggle = await browser.$('nav [role="switch"][aria-label="Dark mode"]');
const startingTheme = await browser.execute(() => document.documentElement.dataset.theme);
for (const theme of ["light", "dark"]) {
  if (await browser.execute(() => document.documentElement.dataset.theme) !== theme) {
    await toggle.click();
  }
  await browser.waitUntil(async () =>
    await browser.execute(() => document.documentElement.dataset.theme) === theme,
    { timeout: 5_000 });
  await browser.saveScreenshot(join(artifacts,
    `${name.toLowerCase().replace(/[^a-z0-9]+/g, "-")}-${theme}.png`));
}
if (await browser.execute(() => document.documentElement.dataset.theme) !== startingTheme) {
  await toggle.click();
}
```

The existing suite already isolates app data in a temporary fixture and cleans it up. Preserve its real CLI requirements, diagnostics, and driver cleanup. This extends that suite instead of introducing frontend mocks or another driver instance.

- **Depends on:** 3

## 8. Testing

### Unit tests

No new frontend unit tests: repository instructions prohibit ad hoc source-transpilation/mock harnesses, and no frontend unit framework is configured. For this small presentation feature, the real desktop checks cover preference validation, resolution, persistence across webview reload, and switch state through the running app. No Rust logic changes require Rust unit tests. Introducing a properly configured frontend unit framework would be separate scope requiring user direction.

### Integration tests

- `npm run build`: strict TypeScript checks and production CSS generation.
- `npm ci --prefix e2e` if dependencies are not installed; `xvfb-run -a npm run test:desktop` on Linux (or run directly on X11).
- Real executables required: `software-factory`, `agent-sandbox`, `tauri-driver`, `WebKitWebDriver`, and git; Node >=22.19. No mocked native commands.
- Automated: saved Dark/Light, toggle in both directions, DOM switch semantics, body palette, native-control color scheme, webview reload persistence, missing/invalid storage, all existing route checks with screenshots in both palettes.
- Existing failure screenshot/page-source capture and fixture cleanup remain intact.

### Manual verification

- On the actual desktop app, exit and relaunch with Light saved, then Dark saved. Webview reload tests alone do not prove process-restart persistence.
- With no stored preference, change OS appearance while the app runs; verify live following. Toggle once, change OS again, and verify the explicit choice wins. Use real OS settings, not synthetic media-query mocks.
- Check keyboard Tab focus, Space/Enter activation, announced `Dark mode` switch state, and visible Light/Dark labels.
- With many repositories and at the minimum 960×640 window size, scroll the repository list and confirm the toggle stays visible and the context menu still positions correctly.
- Review the per-screen screenshots, including form fields, logs, Markdown, disabled controls, error/status colors, selections, focus rings, and hover states. Inspect the walkthrough with real available content if the automated fixture does not contain it.
- Toggle while editing configuration or a new-run form and while viewing a live run. Verify no route changes, lost edits, run cancellation, or native workflow regressions.
- Check first web-content appearance on a saved Light launch. A native dark frame before webview rendering remains outside scope.
- Review light palette contrast (normal text 4.5:1; large text and meaningful UI indicators 3:1). Existing dark palette is retained; any pre-existing contrast issue should be reported separately.
- Storage failure paths: inspect catch behavior in review; use actual storage restrictions if available. Do not monkey-patch storage/React/Tauri to manufacture coverage.

## 9. Acceptance criteria

1. A two-state toggle is visible below the brand at the sidebar top on every route and remains visible while repositories scroll (desktop/manual).
2. Switching immediately updates all app surfaces, text, status colors, and form controls without changing the current screen or data (integration/screenshots/manual).
3. Light uses a white background and dark text without changing the existing layout, typography, spacing, or component shapes; Dark retains the current palette (CSS diff, palette assertions, visual review).
4. Explicit preference survives webview reload and a full app restart (integration plus desktop relaunch).
5. Initial System resolution and live changes work until the first explicit choice, subject to review of decision 6 (integration startup and manual OS change).
6. Keyboard and assistive semantics expose a named switch with accurate state (DOM assertions and manual keyboard/accessibility review).
7. The build and real desktop suite pass, and screenshots from both themes are retained for review.

## 10. Risks & open items

- **One product interpretation for review:** System until the first toggle, then an explicit persistent choice; there is no way to return to System from this two-state UI. The user asked for the file before answering that follow-up. Resolve this before implementing Task 1 if that behavior is disputed.
- Native window background remains configured as `#111110` in `src-tauri/tauri.conf.json`; a dark frame before webview rendering is outside this app-content-only scope.
- The light palette is a concrete proposed design, subject to contrast and visual verification during implementation. No screenshot approval has been claimed.
- Storage availability determines persistence. Failed writes preserve only the session choice and produce a console warning.
- Unit coverage is deliberately omitted under repository testing rules; real desktop tests and manual OS/restart checks supply coverage instead.
- Current automated coverage is Linux only; visually verify other supported platforms where available. Native system-following behavior is verified manually rather than via mocked media queries.
- Re-read the current sidebar and e2e files before applying the diffs: other work changed the integration suite during planning. Preserve unrelated changes.
