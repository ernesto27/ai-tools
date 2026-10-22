# Fieldnotes

An independent software engineering journal built with Astro, TypeScript, and Tailwind CSS. Static HTML, local Markdown content, self-hosted fonts, and a small amount of JavaScript for search and theme preferences. No account, database, or external CMS is required.

The existing [Agent Sandbox](agent-sandbox/) and [Soft Factory](soft-factory/) CLI projects remain in their original directories and have independent setup instructions.

## Run locally

Use Node.js 22.22 or newer and npm 9.6.5 or newer. Astro is pinned to 7.4.1 to support this Node version; use the committed lockfile for reproducible installs.

```sh
npm ci
npm run dev
```

Open `http://localhost:4321`. Other commands:

| Command           | Purpose                                                            |
| ----------------- | ------------------------------------------------------------------ |
| `npm run check`   | Sync collections and check Astro and TypeScript                    |
| `npm test`        | Test search, combined category filters, and frontmatter validation |
| `npm run build`   | Validate content and generate the static site in `dist/`           |
| `npm run preview` | Serve the production build locally                                 |

In a restricted container with an unwritable home directory, use a writable npm cache (`npm ci --cache /tmp/fieldnotes-npm-cache`) and set `ASTRO_TELEMETRY_DISABLED=1` before Astro commands.

## Configure your publication

Set your production origin when building:

```sh
SITE_URL=https://your-journal.com npm run build
npm run preview
```

The default `https://fieldnotes.example` is a placeholder. `SITE_URL` controls canonical URLs, social URLs, RSS links, structured data, and the sitemap. Deploy the contents of `dist/` to any static host, configure it to serve `404.html` for missing routes, and preserve trailing slashes. No server adapter is needed. Host at the domain root; subpath hosting is not configured.

- `astro.config.mjs`: origin, static output, sitemap, and Tailwind's Vite plugin.
- `src/layouts/BaseLayout.astro`: publication identity, navigation, SEO defaults, and persistent theme control.
- `src/styles/global.css`: typography, responsive layout, light/dark tokens, and article artwork.
- `public/og-image.png`: 1200×630 social card; `og-image.svg` is its editable source.
- `src/pages/rss.xml.ts` and `src/pages/robots.txt.ts`: generated feed and crawler instructions.

The first visit follows the operating system's theme. The theme button saves an explicit preference in local storage; it persists across pages and reloads. Storage-disabled browsers still support the theme button for the current page.

## Add an article

Create a lowercase, hyphenated Markdown file in `src/content/articles/`, such as `debugging-with-intent.md`. Its filename becomes `/articles/debugging-with-intent/`.

```md
---
title: "Debugging with intent and evidence"
description: "A practical approach to tracing a failure, testing a hypothesis, and making the next incident easier to understand."
publishedAt: 2026-10-22
author:
  name: Maya Chen
  role: Staff software engineer
category: Development
tags: [debugging, developer-experience]
artwork: typescript
featured: false
draft: false
---

Start with the problem and why the reader should care.

## Follow the evidence

Write the article using ordinary Markdown. Use fenced code blocks with a
language name for syntax highlighting.

## Keep the lesson useful

Second-level and third-level headings appear in the table of contents.
```

Frontmatter is validated by the shared Zod schema in `src/lib/schema.ts`, used by Astro's Content Collections (`src/content.config.ts`) and tests:

| Field         | Requirements                                                                                                |
| ------------- | ----------------------------------------------------------------------------------------------------------- |
| `title`       | 10–110 characters                                                                                           |
| `description` | 40–200 characters; used in cards and SEO                                                                    |
| `publishedAt` | Valid date, preferably `YYYY-MM-DD`; displayed in UTC                                                       |
| `author`      | `name` and `role`, each at least two characters                                                             |
| `category`    | `Architecture`, `Development`, `Engineering culture`, or `Performance`                                      |
| `tags`        | 1–5 lowercase, hyphenated tags, such as `system-design`                                                     |
| `artwork`     | `systems`, `typescript`, `testing`, `performance`, `reviews`, or `simplicity`                               |
| `featured`    | Optional boolean, defaults to `false`                                                                       |
| `draft`       | Optional boolean, defaults to `false`; drafts are excluded from all public routes, search, RSS, and sitemap |

Use one featured article. If several are featured, the newest is selected; if none are featured, the newest published article is selected. Publication dates control sorting, not scheduling: future-dated articles are published unless marked as drafts. Reading time is estimated at 200 words per minute, excluding fenced code. Category pages, tag pages, RSS, and the sitemap update on each build.

To add a category, extend the schema enum and the description map in `src/pages/topics.astro`. Tags are generated automatically. Articles use Markdown, without MDX or executable content.

Run `npm run check`, `npm test`, and `npm run build` before publishing. The sample-content test intentionally checks the six initial articles; update its expected count and featured count when adding content.

## Search and accessibility

Search matches titles, descriptions, and tags, case-insensitively, and requires every query term to match. Hyphenated tags also match their space-separated form. Category selection combines with the search query. Shareable `q` and `category` URL parameters preserve a search on reload. `/` opens search from other pages and focuses its field on the search page. Without JavaScript, all article links and topic archives remain available.

The site includes semantic landmarks, a skip link, visible keyboard focus, labelled controls, live search result counts, reduced-motion support, responsive reading layouts, and light/dark themes. Search and 404 pages request `noindex`; article pages include canonical, Open Graph, Twitter, and BlogPosting metadata. The sample contributors are fictional.
