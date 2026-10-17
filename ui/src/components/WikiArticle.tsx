import { type Entry, useClient } from "../api/client";
import { Markdown } from "./Markdown";
import { FieldChip } from "./ui";

// One wiki entry (wiki/product/features/wiki-reader.md#article): frontmatter
// chips, then its Markdown. Links between entries open in the reader;
// checkboxes tick (the only write).

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
  const fields = Object.entries(entry.frontmatter).filter(([, v]) => v !== "");
  return (
    <article className="flex flex-col gap-3.5">
      {fields.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {fields.map(([k, v]) => (
            <FieldChip key={k} name={k} value={v} />
          ))}
        </div>
      )}
      <div className={`wiki ${size === "sheet" ? "sheet" : ""}`}>
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
