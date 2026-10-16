# Soft Factory documentation

This VitePress site uses the same theme and dependency versions as Agent Sandbox.
Use Node.js 22 or later. From `soft-factory/site-docs`:

```bash
npm ci
npm run docs:dev
npm run docs:build
npm run docs:preview
```

The production base is `/ai-tools/soft-factory/`. The shared workflow at
`.github/workflows/agent-sandbox-docs.yml` builds both documentation sites,
keeps Agent Sandbox at the Pages root, and adds Soft Factory at `soft-factory/`.
Pushing a stable `agent-sandbox-vMAJOR.MINOR.PATCH` tag deploys both sites from
that tagged commit. Manual deployment from `master` is also available through
**Actions → AI Tools Documentation → Run workflow**. Branch pushes and pull
requests do not trigger this workflow. Generated files and dependencies are ignored.
