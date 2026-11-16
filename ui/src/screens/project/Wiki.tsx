import { useNavigate, useRouterState, useSearch } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import type { Node } from "../../api/client";
import { useEntry, useMissing, useProjectHQ, useTick, useTree } from "../../app/queries";
import { TreeChildren, TreeRow } from "../../components/FileTree";
import { TreeIcon, WarningIcon } from "../../components/icons";
import { useToast } from "../../components/Toasts";
import { Button, Dot, Panel } from "../../components/ui";
import { WikiArticle } from "../../components/WikiArticle";
import { ScaffoldNotice, WikiSetup } from "../../components/WikiSetup";
import { useProject } from "../Project";

// A project's Wiki section: a reader for its wiki (wiki/product/features/wiki-reader.md),
// the project's description at its head.

export function Wiki() {
  const { id: project, config } = useProject();
  const { entry } = useSearch({ from: "/projects/$project/wiki" });
  const repo = useProjectHQ(project);
  const navigate = useNavigate();
  const toast = useToast();
  // The tree is always shown; under 760px it folds behind Files, closed by
  // default (the CEO, 2026-11-01).
  const [files, setFiles] = useState(false);
  const lacking = useMissing(project).data ?? [];
  const tree = useTree(project);
  const path = entry ?? "index.md";
  const current = useEntry(project, path);
  const tick = useTick(project, (e) =>
    toast({ title: path, body: e.message, color: "var(--err)" }),
  );
  // At its anchor, when a link names one: the URL's #, as a repo's file has it.
  const open = (p: string, anchor = "", replace = false) =>
    navigate({
      to: "/projects/$project/wiki",
      params: { project },
      search: { entry: p },
      hash: anchor || undefined,
      replace,
    });
  const hash = useRouterState({ select: (s) => s.location.hash });
  const loaded = current.data !== undefined;
  // Opening an entry, from anywhere, shows its top; the tree keeps its place.
  // Under 760px the panes stack and the page scrolls, in <main>.
  const article = useRef<HTMLDivElement>(null);
  // biome-ignore lint/correctness/useExhaustiveDependencies: on every entry shown
  useEffect(() => {
    article.current?.scrollTo({ top: 0 });
    document.querySelector("main")?.scrollTo({ top: 0 });
  }, [path]);
  // Then to the heading the link named (the CEO's card, wiki-links-polish), once it is drawn;
  // not again when the entry reloads (a tick), which would move what the CEO reads.
  // biome-ignore lint/correctness/useExhaustiveDependencies: on every entry shown, too
  useEffect(() => {
    if (hash && loaded) document.getElementById(hash)?.scrollIntoView();
  }, [path, hash, loaded]);
  // An unknown path falls back to index.md, replacing it so Back doesn't return to it.
  const missing = current.isError && path !== "index.md";
  // biome-ignore lint/correctness/useExhaustiveDependencies: only when the entry turns out missing
  useEffect(() => {
    if (missing) open("index.md", "", true);
  }, [missing]);
  // Without its files there is no wiki to read: the tree is irrelevant.
  if (lacking.some((m) => m.required))
    return (
      <div className="px-6 pt-6">
        <WikiSetup project={project} missing={lacking} />
      </div>
    );
  if (tree.error) return <p className="px-6 pt-6 text-[13px] text-err">{tree.error.message}</p>;
  // The tree and the entry scroll apart, as a repo's page does (the CEO, 2026-11-02).
  return (
    <div className="flex h-full min-h-[420px] flex-col gap-4 px-6 pt-6 pb-6 max-[760px]:h-auto max-[760px]:pb-12">
      {/* As a repo's page is headed (Repos → a repo), so the two read alike. */}
      <header className="flex flex-col gap-0.5">
        <h1 className="m-0 text-[18px] font-semibold">Knowledge base</h1>
        <span className="truncate font-mono text-[12.5px] text-tx4" title={`${repo}/wiki`}>
          {repo}/wiki
        </span>
      </header>
      {config.description && (
        <p className="m-0 max-w-[780px] text-[15px] text-tx3">{config.description}</p>
      )}
      {lacking.length > 0 && <ScaffoldNotice project={project} missing={lacking} />}
      <span className="hidden max-[760px]:block">
        <Button
          variant={files ? "raised" : "secondary"}
          aria-pressed={files}
          onClick={() => setFiles(!files)}
        >
          <span className="flex items-center gap-2">
            <TreeIcon />
            Files
          </span>
        </Button>
      </span>
      <div className="grid min-h-0 w-full flex-1 grid-cols-[minmax(180px,260px)_minmax(0,1fr)] gap-6 max-[760px]:grid-cols-[minmax(0,1fr)]">
        <nav
          aria-label="Files"
          className={`flex min-h-0 flex-col overflow-auto pr-1 max-[760px]:overflow-visible ${files ? "" : "max-[760px]:hidden"}`}
        >
          <Tree nodes={tree.data ?? []} current={path} onOpen={open} />
        </nav>
        {/* A comfortable measure for reading, whatever the window's width. */}
        {/* The scroller is a block: a flex column of its height would shrink
            the panel to fit, clipping the entry instead of scrolling it. */}
        <div ref={article} className="min-h-0 min-w-0 overflow-auto max-[760px]:overflow-visible">
          <div className="flex max-w-[820px] flex-col gap-2.5 pb-6">
            {/* The entry's path, in view while it scrolls, as a repo's file's is. */}
            <div className="sticky top-0 z-10 bg-bg pb-1">
              <span className="block truncate font-mono text-[12.5px] text-tx2" title={path}>
                {path}
              </span>
            </div>
            {current.data && (
              <Panel className="px-6 py-5">
                <WikiArticle
                  project={project}
                  entry={current.data}
                  onOpen={open}
                  onTick={(line, done) =>
                    tick.mutate({ path, line, done, version: current.data.version })
                  }
                />
              </Panel>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

function hasChanged(nodes: Node[]): boolean {
  return nodes.some((n) => n.changed || hasChanged(n.children));
}

/** Whether a folder holds the current entry: those start expanded. */
function holds(n: Node, path: string): boolean {
  return n.children.some((c) => c.path === path || holds(c, path));
}

/** An entry's name in the tree: its title, and a folder's page says it is one
 * (the CEO, 2026-11-05: a folder's index.md went unnoticed). */
export function treeLabel(n: Node): string {
  if (!n.title) return n.path;
  return n.path === "index.md" || n.path.endsWith("/index.md") ? `${n.title} - index.md` : n.title;
}

/** An entry whose header couldn't be read, named with the reason: the entry
 * still opens, so amber, a thing to fix. */
export function UnreadableMark({ error }: { error: string }) {
  const name = `Frontmatter couldn't be read: ${error}`;
  return (
    <span role="img" aria-label={name} title={name} className="text-attnTx">
      <WarningIcon />
    </span>
  );
}

function Tree({
  nodes,
  current,
  onOpen,
}: {
  nodes: Node[];
  current: string;
  onOpen: (path: string) => void;
}) {
  return nodes.map((n) =>
    n.folder ? (
      <Folder key={n.path} node={n} current={current} onOpen={onOpen} />
    ) : (
      <TreeRow
        key={n.path}
        current={n.path === current}
        onClick={() => onOpen(n.path)}
        after={
          <>
            {n.frontmatterError && <UnreadableMark error={n.frontmatterError} />}
            {n.changed && <Dot size={6} title="Changed since you last looked" />}
          </>
        }
      >
        {treeLabel(n)}
      </TreeRow>
    ),
  );
}

function Folder({
  node,
  current,
  onOpen,
}: {
  node: Node;
  current: string;
  onOpen: (path: string) => void;
}) {
  const [open, setOpen] = useState(holds(node, current));
  // Following a link into a collapsed folder opens it; a folder the CEO opened stays open.
  // biome-ignore lint/correctness/useExhaustiveDependencies: when the current entry changes
  useEffect(() => {
    if (holds(node, current)) setOpen(true);
  }, [current]);
  // The whole row toggles, and only that: its index.md is a row inside.
  return (
    <>
      <TreeRow
        folder
        open={open}
        onClick={() => setOpen(!open)}
        after={
          !open &&
          hasChanged(node.children) && <Dot size={6} title="Changed since you last looked" />
        }
      >
        {node.title || node.path}
      </TreeRow>
      {open && (
        <TreeChildren>
          <Tree nodes={node.children} current={current} onOpen={onOpen} />
        </TreeChildren>
      )}
    </>
  );
}
