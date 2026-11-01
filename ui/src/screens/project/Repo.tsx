import { Link, useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import type { Repo as RepoInfo } from "../../api/client";
import { when } from "../../app/model";
import { useRepoFile, useRepoLog, useRepos, useRepoTree, useTeams } from "../../app/queries";
import {
  branchOwner,
  bytes,
  crumbs,
  folderOf,
  type GraphRow,
  graph,
  isMarkdown,
  shownAt,
} from "../../app/repos";
import { FileMarkdown } from "../../components/Markdown";
import { Button, Empty, Loading, Panel, Segmented } from "../../components/ui";
import { useProject } from "../Project";

// A repo's page in the project's Repos section (wiki/product/features/repos.md):
// read-only and as committed, its branches, and a branch's files and history.
// The branch shown is in the URL, so a link (an agent's worktree, on
// Deployment) opens it.

type At = { repo: string; branch: string };

export function Repo() {
  const project = useProject();
  const repos = useRepos(project.id);
  const { repo: name } = useParams({ from: "/projects/$project/repos/$repo" });
  const search = useSearch({ from: "/projects/$project/repos/$repo" });
  const navigate = useNavigate({ from: "/projects/$project/repos/$repo" });
  const list = repos.data ?? [];
  const repo = list.find((r) => r.name === name);
  const at = shownAt(list, name, search.branch);
  const choose = (next: At) =>
    navigate({ search: (s) => ({ ...s, branch: next.branch }), replace: true });
  return (
    <div className="flex flex-col gap-3 px-6 pt-6 pb-12">
      <div className="flex items-center gap-3">
        <Link
          to="/projects/$project/repos"
          params={{ project: project.id }}
          aria-label="Back to the repos"
          title="Repos"
          className="grid size-8 shrink-0 place-items-center rounded-md border border-line2 text-tx3 hover:border-line3 hover:text-tx"
        >
          ←
        </Link>
        <h1 className="m-0 font-mono text-[18px] font-semibold">{name}/</h1>
        {repo?.project && <span className="text-[13px] text-tx4">project repo</span>}
      </div>
      {repos.isError && <p className="m-0 text-[13px] text-err">{repos.error.message}</p>}
      {repos.isPending && <Loading rows={6} />}
      {repos.data && !repo && (
        <Empty>
          {project.id} has no repo {name}.
        </Empty>
      )}
      {repo && at && (
        <div className="grid h-[calc(100vh-120px)] min-h-[360px] grid-cols-[minmax(180px,240px)_minmax(0,1fr)] gap-6">
          <Branches repo={repo} at={at} onChoose={choose} />
          {/* Keyed by branch: another branch starts at its root. */}
          <Browser key={at.branch} project={project.id} at={at} />
        </div>
      )}
    </div>
  );
}

/** The repo's branches, scrolling within the column: main first (the
 * default branch), then whose each is, how far ahead of main, and whether its
 * worktree holds uncommitted changes. */
function Branches({ repo, at, onChoose }: { repo: RepoInfo; at: At; onChoose: (at: At) => void }) {
  const teams = useTeams().data ?? [];
  // The branch shown scrolls into the column's view, when a link opened one far down.
  const selected = useRef<HTMLButtonElement>(null);
  // biome-ignore lint/correctness/useExhaustiveDependencies: when the branch shown changes only, never on a refetch (it would undo the CEO's own scrolling)
  useEffect(() => {
    selected.current?.scrollIntoView({ block: "nearest" });
  }, [at.branch]);
  return (
    <nav className="flex min-h-0 min-w-0 flex-col gap-2">
      <div className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-auto">
        {repo.branches.map((b) => {
          const on = at.branch === b.name;
          const detail = [
            b.name === repo.main && "default branch",
            branchOwner(b.name, teams),
            b.ahead > 0 && `${b.ahead} ahead of ${repo.main}`,
          ].filter(Boolean);
          return (
            <button
              key={b.name}
              ref={on ? selected : undefined}
              type="button"
              aria-pressed={on}
              onClick={() => onChoose({ repo: repo.name, branch: b.name })}
              className={`flex min-w-0 flex-col rounded-md px-2 py-1 text-left ${on ? "bg-accSoft" : "hover:bg-hover"}`}
            >
              <span
                className={`truncate font-mono text-[12.5px] ${on ? "text-accTx" : "text-tx2"}`}
              >
                {b.name}
              </span>
              {(detail.length > 0 || b.uncommitted) && (
                <span className="text-[11.5px] text-tx4">
                  {detail.join(" · ")}
                  {b.uncommitted && (
                    <span className="text-accTx">
                      {detail.length > 0 ? " · " : ""}uncommitted changes
                    </span>
                  )}
                </span>
              )}
            </button>
          );
        })}
      </div>
    </nav>
  );
}

/** A branch's files or history. */
function Browser({ project, at }: { project: string; at: At }) {
  const [view, setView] = useState<"files" | "history">("files");
  const [path, setPath] = useState("");
  const [file, setFile] = useState(false); // path is a file, not a folder
  const open = (p: string, isFile: boolean) => {
    setPath(p);
    setFile(isFile);
  };
  return (
    <div className="flex min-h-0 min-w-0 flex-col gap-3">
      {/* The path on the left, the view on the right (the CEO, 2026-10-17). */}
      <div className="flex flex-wrap items-center gap-3">
        {view === "files" && (
          <span className="flex min-w-0 flex-wrap items-center gap-1 font-mono text-[12.5px]">
            <button type="button" onClick={() => open("", false)} className="text-accTx">
              {at.repo}
            </button>
            {crumbs(path).map((c, i, all) => (
              <span key={c.path} className="flex items-center gap-1">
                <span className="text-tx4">/</span>
                {i === all.length - 1 ? (
                  <span className="text-tx">{c.name}</span>
                ) : (
                  <button type="button" onClick={() => open(c.path, false)} className="text-accTx">
                    {c.name}
                  </button>
                )}
              </span>
            ))}
          </span>
        )}
        <span className="flex-1" />
        <Segmented
          label="View"
          value={view}
          onChange={setView}
          options={[
            { value: "files", label: "Files" },
            { value: "history", label: "History" },
          ]}
        />
      </div>
      {/* The view scrolls within the section, as the branches do, beside them. */}
      <div className="min-h-0 flex-1 overflow-auto">
        {view === "history" ? (
          <History project={project} at={at} />
        ) : file ? (
          <FileView
            project={project}
            at={at}
            path={path}
            onUp={() => open(folderOf(path), false)}
          />
        ) : (
          <Folder project={project} at={at} path={path} onOpen={open} />
        )}
      </div>
    </div>
  );
}

function Folder({
  project,
  at,
  path,
  onOpen,
}: {
  project: string;
  at: At;
  path: string;
  onOpen: (path: string, isFile: boolean) => void;
}) {
  const tree = useRepoTree(project, at.repo, at.branch, path);
  if (tree.isPending) return <Loading rows={6} />;
  if (tree.isError) return <p className="m-0 text-[13px] text-err">{tree.error.message}</p>;
  if (tree.data.length === 0) return <Empty>Nothing committed here yet.</Empty>;
  return (
    <Panel>
      {tree.data.map((e) => {
        const to = path ? `${path}/${e.name}` : e.name;
        const name = e.kind === "dir" ? `${e.name}/` : e.name;
        return e.kind === "module" ? (
          <div
            key={e.name}
            className="flex items-center gap-3 border-b border-line px-4 py-2 last:border-b-0"
          >
            <span className="flex-1 font-mono text-[13px] text-tx3">{name}</span>
            <span className="text-[12px] text-tx4">submodule</span>
          </div>
        ) : (
          <button
            key={e.name}
            type="button"
            onClick={() => onOpen(to, e.kind !== "dir")}
            className="flex w-full items-center gap-3 border-b border-line px-4 py-2 text-left last:border-b-0 hover:bg-hover"
          >
            <span
              className={`flex-1 truncate font-mono text-[13px] ${e.kind === "dir" ? "text-tx" : "text-tx2"}`}
            >
              {name}
            </span>
            {e.kind === "link" && <span className="text-[12px] text-tx4">link</span>}
            {e.kind === "file" && <span className="text-[12px] text-tx4">{bytes(e.size)}</span>}
          </button>
        );
      })}
    </Panel>
  );
}

function FileView({
  project,
  at,
  path,
  onUp,
}: {
  project: string;
  at: At;
  path: string;
  onUp: () => void;
}) {
  const file = useRepoFile(project, at.repo, at.branch, path);
  const markdown = isMarkdown(path);
  const [source, setSource] = useState(false);
  if (file.isPending) return <Loading rows={8} />;
  if (file.isError) return <p className="m-0 text-[13px] text-err">{file.error.message}</p>;
  const f = file.data;
  const note = f.binary
    ? `A binary file (${bytes(f.size)}): not shown.`
    : f.tooLarge
      ? `Too large to show here (${bytes(f.size)}).`
      : undefined;
  return (
    <div className="flex flex-col gap-2.5">
      {/* Stays in view while the file scrolls. */}
      <div className="sticky top-0 z-10 flex items-center gap-3 bg-bg pb-1">
        <Button variant="small" onClick={onUp}>
          ← Folder
        </Button>
        <span className="text-[12.5px] text-tx4">{bytes(f.size)}</span>
        <span className="flex-1" />
        {markdown && !note && (
          <Segmented
            label="Show"
            value={source ? "source" : "rendered"}
            onChange={(v) => setSource(v === "source")}
            options={[
              { value: "rendered", label: "Rendered" },
              { value: "source", label: "Source" },
            ]}
          />
        )}
      </div>
      {note ? (
        <Empty>{note}</Empty>
      ) : markdown && !source ? (
        <Panel className="px-6 py-5">
          <div className="wiki">
            <FileMarkdown text={f.text} />
          </div>
        </Panel>
      ) : (
        <Lines text={f.text} />
      )}
    </div>
  );
}

/** A file's text with its line numbers, which selecting the text leaves out. */
function Lines({ text }: { text: string }) {
  const lines = text.replace(/\n$/, "").split("\n");
  const width = String(lines.length).length;
  return (
    <pre className="m-0 overflow-x-auto rounded-xl border border-line bg-term py-3 font-mono text-[12.5px] leading-[1.6] text-tx2 select-text">
      {lines.map((line, i) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: a file's lines are positional
        <div key={i} className="flex">
          <span
            className="shrink-0 pr-4 pl-4 text-right text-tx4 select-none"
            style={{ width: `${width + 3}ch` }}
          >
            {i + 1}
          </span>
          <span className="pr-4 whitespace-pre">{line}</span>
        </div>
      ))}
    </pre>
  );
}

