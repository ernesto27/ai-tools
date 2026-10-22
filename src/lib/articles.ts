import { getCollection } from "astro:content";

export const slugify = (value: string) =>
  value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
export const formatDate = (date: Date) =>
  new Intl.DateTimeFormat("en", {
    month: "short",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  }).format(date);
export const readingTime = (body = "") =>
  Math.max(
    1,
    Math.ceil(
      body
        .replace(/```[\s\S]*?```/g, "")
        .split(/\s+/)
        .filter(Boolean).length / 200,
    ),
  );
export async function getArticles() {
  return (await getCollection("articles", ({ data }) => !data.draft)).sort(
    (a, b) =>
      b.data.publishedAt.valueOf() - a.data.publishedAt.valueOf() ||
      a.id.localeCompare(b.id),
  );
}
