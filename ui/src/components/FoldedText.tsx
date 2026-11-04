import { useState } from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

// An agent's own text under its title (a request's body, a review's), as the
// Markdown it is (the CEO, 2026-11-04), in the wiki's styles at a smaller
// size. Raw HTML stays text: agents write these. Folded when long, so one long
// text can't push the rest away.

/** Past this a text starts folded. */
const foldedLines = 6;
const foldedChars = 480;

// A link leaves Foreman: a new tab, telling the site nothing.
const components: Components = {
  a: ({ href, children }) => (
    <a href={href} target="_blank" rel="noreferrer">
      {children}
    </a>
  ),
};

/** Nothing when there is no text. */
export function FoldedText({ text }: { text: string }) {
  const long = text.length > foldedChars || text.split("\n").length > foldedLines;
  const [open, setOpen] = useState(false);
  if (!text.trim()) return null;
  return (
    <div className="flex flex-col items-start gap-1">
      <div
        className={`wiki compact self-stretch break-words ${long ? (open ? "max-h-[50vh] overflow-auto" : "folded") : ""}`}
      >
        <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
          {text}
        </ReactMarkdown>
      </div>
      {long && (
        <button
          type="button"
          aria-expanded={open}
          onClick={() => setOpen(!open)}
          className="text-[13px] text-tx3 hover:text-accTx"
        >
          {open ? "Show less" : "Show all"}
        </button>
      )}
    </div>
  );
}