/** A branch's history, or every branch's, drawn as a graph (the CEO,
 * 2026-11-01): each commit on its lane, the lines to its parents. */
function History({ project, at }: { project: string; at: At }) {
  const [all, setAll] = useState(false);
  const log = useRepoLog(project, at.repo, at.branch, all);
  const commits = log.data?.pages.flatMap((p) => p.commits) ?? [];
  const rows = graph(commits);
  const width = Math.max(1, ...rows.map((r) => r.width));
  return (
    <div className="flex flex-col gap-3">
      <Segmented
        label="Which history"
        value={all ? "all" : "branch"}
        onChange={(v) => setAll(v === "all")}
        options={[
          { value: "branch", label: at.branch },
          { value: "all", label: "All branches" },
        ]}
      />
      {log.isPending && <Loading rows={6} />}
      {log.isError && <p className="m-0 text-[13px] text-err">{log.error.message}</p>}
      {commits.length > 0 && (
        <Panel>
          {commits.map((c, i) => (
            <div
              key={c.hash}
              className="flex h-[52px] items-center gap-3 border-b border-line pr-4 last:border-b-0"
            >
              <Lanes row={rows[i]} width={width} />
              <span className="font-mono text-[12px] text-tx4">{c.hash.slice(0, 7)}</span>
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="truncate text-[13.5px] text-tx">
                  {c.subject}
                  {c.parents.length > 1 && (
                    <span className="ml-2 text-[11.5px] text-tx4">merge</span>
                  )}
                </span>
                {/* The branches at this commit, as a second line. */}
                {c.branches.length > 0 && (
                  <span className="truncate font-mono text-[11.5px] text-accTx">
                    {c.branches.join(", ")}
                  </span>
                )}
              </span>
              <span className="shrink-0 text-[12.5px] text-tx3">{c.author}</span>
              <span className="shrink-0 text-[12px] text-tx4">{when(c.date)}</span>
            </div>
          ))}
        </Panel>
      )}
      {log.hasNextPage && (
        <Button
          className="self-start"
          disabled={log.isFetchingNextPage}
          onClick={() => log.fetchNextPage()}
        >
          Show more
        </Button>
      )}
    </div>
  );
}

const laneColors = ["var(--acc)", "var(--info)", "var(--ok)", "var(--tx3)"];
const laneGap = 12;
const x = (lane: number) => 10 + lane * laneGap;

/** One row's slice of the graph, the full row's height: lines in from the
 * row above, out to the parents below, and the commit's dot. */
function Lanes({ row, width }: { row: GraphRow; width: number }) {
  const color = (lane: number) => laneColors[lane % laneColors.length];
  return (
    <svg
      aria-hidden
      width={x(width - 1) + 10}
      height={52}
      className="shrink-0"
      viewBox={`0 0 ${x(width - 1) + 10} 52`}
    >
      {row.top.map(([from, to]) => (
        <line
          key={`t${from}`}
          x1={x(from)}
          y1={0}
          x2={x(to)}
          y2={26}
          stroke={color(from)}
          strokeWidth={1.5}
        />
      ))}
      {row.bottom.map(([from, to]) => (
        <line
          key={`b${from}-${to}`}
          x1={x(from)}
          y1={26}
          x2={x(to)}
          y2={52}
          stroke={color(to)}
          strokeWidth={1.5}
        />
      ))}
      <circle cx={x(row.lane)} cy={26} r={4} fill={color(row.lane)} />
    </svg>
  );
}
