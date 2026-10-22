export interface SearchableArticle {
  title: string;
  description: string;
  tags: string[];
  category: string;
}
export function matchesArticle(
  article: SearchableArticle,
  query: string,
  category = "",
): boolean {
  const terms = query.toLocaleLowerCase().trim().split(/\s+/).filter(Boolean);
  const text = [
    article.title,
    article.description,
    ...article.tags.map((tag) => tag.replaceAll("-", " ")),
  ]
    .join(" ")
    .toLocaleLowerCase();
  return (
    (!category || article.category === category) &&
    terms.every((term) => text.includes(term.replaceAll("-", " ")))
  );
}
