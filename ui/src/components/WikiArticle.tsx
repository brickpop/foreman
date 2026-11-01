import { type Entry, useClient } from "../api/client";
import { withoutTitle } from "../app/frontmatter";
import { Frontmatter } from "./Frontmatter";
import { Markdown } from "./Markdown";

// One wiki entry (wiki/product/features/wiki-reader.md#article): its
// frontmatter, then its Markdown. Links between entries open in the reader,
// from the body and from the frontmatter; checkboxes tick (the only write).

export function WikiArticle({
  project,
  entry,
  size = "reader",
  onOpen,
  onTick,
}: {
  project: string;
  entry: Entry;
  size?: "reader" | "sheet";
  onOpen: (path: string) => void;
  onTick: (line: number, done: boolean) => void;
}) {
  const client = useClient();
  // The title field is the heading: a card's body doesn't repeat it.
  const { title, fields } = withoutTitle(entry.frontmatter);
  return (
    <article className="flex flex-col gap-3.5">
      <Frontmatter fields={fields} onOpen={onOpen} />
      <div className={`wiki ${size === "sheet" ? "sheet" : ""}`}>
        {title && <h1>{title}</h1>}
        <Markdown
          entry={entry}
          onOpen={onOpen}
          onTick={onTick}
          imageUrl={(path) => client.wiki.imageUrl(project, path)}
        />
      </div>
    </article>
  );
}
