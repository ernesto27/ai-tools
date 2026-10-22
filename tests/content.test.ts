import { test } from "node:test";
import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import { parse } from "yaml";
import { articleSchema } from "../src/lib/schema";

const valid = {
  title: "A useful article title",
  description: "A sufficiently detailed description of an engineering article.",
  publishedAt: "2026-10-20",
  author: { name: "Maya Chen", role: "Engineer" },
  category: "Architecture",
  tags: ["system-design"],
  artwork: "systems",
};
test("validates article metadata and applies publication defaults", () => {
  const result = articleSchema.parse(valid);
  assert.ok(result.publishedAt instanceof Date);
  assert.equal(result.draft, false);
  assert.equal(result.featured, false);
});
test("rejects missing or malformed article frontmatter", () => {
  for (const override of [
    { title: "" },
    { description: "Too short" },
    { publishedAt: "not-a-date" },
    { author: {} },
    { category: "Unknown" },
    { tags: [] },
    { tags: ["Bad Tag"] },
    { artwork: "unknown" },
  ]) {
    assert.equal(
      articleSchema.safeParse({ ...valid, ...override }).success,
      false,
      JSON.stringify(override),
    );
  }
});
test("all six sample articles validate and contain meaningful sections", async () => {
  const directory = new URL("../src/content/articles/", import.meta.url);
  const files = (await readdir(directory)).filter((file) =>
    file.endsWith(".md"),
  );
  assert.equal(files.length, 6);
  const titles = new Set<string>();
  let featured = 0;
  for (const file of files) {
    const source = await readFile(new URL(file, directory), "utf8");
    const [, frontmatter, content] = source.split(/^---\s*$/m);
    const data = parse(frontmatter);
    const parsed = articleSchema.parse(data);
    assert.equal(parsed.draft, false);
    assert.ok(content.split(/\s+/).length > 350, file);
    assert.ok((content.match(/^## /gm) ?? []).length >= 3, file);
    titles.add(parsed.title);
    featured += Number(parsed.featured);
  }
  assert.equal(titles.size, files.length);
  assert.equal(featured, 1);
});
