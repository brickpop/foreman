import { type ReactNode, useMemo } from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import type { Entry, RepoLink } from "../api/client";
import { remarkCallout } from "../app/callout";

// Markdown, rendered in the browser the way wikiview does it (its
// ui/src/markdown/Markdown.tsx, v0.13.0, adapted): a wiki entry, or a repo's
// file. The server's rules arrive as data and are applied by lookup, never
// re-derived: a link resolves by its href in links, a heading takes its id
// from headings, a checkbox its file line from checkboxes. Matched by source
// line, which a re-render can't throw off. Raw HTML stays off: agents write
// these files.

/** A link as the server resolved it: a wiki entry's, or a repo file's. */
export type ResolvedLink = Omit<Entry["links"][number], "file"> &
  Partial<Pick<Entry["links"][number], "file"> & Pick<RepoLink, "folder">>;

/** The source line react-markdown gives every node it renders. */
const lineOf = (props: unknown): number =>
  (props as { node?: { position?: { start?: { line?: number } } } }).node?.position?.start?.line ??
  -1;

/** The href as written: the renderer percent-encodes some (`<my file.md>`). */
const decoded = (href: string) => {
  try {
    return decodeURI(href);
  } catch {
    return href;
  }
};

export function Markdown({
  body,
  links: resolved,
  headings: ids,
  checkboxes: ticks = [],
  onOpen,
  onTick,
  imageUrl,
  scope = "wiki",
}: {
  body: string;
  links: ResolvedLink[];
  headings: Entry["headings"];
  /** Tickable ones (a wiki entry's); none for a read-only file. */
  checkboxes?: Entry["checkboxes"];
  onOpen: (link: ResolvedLink) => void;
  onTick?: (line: number, done: boolean) => void;
  imageUrl: (path: string) => string;
  /** What the links stay inside, for their titles. */
  scope?: "wiki" | "repo";
}) {
  const links = useMemo(() => new Map(resolved.map((l) => [l.raw, l])), [resolved]);
  const lookup = (href: string) => links.get(href) ?? links.get(decoded(href));
  const headings = useMemo(() => new Map(ids.map((h) => [h.bodyLine, h.id])), [ids]);
  const boxes = useMemo(() => new Map(ticks.map((c) => [c.bodyLine, c])), [ticks]);
  const heading = (level: 1 | 2 | 3 | 4 | 5 | 6) =>
    function Heading({ children, ...props }: { children?: ReactNode }) {
      const Tag = `h${level}` as const;
      // A heading with no letter or digit (`## ???`) has the id "": no id at
      // all, rather than an empty one, or an anchor nothing can link to.
      return <Tag id={headings.get(lineOf(props)) || undefined}>{children}</Tag>;
    };

  const components: Components = {
    a({ href, children }) {
      const link = href ? lookup(href) : undefined;
      // Out of the wiki or the repo: nothing here serves it, so it isn't a link.
      if (link?.outside) {
        return (
          <span className="outside" title={`${href} is outside the ${scope}`}>
            {children}
          </span>
        );
      }
      if (link?.image) {
        return (
          <a href={imageUrl(link.to)} target="_blank" rel="noopener noreferrer">
            {children}
          </a>
        );
      }
      // Any other file of the wiki: Foreman serves its images only.
      if (link?.file) {
        return (
          <span className="outside" title={`Foreman doesn't show ${link.to}`}>
            {children}
          </span>
        );
      }
      if (link) {
        return (
          <a
            href={link.to}
            className={link.exists ? undefined : "unwritten"}
            title={
              link.exists ? undefined : scope === "wiki" ? "Not written yet" : "Not on this branch"
            }
            onClick={(e) => {
              e.preventDefault();
              onOpen(link);
            }}
          >
            {children}
          </a>
        );
      }
      // A URL the renderer emptied (javascript: and the like) is no link.
      if (!href) return <span>{children}</span>;
      // Not the wiki's: a page elsewhere, or an anchor on this one.
      const external = !href.startsWith("#");
      return (
        <a href={href} {...(external ? { target: "_blank", rel: "noopener noreferrer" } : {})}>
          {children}
        </a>
      );
    },
    // Only the wiki's own images load: an image elsewhere would have the
    // browser call whatever address an agent wrote, so it shows as a link.
    img({ src, alt }) {
      const link = typeof src === "string" ? lookup(src) : undefined;
      if (link?.image) return <img src={imageUrl(link.to)} alt={alt ?? ""} loading="lazy" />;
      if (typeof src === "string" && !link) {
        return (
          <a href={src} target="_blank" rel="noopener noreferrer">
            {alt || src}
          </a>
        );
      }
      return <span className="unwritten">{alt || src}</span>;
    },
    table({ children }) {
      return (
        <div className="table">
          <table>{children}</table>
        </div>
      );
    },
    // remarkCallout left the type and label on the blockquote; the label is
    // an element rather than CSS content, so a screen reader hears it.
    blockquote({ children, node: _node, ...props }) {
      const { "data-callout-label": label, ...rest } = props as { "data-callout-label"?: string };
      if (!("data-callout" in props)) return <blockquote>{children}</blockquote>;
      return (
        <blockquote className="callout" {...rest}>
          <p className="callout-label">{label}</p>
          {children}
        </blockquote>
      );
    },
    // remark-gfm gives the <input> no position, so the item carries the line.
    li({ children, ...props }) {
      const box = boxes.get(lineOf(props));
      if (!box || !onTick) return <li>{children}</li>;
      return (
        <li>
          <input
            type="checkbox"
            checked={box.done}
            onChange={() => onTick(box.line, !box.done)}
            aria-label={box.text}
          />
          {withoutRenderedCheckbox(children)}
        </li>
      );
    },
    h1: heading(1),
    h2: heading(2),
    h3: heading(3),
    h4: heading(4),
    h5: heading(5),
    h6: heading(6),
  };

  return (
    <ReactMarkdown remarkPlugins={[remarkGfm, remarkCallout]} components={components}>
      {body}
    </ReactMarkdown>
  );
}

/** Drops the disabled <input> remark-gfm renders in a task item: this one ticks. */
function withoutRenderedCheckbox(children: ReactNode): ReactNode {
  if (!Array.isArray(children)) return children;
  return children.filter((c) => !(c && typeof c === "object" && "type" in c && c.type === "input"));
}
