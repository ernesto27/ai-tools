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
A push to `master` affecting either site triggers deployment; pull requests
build both sites without deploying. Generated files and dependencies are ignored.
