import { type MouseEvent, useEffect, useRef } from "react";
import type { Entry } from "../api/client";
import { isExternal, resolveEntry } from "../app/paths";
import { FieldChip } from "./ui";

// One rendered wiki entry (wiki/product/features/wiki-reader.md#article): "On
// this page", frontmatter chips, then the server's HTML. Links between
// entries open in the reader; checkboxes tick (the only write).

export function WikiArticle({
  entry,
  size = "reader",
  onOpen,
  onTick,
}: {
  entry: Entry;
  size?: "reader" | "sheet";
  onOpen: (path: string) => void;
  onTick: (line: number, done: boolean) => void;
}) {
  const body = useRef<HTMLDivElement>(null);
  // The server renders checkboxes disabled; here they become the one thing to
  // tick. Their state comes from the entry's checkbox list, not the HTML.
  const done = (line: number) => entry.checkboxes.find((c) => c.line === line)?.done ?? false;
  // biome-ignore lint/correctness/useExhaustiveDependencies: re-run when the HTML or the boxes change
  useEffect(() => {
    for (const el of body.current?.querySelectorAll("input[type=checkbox]") ?? []) {
      const box = el as HTMLInputElement;
      box.disabled = false;
      if (box.dataset.line) box.checked = done(Number(box.dataset.line));
    }
  }, [entry.html, entry.checkboxes]);
  const click = (e: MouseEvent<HTMLDivElement>) => {
    const target = e.target as HTMLElement;
    const box = target.closest("input[type=checkbox][data-line]") as HTMLInputElement | null;
    if (box) {
      e.preventDefault();
      onTick(Number(box.dataset.line), !done(Number(box.dataset.line)));
      return;
    }
    const link = target.closest("a") as HTMLAnchorElement | null;
    const href = link?.getAttribute("href");
    if (!link || !href || href.startsWith("#")) return;
    e.preventDefault();
    if (isExternal(href)) window.open(href, "_blank", "noopener");
    else onOpen(resolveEntry(entry.path, href));
  };
  const fields = Object.entries(entry.frontmatter).filter(([, v]) => v !== "");
  const headings = entry.headings.filter((h) => h.level === 2 || h.level === 3);
  return (
    <article className="flex flex-col gap-3.5">
      {size === "reader" && headings.length > 0 && (
        <nav className="flex flex-col gap-1 border-b border-line pb-3">
          <span className="text-xs text-tx4">On this page</span>
          {headings.map((h) => (
            <a
              key={h.anchor}
              href={`#${h.anchor}`}
              onClick={(e) => {
                e.preventDefault();
                document.getElementById(h.anchor)?.scrollIntoView({ behavior: "smooth" });
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
      {/* biome-ignore lint/a11y/useKeyWithClickEvents: the links and checkboxes inside are the keyboard path */}
      {/* biome-ignore lint/a11y/noStaticElementInteractions: it catches clicks on the links and checkboxes inside */}
      <div
        ref={body}
        className={`wiki ${size === "sheet" ? "sheet" : ""}`}
        onClick={click}
        // biome-ignore lint/security/noDangerouslySetInnerHtml: server-rendered Markdown, raw HTML off
        dangerouslySetInnerHTML={{ __html: entry.html }}
      />
    </article>
  );
}
