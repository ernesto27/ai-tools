import rss from "@astrojs/rss";
import type { APIContext } from "astro";
import { getArticles } from "../lib/articles";
export async function GET(context: APIContext) {
  return rss({
    title: "Fieldnotes",
    description:
      "Good software. Considered ideas. An independent engineering journal.",
    site: context.site!,
    items: (await getArticles()).map((post) => ({
      title: post.data.title,
      description: post.data.description,
      pubDate: post.data.publishedAt,
      link: `/articles/${post.id}/`,
      categories: post.data.tags,
    })),
    customData: "<language>en-us</language>",
  });
}
