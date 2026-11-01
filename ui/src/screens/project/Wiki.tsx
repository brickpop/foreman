import { useNavigate, useSearch } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { isNotBuilt, type Node } from "../../api/client";
import { useEntry, useMissing, useProjectRepo, useTick, useTree } from "../../app/queries";
import { Chevron, TreeIcon } from "../../components/icons";
import { useToast } from "../../components/Toasts";
import { Button, Dot, NotBuilt } from "../../components/ui";
import { WikiArticle } from "../../components/WikiArticle";
import { ScaffoldNotice, WikiSetup } from "../../components/WikiSetup";
import { useProject } from "../Project";

// A project's Wiki section: a reader for its wiki (wiki/product/features/wiki-reader.md),
// the project's description at its head.

export function Wiki() {
  const { id: project, config } = useProject();
  const { entry } = useSearch({ from: "/projects/$project/wiki" });
  const repo = useProjectRepo(project);
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
  const open = (p: string, replace = false) =>
    navigate({ to: "/projects/$project/wiki", params: { project }, search: { entry: p }, replace });
  // Opening an entry, from anywhere, shows its top. The page scrolls in <main>.
  // biome-ignore lint/correctness/useExhaustiveDependencies: on every entry shown
  useEffect(() => {
    document.querySelector("main")?.scrollTo({ top: 0 });
  }, [path]);
  // An unknown path falls back to index.md, replacing it so Back doesn't return to it.
  const notBuilt = isNotBuilt(tree.error);
  const missing = current.isError && !notBuilt && path !== "index.md";
  // biome-ignore lint/correctness/useExhaustiveDependencies: only when the entry turns out missing
  useEffect(() => {
    if (missing) open("index.md", true);
  }, [missing]);
  if (notBuilt)
    return (
      <div className="px-6 pt-6">
        <NotBuilt what="the wiki" />
      </div>
    );
  // Without its files there is no wiki to read: the tree is irrelevant.
  if (lacking.some((m) => m.required))
    return (
      <div className="px-6 pt-6">
        <WikiSetup project={project} missing={lacking} />
      </div>
    );
  if (tree.error) return <p className="px-6 pt-6 text-[13px] text-err">{tree.error.message}</p>;
  return (
    <div className="flex flex-col gap-5 px-6 pt-5 pb-12">
      {config.description && (
        <p className="m-0 max-w-[780px] text-[15px] text-tx3">{config.description}</p>
      )}
      {lacking.length > 0 && <ScaffoldNotice project={project} missing={lacking} />}
      <div className="flex items-center gap-3">
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
        <span className="truncate font-mono text-xs text-tx4">
          {repo}/wiki/{path}
        </span>
      </div>
      <div className="grid w-full grid-cols-[minmax(150px,200px)_minmax(0,1fr)] gap-8 max-[760px]:grid-cols-[minmax(0,1fr)]">
        <nav
          className={`sticky top-4 self-start max-[760px]:static ${files ? "" : "max-[760px]:hidden"}`}
        >
          <Tree nodes={tree.data ?? []} current={path} depth={0} onOpen={open} />
        </nav>
        {/* A comfortable measure for reading, whatever the window's width. */}
        <div className="min-w-0 max-w-[780px]">
          {current.data && (
            <WikiArticle
              project={project}
              entry={current.data}
              onOpen={open}
              onTick={(line, done) =>
                tick.mutate({ path, line, done, version: current.data.version })
              }
            />
          )}
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
  if (path === `${n.path}/index.md`) return true; // its own index isn't among its children
  return n.children.some((c) => c.path === path || holds(c, path));
}

function Tree({
  nodes,
  current,
  depth,
  onOpen,
}: {
  nodes: Node[];
  current: string;
  depth: number;
  onOpen: (path: string) => void;
}) {
  return (
    <ul className="m-0 list-none p-0">
      {nodes.map((n) =>
        n.folder ? (
          <Folder key={n.path} node={n} current={current} depth={depth} onOpen={onOpen} />
        ) : (
          <li key={n.path}>
            <button
              type="button"
              onClick={() => onOpen(n.path)}
              style={{ paddingLeft: 10 + 14 * depth }}
              className={`flex w-full items-center gap-1.5 rounded-md py-[5px] pr-2.5 text-left text-[13.5px] ${
                n.path === current ? "bg-raised text-tx" : "text-tx3 hover:bg-hover"
              }`}
            >
              {/* In line with the folder names beside it. */}
              <span className="w-3 shrink-0" />
              <span className="truncate">{n.title || n.path}</span>
              {n.changed && <Dot size={6} title="Changed since you last looked" />}
            </button>
          </li>
        ),
      )}
    </ul>
  );
}

function Folder({
  node,
  current,
  depth,
  onOpen,
}: {
  node: Node;
  current: string;
  depth: number;
  onOpen: (path: string) => void;
}) {
  const [open, setOpen] = useState(holds(node, current));
  // Following a link into a collapsed folder opens it; a folder the CEO opened stays open.
  // biome-ignore lint/correctness/useExhaustiveDependencies: when the current entry changes
  useEffect(() => {
    if (holds(node, current)) setOpen(true);
  }, [current]);
  const index = `${node.path}/index.md`;
  // The whole row toggles; expanding a folder with an index.md also opens it.
  const toggle = () => {
    if (!open && node.hasIndex) onOpen(index);
    setOpen(!open);
  };
  return (
    <li>
      <button
        type="button"
        aria-expanded={open}
        onClick={toggle}
        style={{ paddingLeft: 10 + 14 * depth }}
        className={`flex w-full items-center gap-1.5 rounded-md py-[5px] pr-2.5 text-left text-[13.5px] font-medium ${
          current === index ? "bg-raised text-tx" : "text-tx2 hover:bg-hover"
        }`}
      >
        <span className="text-tx4">
          <Chevron open={open} />
        </span>
        <span className="truncate">{node.title || node.path}</span>
        {!open && hasChanged(node.children) && (
          <Dot size={6} title="Changed since you last looked" />
        )}
      </button>
      {open && <Tree nodes={node.children} current={current} depth={depth + 1} onOpen={onOpen} />}
    </li>
  );
}
