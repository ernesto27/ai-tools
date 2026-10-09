# Maintaining the user guide

The English user-facing website lives in `site-docs/guide/` and is built with
[VitePress](https://vitepress.dev/).
`../docs/architecture.md` remains a separate developer reference and is not
included in the published website.

Site content, configuration, npm dependencies, and generated files are contained
in `site-docs`. The deployment workflow stays in the repository root's
`.github/workflows/` directory because GitHub Actions discovers workflows there.

## Preview and check locally

From the `agent-sandbox` module directory, with Node.js 22 or newer and npm:

```bash
cd site-docs
npm ci
npm run docs:dev
```

Open `http://localhost:5173/ai-tools/` (or the address printed in the terminal).
The preview updates automatically when you edit a page. Press Ctrl+C to stop it.
To validate Markdown links and build the production website:

```bash
npm run docs:build
npm run docs:preview
```

The second command serves the built website at the address printed in the
terminal, normally `http://localhost:4173/ai-tools/`.
Output goes to the ignored `guide/.vitepress/dist/` directory within `site-docs`.
Edit `guide/.vitepress/config.mjs` to change navigation or appearance.
The custom theme lives in `guide/.vitepress/theme/`: `style.css` defines the
shared visual style, and `DocsHome.vue` defines the landing page. Guide pages
remain Markdown files. The terminal mark is in `guide/public/mark.svg`.
Keep examples aligned with CLI flags and behavior; check `--help` and source
when updating documentation. Commit `package-lock.json` when dependencies change
so local and CI builds use the same versions.

The Vite override in `package.json` replaces VitePress 1.x's older Vite 5
dependency with a patched release. Keep it until VitePress supports a patched
version directly, and verify both the build and preview when updating it.

## Enable GitHub Pages

The repository has one Pages site. This workflow publishes the guide at
`https://ernesto27.github.io/ai-tools/`, using the repository's project URL.
Check for an existing Pages site before enabling or replacing a deployment.

1. Commit the documentation and `.github/workflows/agent-sandbox-docs.yml`,
   then push or merge them into `master`.
2. In `ernesto27/ai-tools`, open **Settings → Pages → Build and deployment**
   and select **GitHub Actions** as the source.
3. Open **Actions → Agent Sandbox Documentation → Run workflow**, choose
   `master`, and run it if the initial push ran before Pages was enabled.
4. Wait for `build` and `deploy` to succeed. The `github-pages` deployment
   environment reports the published URL.

This uses GitHub's [custom Pages workflow](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages).
No personal server or separate hosting account is needed. If environment
protection rules require approval, approve the deployment in GitHub.

The workflow checks for dead Markdown links and builds on relevant pull requests. It deploys
only from `master`, on relevant pushes or manual runs. Pull requests never
receive Pages deployment permissions. The uploaded artifact contains only
the generated website, not repository files or local configuration.

If the repository moves or uses a custom domain, update `base` and the sitemap
hostname in `guide/.vitepress/config.mjs`. If the default branch changes, update the
workflow and the edit-link pattern.
