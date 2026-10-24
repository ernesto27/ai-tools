---
name: update-site-docs
description: Update the Soft Factory English documentation website under site-docs when CLI behavior changes, a guide needs correction, or the user requests site content or design edits. Verify examples against the implementation and validate the VitePress build. Use for the user-facing site, not standalone architecture documentation.
---

# Update the documentation site

Keep the English guide accurate and easy to use for people who want to use
Soft Factory. It coordinates implementation, reviews, risk classification,
and a final walkthrough through Agent Sandbox. Use the existing Node.js/npm
VitePress setup and custom theme.

## Establish context

Resolve the module directory containing `AGENTS.md` and `site-docs/`. The Git
root is one level above the module; confirm with `git rev-parse --show-toplevel`.
Paths below are relative to the module unless stated otherwise.

- Read `AGENTS.md`, `README.md`, and `site-docs/README.md`.
- Inspect `git status --short` and the relevant staged and unstaged diffs.
  Preserve existing edits and staging; do not stage files as part of a docs update.
- Honor the user's feature, paths, commit range, or wording. For a general
  synchronization request, compare the existing guide with current source and
  relevant changes rather than guessing from commit messages.

## Find the right files

| Purpose | Location |
| --- | --- |
| User guide pages | `site-docs/guide/*.md` |
| Overview content and landing layout | `site-docs/guide/.vitepress/theme/DocsHome.vue`, imported by `site-docs/guide/index.md` |
| Navigation, search, metadata, Pages base path | `site-docs/guide/.vitepress/config.mjs` |
| Shared light/dark styles | `site-docs/guide/.vitepress/theme/style.css` |
| Theme entry point | `site-docs/guide/.vitepress/theme/index.js` |
| Public assets | `site-docs/guide/public/` |
| Site maintenance instructions | `site-docs/README.md` |
| npm scripts and dependencies | `site-docs/package.json`, `site-docs/package-lock.json` |
| Deployment workflow | Git root: `.github/workflows/agent-sandbox-docs.yml` |

Keep site files inside `site-docs`; GitHub Actions requires its workflow at
the Git root. Generated `guide/.vitepress/dist/`, `guide/.vitepress/cache/`,
and `node_modules/` remain ignored. The ignored `docs/` directory holds
generated review reports, not website content.

## Verify behavior before describing it

Trace each affected behavior through the linked implementation and tests.
Relevant entry points include `cmd/factory` for commands and sequencing;
`internal/config` for configuration validation; `internal/taskcontext` for
supporting documents and prompts; `internal/sandbox` for execution and reports;
`internal/jira` and `internal/googledrive` for integrations; and `install.sh`
for installation. Use command help when useful; do not launch a real workflow
to validate docs.

Check requirements, defaults, configuration paths, stage selection, and
failure recovery against source. Distinguish default runs using `run.branch`
from `review` and `continue` using `resume.branch`. Reviews can apply fixes;
the final walkthrough is read-only. Check report locations and review statuses
rather than treating command success as proof that all findings were resolved.
Do not change CLI behavior to make a documentation example work.

## Make focused edits

- Write straightforward English with copyable examples for tool users.
  Explain the outcome before optional flags or edge cases. Keep installation
  focused on requirements, the installer, setup checks, and updates; omit Go
  build instructions, checksums, archive names, and release workflow internals
  unless the user explicitly requests those details.
- Update the affected pages and cross-links. Add new pages to navigation and,
  when appropriate, the Overview's chapter links.
- Preserve the existing visual direction: warm paper, charcoal terminals,
  restrained green accents, and readable typography. Change design when the
  request calls for it; ordinary content updates should not redesign the site.
- Use VitePress Markdown containers (`::: warning`), and `withBase()` for
  site links or public asset URLs constructed in Vue components. Respect the
  configured GitHub Pages subpath `/ai-tools/soft-factory/`.
- The shared `.github/workflows/agent-sandbox-docs.yml` builds both sites in one
  Pages artifact: Agent Sandbox stays at `/ai-tools/`, and Soft Factory goes
  under `/ai-tools/soft-factory/`. Preserve both sites when changing deployment;
  build both if shared workflow or Agent Sandbox files change.
- Use placeholders for secrets and host paths. Update dependencies, workflow
  settings, and maintenance instructions only when the requested change needs them.

## Validate and deliver

From `site-docs`, install with `npm ci` when dependencies are missing or the
lockfile changed, then run:

```bash
npm run docs:build
```

Resolve build failures and dead Markdown links, including missing section anchors. Check navigation and links
created in Vue components too; the Markdown link checker does not cover every
custom component link.

For design or interactive changes, preview with `npm run docs:dev` or build
first and run `npm run docs:preview`. Restart the production preview after
rebuilding so its asset metadata matches the latest output. Check desktop and
mobile layouts, light/dark modes, and affected interactions such as search and
command copying. If browser verification is unavailable, state that limitation.

Report the changed pages, relevant behavior corrections, and checks performed.
If the site already matches the requested behavior, report that finding rather
than manufacturing edits. Updating docs does not itself authorize committing,
pushing, changing GitHub settings, or deploying; follow explicit session
authorization for those actions.
