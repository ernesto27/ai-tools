import { z } from "zod";

export const articleSchema = z.object({
  title: z.string().trim().min(10).max(110),
  description: z.string().trim().min(40).max(200),
  publishedAt: z.coerce.date(),
  author: z.object({ name: z.string().min(2), role: z.string().min(2) }),
  category: z.enum([
    "Architecture",
    "Development",
    "Engineering culture",
    "Performance",
  ]),
  tags: z
    .array(z.string().regex(/^[a-z0-9]+(?:-[a-z0-9]+)*$/))
    .min(1)
    .max(5),
  artwork: z.enum([
    "systems",
    "typescript",
    "testing",
    "performance",
    "reviews",
    "simplicity",
  ]),
  featured: z.boolean().default(false),
  draft: z.boolean().default(false),
});
