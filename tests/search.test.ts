import { test } from "node:test";
import assert from "node:assert/strict";
import { matchesArticle } from "../src/lib/search";

const article = {
  title: "TypeScript at the boundaries",
  description: "Make external data safe with runtime validation.",
  tags: ["api-design", "typescript"],
  category: "Development",
};
test("searches titles, descriptions, and tags without case sensitivity", () => {
  for (const query of [
    "TYPESCRIPT",
    "runtime validation",
    "api-design",
    "api design",
  ])
    assert.equal(matchesArticle(article, query), true, query);
  assert.equal(matchesArticle(article, "databases"), false);
});
test("requires every query term and tolerates surrounding whitespace", () => {
  assert.equal(matchesArticle(article, "  TypeScript   runtime  "), true);
  assert.equal(matchesArticle(article, "TypeScript nonexistent"), false);
  assert.equal(matchesArticle(article, "   "), true);
});
test("combines category filtering with query matching", () => {
  assert.equal(matchesArticle(article, "typescript", "Development"), true);
  assert.equal(matchesArticle(article, "typescript", "Architecture"), false);
  assert.equal(matchesArticle(article, "", "Development"), true);
});
test("treats HTML and regular expression characters as literal search text", () => {
  assert.equal(matchesArticle(article, "<script>"), false);
  assert.equal(matchesArticle(article, ".*"), false);
});
