import * as Popover from "@radix-ui/react-popover";
import { Link, useNavigate, useParams, useRouterState, useSearch } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { type RepoBranch, type Repo as RepoInfo, useClient } from "../../api/client";
import { withoutTitle } from "../../app/frontmatter";
import { projectName, repoName, repoTitle, when } from "../../app/model";
import {
  useDeploymentOn,
  useRepoFile,
  useRepoLog,
  useRepos,
  useRepoTree,
  useTeams,
} from "../../app/queries";
import {
  branchOwner,
  bytes,
  crumbs,
  folderOf,
  folderPage,
  type GraphRow,
  graph,
  matching,
  readmeOf,
  shownAt,
} from "../../app/repos";
import { TreeChildren, TreeRow } from "../../components/FileTree";
import { Frontmatter, FrontmatterError } from "../../components/Frontmatter";
import { Markdown, type ResolvedLink } from "../../components/Markdown";
import { Button, Empty, Loading, Panel, Segmented } from "../../components/ui";
import { useProject } from "../Project";

// A repo's page in the project's Repos section (wiki/product/features/repos.md),
// laid out as GitHub does (the CEO, 2026-11-02): read-only and as committed,
// the branch in a dropdown, its latest commit above the files, the file tree
// beside the file open (its README until one is picked), and the history
// taking the page when asked for. Branch, file and history are in the URL, so
// a link (an agent's worktree, on Deployment) opens them.

type At = { repo: string; branch: string };
/** Opens a link of a rendered file: its file, or its folder's page. */
type Open = (link: ResolvedLink) => void;

