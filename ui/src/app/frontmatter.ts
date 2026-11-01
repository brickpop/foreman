import type { Field } from "../api/client";

// A frontmatter as the reader shows it (wiki/product/features/wiki-reader.md#frontmatter),
// for a wiki entry and a repo's Markdown file alike.

/** The `title` field apart, as the page's heading (empty is none), and the
 * fields shown. */
export function withoutTitle(fields: Field[]): { title?: string; fields: Field[] } {
  const title = fields.find((f) => f.key === "title")?.values[0]?.text;
  return { title: title || undefined, fields: fields.filter((f) => f.key !== "title") };
}

/** One part of the folded line: a field's values, with no key. A field naming
 * several entries is counted instead ("2 blocked_by"), so the line stays short. */
export type SummaryPart = { key: string; text: string; tags?: string[] };

export function summary(fields: Field[]): SummaryPart[] {
  return fields
    .filter((f) => f.values.length > 0)
    .map((f) => {
      if (f.key === "tags") return { key: f.key, text: "", tags: f.values.map((v) => v.text) };
      if (f.values.length > 1 && f.values.every((v) => v.to))
        return { key: f.key, text: `${f.values.length} ${f.key}` };
      return { key: f.key, text: f.values.map((v) => v.title || v.text).join(", ") };
    });
}

const tagColors = ["var(--acc)", "var(--info)", "var(--ok)", "var(--team)"];

/** A tag's colour, the same wherever the tag shows: from its name. */
export function tagColor(tag: string): string {
  let h = 0;
  for (const c of tag) h = (h * 31 + c.charCodeAt(0)) >>> 0;
  return tagColors[h % tagColors.length];
}
