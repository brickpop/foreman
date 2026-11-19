import { Fragment, type ReactNode, useState } from "react";
import type { Field } from "../api/client";
import { summary, tagColor } from "../app/frontmatter";
import { Chevron } from "./icons";

// A frontmatter above its Markdown (wiki/product/features/wiki-reader.md#frontmatter),
// as WikiShell shows it (its ui/src/views/EntryView.tsx, adapted): folded, one
// line of its values; open, its keys and values in a grid. For a wiki entry
// and a repo's Markdown file alike.

type Value = Field["values"][number];

const remembered = "foreman.frontmatter";

function wasOpen(): boolean {
  try {
    return localStorage.getItem(remembered) === "open";
  } catch {
    return false;
  }
}

/** Folded or open, as the CEO last left it in this browser. A value naming
 * an entry opens it with onOpen (a wiki's; none in a repo). */
export function Frontmatter({
  fields,
  onOpen,
}: {
  fields: Field[];
  onOpen?: (path: string) => void;
}) {
  const [open, setOpen] = useState(wasOpen);
  if (fields.length === 0) return null;
  const toggle = () => {
    setOpen(!open);
    try {
      localStorage.setItem(remembered, open ? "folded" : "open");
    } catch {
      // Storage blocked: it opens folded next time.
    }
  };
  return (
    <div className="rounded-lg border border-line bg-panel text-[12.5px]">
      <button
        type="button"
        aria-expanded={open}
        onClick={toggle}
        className="flex w-full min-w-0 items-center gap-2 px-3 py-1.5 text-left text-tx3 hover:text-tx2"
      >
        <Chevron open={open} />
        {open ? "Properties" : <Summary fields={fields} />}
      </button>
      {open && (
        <dl className="m-0 grid grid-cols-[max-content_minmax(0,1fr)] gap-x-4 gap-y-1.5 border-t border-line px-3 py-2.5">
          {fields.map((f) => (
            <Fragment key={f.key}>
              {/* Keys stay muted: colour is for what can be clicked. */}
              <dt className="text-tx4">{f.key}</dt>
              <dd className="m-0 flex min-w-0 flex-wrap items-center gap-1.5 text-tx2">
                <Values field={f} onOpen={onOpen} />
              </dd>
            </Fragment>
          ))}
        </dl>
      )}
    </div>
  );
}

/** The folded line: the values, no keys, clipped to one line. */
function Summary({ fields }: { fields: Field[] }) {
  return (
    <span className="min-w-0 flex-1 truncate">
      {summary(fields).map((p, i) => (
        <Fragment key={p.key}>
          {i > 0 && <span className="text-tx4"> · </span>}
          {p.tags
            ? p.tags.map((t, j) => (
                <Fragment key={t}>
                  {j > 0 && " "}
                  <Dot tag={t} />
                  {t}
                </Fragment>
              ))
            : p.text}
        </Fragment>
      ))}
    </span>
  );
}

function Values({ field, onOpen }: { field: Field; onOpen?: (path: string) => void }) {
  if (field.values.length === 0) return <span className="text-tx4">(nothing)</span>;
  if (field.key === "tags")
    return field.values.map((v) => (
      <Chip key={v.text}>
        <Dot tag={v.text} />
        {v.text}
      </Chip>
    ));
  if (field.values.length === 1 && !field.values[0].to)
    return (
      <span className="truncate" title={field.values[0].text}>
        {field.values[0].text}
      </span>
    );
  return field.values.map((v, i) =>
    v.to ? (
      // biome-ignore lint/suspicious/noArrayIndexKey: a list may repeat a value
      <EntryRef key={i} value={v} onOpen={onOpen} />
    ) : (
      // biome-ignore lint/suspicious/noArrayIndexKey: a list may repeat a value
      <Chip key={i}>{v.text}</Chip>
    ),
  );
}

/** A value naming an entry: its title, opening it; the value as written in
 * its tooltip. */
function EntryRef({ value, onOpen }: { value: Value; onOpen?: (path: string) => void }) {
  const label = value.title || value.text;
  if (!onOpen) return <span title={value.text}>{label}</span>;
  return (
    <button
      type="button"
      title={value.text}
      onClick={() => onOpen(value.to)}
      className="max-w-[28ch] truncate text-accTx underline decoration-accLine underline-offset-2 hover:decoration-current"
    >
      {label}
    </button>
  );
}

function Chip({ children }: { children: ReactNode }) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-[5px] bg-raised px-2 py-[2px] text-tx2">
      {children}
    </span>
  );
}

function Dot({ tag }: { tag: string }) {
  return (
    <span
      aria-hidden
      className="mr-1 inline-block size-[7px] rounded-full align-middle"
      style={{ background: tagColor(tag) }}
    />
  );
}

/** A file whose header the library couldn't read has no fields, so it would
 * look as if it had none: say so above the body, which still renders. Shown
 * wherever a frontmatter is, for a wiki entry and a repo's Markdown file alike,
 * each named as what it is. */
export function FrontmatterError({
  error,
  noun = "entry",
}: {
  error: string;
  noun?: "entry" | "file";
}) {
  return (
    <p className="m-0 rounded-lg border border-attnLine bg-attnSoft px-3 py-2.5 text-[13px] text-attnTx">
      This {noun}'s frontmatter couldn't be read: {error}. Fix it, then run <code>wiki check</code>.
    </p>
  );
}
