import { defineConfig } from "astro/config";
import sitemap from "@astrojs/sitemap";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  site: process.env.SITE_URL || "https://fieldnotes.example",
  output: "static",
  trailingSlash: "always",
  integrations: [
    sitemap({
      filter: (page) => !new URL(page).pathname.startsWith("/search"),
    }),
  ],
  vite: { plugins: [tailwindcss()] },
});
