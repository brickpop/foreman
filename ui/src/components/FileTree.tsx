import type { ReactNode } from "react";
import { Chevron, FileIcon, FolderIcon } from "./icons";

// A file tree's look, the same in the Wiki and in a repo's page: a chevron and
// a folder icon on folders, a file icon on files, and a guide line down each
// open folder's contents. Each tree keeps its own data and folding.

/** One row: a folder (open or not) or a file; the current one in the accent.
 * Without onClick it is only shown (a repo's submodule). */
export function TreeRow({
  folder,
  open = false,
  current = false,
  mono = false,
  onClick,
  children,
  after,
  title,
}: {
  folder?: boolean;
  open?: boolean;
  current?: boolean;
  /** File names, as in a repo; a wiki shows its entries' titles. */
  mono?: boolean;
  onClick?: () => void;
  children: ReactNode;
  /** Beside the name: a changed dot, a "link" note. */
  after?: ReactNode;
  /** The tooltip: the whole path, where the row shows only a name. */
  title?: string;
}) {
  const look = `flex w-full min-w-0 items-center gap-1.5 rounded-md px-1.5 py-[3px] text-left ${
    mono ? "font-mono text-[12.5px]" : "text-[13.5px]"
  } ${current ? "bg-accSoft text-accTx" : onClick ? "text-tx2 hover:bg-hover" : "text-tx4"}`;
  const inside = (
    <>
      {folder ? <Chevron open={open} /> : <span className="w-3 shrink-0" />}
      <span className={current ? "" : "text-tx4"}>
        {folder ? <FolderIcon open={open} /> : <FileIcon />}
      </span>
      <span className="truncate">{children}</span>
      {after}
    </>
  );
  if (!onClick)
    return (
      <span className={look} title={title}>
        {inside}
      </span>
    );
  return (
    <button
      type="button"
      aria-expanded={folder ? open : undefined}
      aria-current={current ? "page" : undefined}
      onClick={onClick}
      title={title}
      className={look}
    >
      {inside}
    </button>
  );
}

/** An open folder's contents, indented under its chevron with a guide line. */
export function TreeChildren({ children }: { children: ReactNode }) {
  return <div className="ml-[13px] flex flex-col border-l border-line pl-1">{children}</div>;
}
