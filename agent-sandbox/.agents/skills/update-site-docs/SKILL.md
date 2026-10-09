---
name: update-site-docs
description: Update the agent-sandbox English documentation website under site-docs when CLI behavior changes, a guide needs correction, or the user requests site content or design edits. Verify examples against the implementation and validate the VitePress build. Use for the user-facing site, not standalone architecture documentation.
---

# Update the documentation site

Keep the English guide accurate and easy to use for technical readers. The
website documents a CLI that runs on the user's local machine using Docker
and Git worktrees. Use the existing Node.js/npm VitePress setup and custom theme.

## Establish context

Resolve the module directory containing `AGENTS.md` and `site-docs/`. The Git
root is one level above the module; confirm with `git rev-parse --show-toplevel`.
Paths below are relative to the module unless stated otherwise.

- Read `AGENTS.md`, `README.md`, `docs/architecture.md`, and `site-docs/README.md`.
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
and `node_modules/` remain ignored. The architecture reference is separate
from the published site.

## Verify behavior before describing it

Trace each affected behavior through the linked implementation and tests.
Relevant entry points include `cmd/agent-sandbox` for flags, configuration,
and presentation; `internal/sandbox` for execution and publication;
`internal/agent` for authentication and models; and `install.sh` for installation.
Use command help when useful; do not launch a real sandbox to validate docs.

Check claims about requirements, defaults, configuration precedence, and
failure recovery against source. In particular, distinguish host-session
authentication from JSON API keys, new tasks from recorded-worktree resumes,
and uncommitted work from agent commits with host-side push/PR publication.
Avoid presenting a separate worktree as complete isolation of shared Git metadata.
Do not change CLI behavior to make a documentation example work.

## Make focused edits

- Write English for technical users with straightforward explanations and
  copyable examples. Explain the outcome before optional flags or edge cases.
- Update the affected pages and cross-links. Add new pages to navigation and,
  when appropriate, the Overview's chapter links. Keep the Overview's statement
  that execution happens on the user's local machine.
- Preserve the existing visual direction: warm paper, charcoal terminals,
  restrained green accents, and readable typography. Change design when the
  request calls for it; ordinary content updates should not redesign the site.
- Use VitePress Markdown containers (`::: warning`), and `withBase()` for
  site links or public asset URLs constructed in Vue components. Respect the
  configured GitHub Pages subpath.
- Use placeholders for secrets and host paths. Update dependencies, workflow
  settings, and maintenance instructions only when the requested change needs them.

## Validate and deliver

From `site-docs`, install with `npm ci` when dependencies are missing or the
lockfile changed, then run:

```bash
npm run docs:build
```

Resolve build failures and dead Markdown links. Check navigation and links
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