export function Repo() {
  const project = useProject();
  const repos = useRepos(project.id);
  const { repo: id } = useParams({ from: "/projects/$project/repos/$repo" });
  const search = useSearch({ from: "/projects/$project/repos/$repo" });
  const navigate = useNavigate({ from: "/projects/$project/repos/$repo" });
  const list = repos.data ?? [];
  const repo = list.find((r) => r.id === id);
  const at = shownAt(list, id, search.branch);
  // Each is a step Back returns from (the CEO, 2026-11-02): another branch, a
  // file, the history.
  const go = (next: typeof search) => navigate({ search: (s) => ({ ...s, ...next }) });
  // A link in a rendered file: its file, or its folder's page, at its anchor.
  const open = (l: ResolvedLink) =>
    navigate({
      search: (s) => ({ ...s, path: l.folder ? (l.to ? `${l.to}/` : undefined) : l.to }),
      hash: l.anchor || undefined,
    });
  return (
    <div className="flex h-full min-h-[420px] flex-col gap-3 px-6 pt-6 pb-6">
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
        <h1
          title={repoTitle(project, id)}
          className="m-0 min-w-0 truncate text-[18px] font-semibold"
        >
          {repoName(project, id)}
        </h1>
        <span className="font-mono text-[12.5px] text-tx4">{id}/</span>
      </div>
      {repos.isError && <p className="m-0 text-[13px] text-err">{repos.error.message}</p>}
      {repos.isPending && <Loading rows={6} />}
      {repos.data && !repo && (
        <Empty>
          {projectName([project], project.id)} has no repo {id}.
        </Empty>
      )}
      {repo && at && (
        <>
          <div className="flex min-w-0 items-center gap-3">
            <BranchPicker repo={repo} at={at} onChoose={(branch) => go({ branch })} />
            <LastCommit
              project={project.id}
              at={at}
              history={search.history === true}
              onHistory={(on) => go({ history: on ? true : undefined })}
            />
          </div>
          {search.history ? (
            <div className="min-h-0 flex-1 overflow-auto">
              <History project={project.id} at={at} />
            </div>
          ) : (
            <div className="grid min-h-0 flex-1 grid-cols-[minmax(180px,260px)_minmax(0,1fr)] gap-6">
              {/* Keyed by branch: another branch's tree starts folded. */}
              <Tree
                key={at.branch}
                project={project.id}
                at={at}
                path={search.path ?? ""}
                onPick={(path) => go({ path })}
              />
              <div className="min-h-0 min-w-0 overflow-auto">
                {search.path?.endsWith("/") ? (
                  <FolderView
                    project={project.id}
                    at={at}
                    path={search.path.slice(0, -1)}
                    onOpen={open}
                  />
                ) : search.path ? (
                  <FileView project={project.id} at={at} path={search.path} onOpen={open} />
                ) : (
                  <Readme project={project.id} at={at} onOpen={open} />
                )}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}

/** The branch shown, as a button opening the repo's branches to pick from:
 * a box to find one by name, then the default first and the most recent,
 * each with whose it is, whether it is the default, how far ahead of it,
 * and whether its worktree has
 * uncommitted changes. */
function BranchPicker({
  repo,
  at,
  onChoose,
}: {
  repo: RepoInfo;
  at: At;
  onChoose: (branch: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const shown = matching(repo.branches, query);
  const choose = (branch: string) => {
    onChoose(branch);
    setOpen(false);
    setQuery("");
  };
  return (
    <Popover.Root
      open={open}
      onOpenChange={(o) => {
        setOpen(o);
        if (!o) setQuery("");
      }}
    >
      <Popover.Trigger
        aria-label="Branch"
        className="flex h-12 max-w-[320px] shrink-0 items-center gap-2 rounded-lg border border-line2 bg-panel px-3 text-[13px] text-tx hover:border-line3 data-[state=open]:border-acc"
      >
        <span className="truncate font-mono">{at.branch}</span>
        <span className="text-[10px] text-tx4">▾</span>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          align="start"
          sideOffset={4}
          className="z-50 flex w-[380px] max-w-[calc(100vw-32px)] flex-col gap-1 rounded-lg border border-line2 bg-panel p-1.5 shadow-overlay"
        >
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && shown[0]) choose(shown[0].name);
            }}
            placeholder="Find a branch…"
            aria-label="Find a branch"
            spellCheck={false}
            className="rounded-md border border-line2 bg-term px-2.5 py-1.5 font-mono text-[12.5px] text-tx"
          />
          <div className="flex max-h-[min(380px,60vh)] flex-col gap-0.5 overflow-auto">
            {shown.map((b) => (
              <BranchItem
                key={b.name}
                branch={b}
                defaultBranch={repo.defaultBranch}
                on={b.name === at.branch}
                onClick={() => choose(b.name)}
              />
            ))}
            {shown.length === 0 && (
              <span className="px-2 py-1.5 text-[12.5px] text-tx4">No branch has that name.</span>
            )}
          </div>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}

function BranchItem({
  branch: b,
  defaultBranch,
  on,
  onClick,
}: {
  branch: RepoBranch;
  defaultBranch: string;
  on: boolean;
  onClick: () => void;
}) {
  const teams = useTeams().data ?? [];
  const project = useProject();
  const deployment = useDeploymentOn(project.id)?.id;
  // The branch shown is in view when the list opens, however far down.
  const self = useRef<HTMLButtonElement>(null);
  // biome-ignore lint/correctness/useExhaustiveDependencies: once, as the list opens
  useEffect(() => {
    if (on) self.current?.scrollIntoView({ block: "nearest" });
  }, []);
  const detail = [
    b.name === defaultBranch && "default branch",
    branchOwner(b.name, project.id, teams, deployment),
    b.ahead > 0 && `${b.ahead} ahead of ${defaultBranch}`,
  ].filter(Boolean);
  return (
    <button
      ref={self}
      type="button"
      aria-pressed={on}
      onClick={onClick}
      className={`flex min-w-0 flex-col rounded-md px-2 py-1 text-left ${on ? "bg-accSoft" : "hover:bg-hover"}`}
    >
      <span className={`truncate font-mono text-[12.5px] ${on ? "text-accTx" : "text-tx2"}`}>
        {b.name}
      </span>
      {(detail.length > 0 || b.uncommitted) && (
        <span className="text-[11.5px] text-tx4">
          {detail.join(" · ")}
          {b.uncommitted && (
            <span className="text-attnTx">{detail.length > 0 ? " · " : ""}uncommitted changes</span>
          )}
        </span>
      )}
    </button>
  );
}

/** The branch's latest commit, and how many it has: a click anywhere on it
 * shows them all (the history takes the page), another goes back to the files. */
function LastCommit({
  project,
  at,
  history,
  onHistory,
}: {
  project: string;
  at: At;
  history: boolean;
  onHistory: (on: boolean) => void;
}) {
  // The history's own first page: opening it reads nothing new.
  const log = useRepoLog(project, at.repo, at.branch);
  const first = log.data?.pages[0];
  const c = first?.commits[0];
  const box =
    "flex h-12 min-w-0 flex-1 items-center gap-3 rounded-lg border border-line bg-panel px-3.5 text-left text-[13px]";
  if (!first)
    return (
      <div className={box}>
        {log.isError && <span className="truncate text-err">{log.error.message}</span>}
      </div>
    );
  // All of it is the button (the CEO, 2026-11-02): the history, or back to the files.
  return (
    <button
      type="button"
      aria-pressed={history}
      onClick={() => onHistory(!history)}
      className={`${box} hover:border-line3 ${history ? "border-accLine" : ""}`}
    >
      {c ? (
        <>
          <span className="shrink-0 text-tx3">{c.author}</span>
          <span className="min-w-0 flex-1 truncate text-tx" title={c.subject}>
            {c.subject}
          </span>
          <span className="shrink-0 font-mono text-[12px] text-tx4">{c.hash.slice(0, 7)}</span>
          <span className="shrink-0 text-[12px] text-tx4 max-[900px]:hidden">{when(c.date)}</span>
        </>
      ) : (
        <span className="flex-1 text-tx4">No commits yet.</span>
      )}
      <span className="shrink-0 rounded-md bg-raised px-2 py-1 text-[12.5px] text-tx2">
        {history ? "← Files" : `${first.total} ${first.total === 1 ? "commit" : "commits"}`}
      </span>
    </button>
  );
}

/** The branch's files as a tree: a folder opens in place (read when opened),
 * a file opens beside it. The folders above the file open are open. */
function Tree({
  project,
  at,
  path,
  onPick,
}: {
  project: string;
  at: At;
  path: string;
  onPick: (path: string) => void;
}) {
  const [open, setOpen] = useState(() => new Set(above(path)));
  // A link to a file far down opens its folders; the CEO's folding stays.
  useEffect(() => {
    setOpen((now) => new Set([...now, ...above(path)]));
  }, [path]);
  const toggle = (folder: string) =>
    setOpen((now) => {
      const next = new Set(now);
      if (!next.delete(folder)) next.add(folder);
      return next;
    });
  return (
    <nav aria-label="Files" className="flex min-h-0 flex-col overflow-auto pr-1">
      <Folder
        project={project}
        at={at}
        path=""
        depth={0}
        open={open}
        selected={path}
        onToggle={toggle}
        onPick={onPick}
      />
    </nav>
  );
}

/** The folders a path is in, outermost first. */
const above = (path: string) => crumbs(folderOf(path)).map((c) => c.path);

function Folder({
  project,
  at,
  path,
  depth,
  open,
  selected,
  onToggle,
  onPick,
}: {
  project: string;
  at: At;
  path: string;
  depth: number;
  open: Set<string>;
  selected: string;
  onToggle: (folder: string) => void;
  onPick: (path: string) => void;
}) {
  const tree = useRepoTree(project, at.repo, at.branch, path);
  if (tree.isPending) return <Loading rows={depth === 0 ? 6 : 1} />;
  if (tree.isError)
    return <p className="m-0 px-1.5 py-1 text-[12.5px] text-err">{tree.error.message}</p>;
  if (tree.data.length === 0 && depth === 0)
    return <p className="m-0 px-2 text-[13px] text-tx4">Nothing committed here yet.</p>;
  return tree.data.map((e) => {
    const to = path ? `${path}/${e.name}` : e.name;
    if (e.kind === "module")
      return (
        <TreeRow key={e.name} mono after={<span className="font-sans text-[11px]">submodule</span>}>
          {e.name}
        </TreeRow>
      );
    if (e.kind === "dir") {
      const isOpen = open.has(to);
      return (
        <div key={e.name} className="flex flex-col">
          <TreeRow folder mono open={isOpen} onClick={() => onToggle(to)}>
            {e.name}
          </TreeRow>
          {isOpen && (
            <TreeChildren>
              <Folder
                project={project}
                at={at}
                path={to}
                depth={depth + 1}
                open={open}
                selected={selected}
                onToggle={onToggle}
                onPick={onPick}
              />
            </TreeChildren>
          )}
        </div>
      );
    }
    return (
      <TreeRow
        key={e.name}
        mono
        current={to === selected}
        onClick={() => onPick(to)}
        after={e.kind === "link" && <span className="font-sans text-[11px] text-tx4">link</span>}
      >
        {e.name}
      </TreeRow>
    );
  });
}

/** What shows before a file is picked: the root's README (or index.md), else
 * a word to pick one. */
function Readme({ project, at, onOpen }: { project: string; at: At; onOpen: Open }) {
  // The tree's own root read: cached, nothing new asked.
  const root = useRepoTree(project, at.repo, at.branch, "");
  const readme = root.data && readmeOf(root.data);
  if (root.isPending) return <Loading rows={6} />;
  if (!readme) return <Empty>Pick a file on the left to read it.</Empty>;
  return <FileView project={project} at={at} path={readme} onOpen={onOpen} />;
}

/** A folder a link opened: its index.md, else its README, else what it holds. */
function FolderView({
  project,
  at,
  path,
  onOpen,
}: {
  project: string;
  at: At;
  path: string;
  onOpen: Open;
}) {
  const tree = useRepoTree(project, at.repo, at.branch, path);
  if (tree.isPending) return <Loading rows={6} />;
  if (tree.isError) return <p className="m-0 text-[13px] text-err">{tree.error.message}</p>;
  const page = folderPage(tree.data);
  if (page) return <FileView project={project} at={at} path={`${path}/${page}`} onOpen={onOpen} />;
  return (
    <div className="flex flex-col gap-2.5">
      <p className="m-0 text-[13px] text-tx3">
        <span className="font-mono text-tx2">{path}/</span> has no index.md or README. It holds:
      </p>
      <ul className="m-0 flex list-none flex-col gap-1 p-0 font-mono text-[12.5px]">
        {tree.data.map((e) => (
          <li key={e.name}>
            <button
              type="button"
              className="text-accTx hover:underline"
              onClick={() => onOpen(entryLink(`${path}/${e.name}`, e.kind === "dir"))}
            >
              {e.kind === "dir" ? `${e.name}/` : e.name}
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}

/** A link to something the tree lists: there, as the server would resolve it. */
const entryLink = (to: string, folder: boolean): ResolvedLink => ({
  raw: "",
  to,
  anchor: "",
  exists: true,
  folder,
  image: false,
  outside: false,
});

function FileView({
  project,
  at,
  path,
  onOpen,
}: {
  project: string;
  at: At;
  path: string;
  onOpen: Open;
}) {
  const client = useClient();
  const file = useRepoFile(project, at.repo, at.branch, path);
  const [source, setSource] = useState(false);
  // A link's #anchor: its heading, once the file is drawn.
  const hash = useRouterState({ select: (s) => s.location.hash });
  useEffect(() => {
    if (hash && file.data && !source) document.getElementById(hash)?.scrollIntoView();
  }, [hash, file.data, source]);
  if (file.isPending) return <Loading rows={8} />;
  if (file.isError) return <p className="m-0 text-[13px] text-err">{file.error.message}</p>;
  const f = file.data;
  const note = f.binary
    ? `A binary file (${bytes(f.size)}): not shown.`
    : f.tooLarge
      ? `Too large to show here (${bytes(f.size)}).`
      : undefined;
  const { title, fields } = withoutTitle(f.frontmatter ?? []);
  return (
    <div className="flex flex-col gap-2.5">
      {/* Stays in view while the file scrolls. */}
      <div className="sticky top-0 z-10 flex min-w-0 items-center gap-3 bg-bg pb-1">
        <span className="min-w-0 flex-1 truncate font-mono text-[12.5px] text-tx2" title={path}>
          {path}
        </span>
        <span className="shrink-0 text-[12.5px] text-tx4">{bytes(f.size)}</span>
        {f.markdown && (
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
      ) : f.markdown && !source ? (
        <div className="flex flex-col gap-3">
          <Frontmatter fields={fields} />
          {f.frontmatterError && <FrontmatterError error={f.frontmatterError} noun="file" />}
          <Panel className="px-6 py-5">
            <div className="wiki">
              {title && <h1>{title}</h1>}
              <Markdown
                body={f.body}
                links={f.links ?? []}
                headings={f.headings ?? []}
                onOpen={onOpen}
                imageUrl={(p) => client.repos.imageUrl(project, at.repo, at.branch, p)}
                scope="repo"
              />
            </div>
          </Panel>
        </div>
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

const laneColors = ["var(--attn)", "var(--info)", "var(--ok)", "var(--tx3)"];
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
