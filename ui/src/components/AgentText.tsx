import * as Popover from "@radix-ui/react-popover";
import { useState } from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

// An agent's own text under its title (a request's body, a review's), as the
// Markdown it is (the CEO, 2026-11-04), in the wiki's styles at a smaller
// size. Raw HTML stays text: agents write these. Two ways to show it, so one
// long text can't push the rest away: folded in place (a request's panel), or
// a one-line box that opens it in a popover (a review's header, where the
// diffs need the room: the CEO, 2026-11-05).

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

function AgentMarkdown({ text }: { text: string }) {
  return (
    <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
      {text}
    </ReactMarkdown>
  );
}

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
        <AgentMarkdown text={text} />
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

/** A text's first line as plain words: its Markdown marks dropped. */
export function firstLine(text: string): string {
  const line = text.split("\n").find((l) => l.trim()) ?? "";
  return line
    .replace(/^\s*(?:#+|>|[-*+]|\d+\.)\s+/, "")
    .replace(/[*_`]/g, "")
    .replace(/\[([^\]]*)\]\([^)]*\)/g, "$1")
    .trim();
}

/** The text's first line in a box, the whole of it in a popover once clicked.
 * Nothing when there is no text. */
export function TextBox({ text, label }: { text: string; label: string }) {
  if (!text.trim()) return null;
  return (
    <Popover.Root>
      <Popover.Trigger
        aria-label={label}
        className="flex h-10 min-w-0 items-center gap-3 rounded-lg border border-line bg-panel px-3.5 text-left text-[13.5px] hover:border-line3 data-[state=open]:border-accLine"
      >
        <span className="min-w-0 flex-1 truncate text-tx2">{firstLine(text)}</span>
        <span className="shrink-0 rounded-md bg-raised px-2 py-0.5 text-[12.5px] text-tx3">
          {label}
        </span>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          align="start"
          sideOffset={4}
          collisionPadding={16}
          className="wiki compact z-50 max-h-[min(520px,70vh)] w-[min(720px,calc(100vw-32px))] overflow-auto break-words rounded-lg border border-line2 bg-panel px-4 py-3 shadow-overlay"
        >
          <AgentMarkdown text={text} />
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
