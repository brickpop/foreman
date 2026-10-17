import { type Entry, useClient } from "../api/client";
import { Markdown } from "./Markdown";
import { FieldChip } from "./ui";

// One wiki entry (wiki/product/features/wiki-reader.md#article): "On this
// page", frontmatter chips, then its Markdown. Links between entries open in
// the reader; checkboxes tick (the only write).

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
  const headings = entry.headings.filter((h) => h.level === 2 || h.level === 3);
  return (
    <article className="flex flex-col gap-3.5">
      {size === "reader" && headings.length > 0 && (
        <nav className="flex flex-col gap-1 border-b border-line pb-3">
          <span className="text-xs text-tx4">On this page</span>
          {headings.map((h) => (
            <a
              key={h.id}
              href={`#${h.id}`}
              onClick={(e) => {
                e.preventDefault();
                document.getElementById(h.id)?.scrollIntoView({ behavior: "smooth" });
              }}
              className={`text-[13px] text-tx3 hover:text-tx2 ${h.level === 3 ? "pl-3" : ""}`}
            >
              {h.text}
            </a>
          ))}
        </nav>
      )}
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
