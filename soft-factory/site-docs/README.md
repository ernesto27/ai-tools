# Soft Factory documentation

This VitePress site uses the same theme and dependency versions as Agent Sandbox.
The English guide is served at `/` and the Spanish guide at `/es/`, with a
language switcher that keeps the current page. Both use the shared custom home
component and local search, following Agent Sandbox’s locale setup.

Keep matching page filenames in both languages. Translate navigation and search
labels in `guide/.vitepress/config.mjs`, and home-page text in
`guide/.vitepress/theme/home-es.js`. Preserve explicit section anchors used by
cross-links when translating headings. Update both guides when content changes.

Use Node.js 22 or later. From `soft-factory/site-docs`:

```bash
npm ci
npm run docs:dev
npm run docs:build
npm run docs:preview
```

The production base is `/ai-tools/soft-factory/`. The shared workflow at
`.github/workflows/ai-tools-docs.yml` builds all three documentation sites,
keeps Agent Sandbox at the Pages root, and adds Soft Factory at `soft-factory/` and Desktop at `desktop/`.
Pushing a stable `agent-sandbox-vMAJOR.MINOR.PATCH` or
`software-factory-vMAJOR.MINOR.PATCH`, or `desktop-vMAJOR.MINOR.PATCH` tag deploys all sites from that tagged
commit. The `github-pages` environment must allow all three tag patterns
(`agent-sandbox-v*`, `software-factory-v*`, and `desktop-v*`) and the `master` branch.
Manual deployment from `master` is also available through
**Actions → AI Tools Documentation → Run workflow**. Branch pushes and pull
requests do not trigger this workflow. Generated files and dependencies are ignored.
