# Maintaining the desktop user guide

The English documentation lives in `guide/`, with the same VitePress setup and visual style as Agent Sandbox. It uses local search and supports light and dark site themes. All desktop screenshots use dark mode.

## Preview and build

From `desktop-app/site-docs`, with Node.js 22 or newer:

```bash
npm ci
npm run docs:dev
```

Open the printed URL with `/ai-tools/desktop/` (normally `http://localhost:5173/ai-tools/desktop/`). To check Markdown links and build the production site:

```bash
npm run docs:build
npm run docs:preview
```

Generated output, VitePress caches, and dependencies are ignored. Commit the lockfile when dependencies change. The dependency versions and Vite override match the sibling documentation sites.

Navigation and the Pages base path are in `guide/.vitepress/config.mjs`. The custom landing page and styles live in `guide/.vitepress/theme/`. Keep user-facing behavior aligned with the desktop implementation and its CLI integration.

## Refresh screenshots

Use the actual desktop application and temporary fixtures. From `desktop-app`, run the existing Linux integration suite with its prerequisites installed (see the main README):

```bash
xvfb-run -a npm run test:desktop
```

The suite prints its artifact directory under `e2e/artifacts/launch-*`. After a successful run, copy these dark screenshots to `guide/public/screenshots/`:

| Test artifact | Documentation asset |
| --- | --- |
| `new-run-run-dark.png` | `new-run-dark.png` |
| `config-dark.png` | `config-dark.png` |
| `branch-runs-dark.png` | `branch-runs-dark.png` |

Inspect the images before replacing them. Keep example data free of secrets and private project content. The suite uses prepared logs and real Git worktrees; it does not launch agents or workflows. The screenshots page labels the example failed run accordingly. Keep screenshots dark even when the documentation site is viewed in light mode.

## Shared GitHub Pages deployment

The repository-root `.github/workflows/ai-tools-docs.yml` builds all three guides into one Pages artifact:

- Agent Sandbox: `/ai-tools/`
- Software Factory: `/ai-tools/soft-factory/`
- Desktop: `/ai-tools/desktop/`

Stable `agent-sandbox-v*`, `software-factory-v*`, and `desktop-v*` release tags trigger the shared workflow; it can also be run manually from `master`. The desktop guide's intended public URL is `https://ernesto27.github.io/ai-tools/desktop/` after deployment.

If the existing `github-pages` environment restricts release tags, add a separate **Tag** rule for `desktop-v*` alongside the two CLI tag rules. This repository change does not change GitHub settings or publish the site by itself. Preserve the sibling sites when editing deployment assembly or base paths, and build all three guides when changing the shared workflow.
