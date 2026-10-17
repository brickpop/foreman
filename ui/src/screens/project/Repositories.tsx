import { useState } from "react";
import type { Project, Repo } from "../../api/client";
import { when } from "../../app/model";
import { useRepoFile, useRepoLog, useRepos, useRepoTree, useTeams } from "../../app/queries";
import { branchOwner, bytes, crumbs, folderOf, isMarkdown } from "../../app/repos";
import { FileMarkdown } from "../../components/Markdown";
import { Button, Empty, Panel, SectionLabel, Segmented } from "../../components/ui";
import { InFlight } from "./Pipeline";

// The project's Repositories tab (wiki/product/features/repositories.md): the
// work on its way to main, then its repos, read-only and as committed: each
// repo's branches, and a branch's files and history.

type At = { repo: string; branch: string };

export function Repositories({ project }: { project: Project }) {
  const repos = useRepos(project.id);
  const [chosen, setChosen] = useState<At>();
  const list = repos.data ?? [];
  // Until the CEO picks one: the first repo's first branch (main, when it has one).
  const at =
    chosen ??
    (list[0] && { repo: list[0].name, branch: list[0].branches[0]?.name ?? list[0].main });
  return (
    <div className="flex flex-col gap-9 px-12 pt-6 pb-16">
      <InFlight project={project} />
      <section className="flex flex-col gap-2.5">
        <SectionLabel>Repositories</SectionLabel>
        {repos.isError && <p className="m-0 text-[13px] text-err">{repos.error.message}</p>}
        {at && (
          <div className="grid grid-cols-[minmax(180px,240px)_minmax(0,1fr)] gap-6">
            <Branches repos={list} at={at} onChoose={setChosen} />
            {/* Keyed by branch: another branch starts at its root. */}
            <Browser key={`${at.repo} ${at.branch}`} project={project.id} at={at} />
          </div>
        )}
      </section>
    </div>
  );
}

/** Each repo with its branches: main first, then whose each is, how far ahead
 * of main, and whether its worktree holds uncommitted changes. */
function Branches({ repos, at, onChoose }: { repos: Repo[]; at: At; onChoose: (at: At) => void }) {
  const teams = useTeams().data ?? [];
  return (
    <nav className="flex min-w-0 flex-col gap-4">
      {repos.map((r) => (
        <div key={r.name} className="flex flex-col gap-0.5">
          <span className="font-mono text-[13px] font-semibold">
            {r.name}/
            {r.project && <span className="font-sans font-normal text-tx4"> project repo</span>}
          </span>
          {r.branches.map((b) => {
            const on = at.repo === r.name && at.branch === b.name;
            const owner = branchOwner(b.name, teams);
            return (
              <button
                key={b.name}
                type="button"
                aria-pressed={on}
                onClick={() => onChoose({ repo: r.name, branch: b.name })}
                className={`flex min-w-0 flex-col rounded-md px-2 py-1 text-left ${on ? "bg-accSoft" : "hover:bg-hover"}`}
              >
                <span
                  className={`truncate font-mono text-[12.5px] ${on ? "text-accTx" : "text-tx2"}`}
                >
                  {b.name}
                </span>
                {(owner || b.ahead > 0 || b.uncommitted) && (
                  <span className="text-[11.5px] text-tx4">
                    {[owner, b.ahead > 0 && `${b.ahead} ahead of ${r.main}`]
                      .filter(Boolean)
                      .join(" · ")}
                    {b.uncommitted && (
                      <span className="text-accTx">
                        {owner || b.ahead > 0 ? " · " : ""}uncommitted changes
                      </span>
                    )}
                  </span>
                )}
              </button>
            );
          })}
        </div>
      ))}
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
    <div className="flex min-w-0 flex-col gap-3">
      <div className="flex flex-wrap items-center gap-3">
        <Segmented
          label="View"
          value={view}
          onChange={setView}
          options={[
            { value: "files", label: "Files" },
            { value: "history", label: "History" },
          ]}
        />
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
      </div>
      {view === "history" ? (
        <History project={project} at={at} />
      ) : file ? (
        <FileView project={project} at={at} path={path} onUp={() => open(folderOf(path), false)} />
      ) : (
        <Folder project={project} at={at} path={path} onOpen={open} />
      )}
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
  if (tree.isPending) return null;
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
  if (file.isPending) return null;
  if (file.isError) return <p className="m-0 text-[13px] text-err">{file.error.message}</p>;
  const f = file.data;
  const note = f.binary
    ? `A binary file (${bytes(f.size)}): not shown.`
    : f.tooLarge
      ? `Too large to show here (${bytes(f.size)}).`
      : undefined;
  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex items-center gap-3">
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

function History({ project, at }: { project: string; at: At }) {
  const log = useRepoLog(project, at.repo, at.branch);
  if (log.isPending) return null;
  if (log.isError) return <p className="m-0 text-[13px] text-err">{log.error.message}</p>;
  const commits = log.data.pages.flatMap((p) => p.commits);
  return (
    <div className="flex flex-col gap-3">
      <Panel>
        {commits.map((c) => (
          <div
            key={c.hash}
            className="flex items-baseline gap-3 border-b border-line px-4 py-2.5 last:border-b-0"
          >
            <span className="font-mono text-[12px] text-tx4">{c.hash.slice(0, 7)}</span>
            <span className="min-w-0 flex-1 truncate text-[13.5px] text-tx">
              {c.subject}
              {c.merge && <span className="ml-2 text-[11.5px] text-tx4">merge</span>}
            </span>
            <span className="shrink-0 text-[12.5px] text-tx3">{c.author}</span>
            <span className="shrink-0 text-[12px] text-tx4">{when(c.date)}</span>
          </div>
        ))}
      </Panel>
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
