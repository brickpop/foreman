import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";
import { type Project, useClient } from "../../api/client";
import { keys, useBranches, useDeployments, useRepos } from "../../app/queries";
import { AddRepo } from "../../components/AddRepo";
import { Confirm, Modal } from "../../components/Overlays";
import { RepoPicker } from "../../components/RepoPicker";
import { Select } from "../../components/Select";
import { useToast } from "../../components/Toasts";
import { Button, Loading, Panel } from "../../components/ui";
import { useProject } from "../Project";

// A project's Repos section (wiki/product/features/repos.md): its code repos,
// wherever they are on disk, each with what keeps agents from using it, then
// its project repo; each opens its own page (Repo). Added, created, located,
// renamed and removed here (in Settings until 2026-11-01); renaming and
// removing wait until no team is deployed on the project.

export function Repos() {
  const project = useProject();
  const client = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  // The branches, and the project repo, from the repos read.
  const read = useRepos(project.id);
  const deployed = (useDeployments().data ?? []).some((d) => d.projectId === project.id);
  const [adding, setAdding] = useState(false);
  const [locating, setLocating] = useState<string>();
  const [renaming, setRenaming] = useState<{ name: string; to: string }>();
  const [removing, setRemoving] = useState<string>();
  const change = useMutation({
    mutationFn: (run: () => Promise<Project>) => run(),
    onSuccess: () => queries.invalidateQueries({ queryKey: keys.projects }),
    onError: (e: Error) =>
      toast({ title: "Repos not changed", body: e.message, color: "var(--err)" }),
  });
  const stopFirst = deployed ? "Stop the team on this project first" : undefined;
  const branches = (name: string) => {
    const n = read.data?.find((r) => r.name === name)?.branches.length;
    return n === undefined ? "" : `${n} ${n === 1 ? "branch" : "branches"}`;
  };
  const projectRepo = read.data?.find((r) => r.project);
  return (
    <div className="flex max-w-[920px] flex-col gap-4 px-6 pt-6 pb-12">
      {project.repos.length === 0 && (
        <p className="m-0 text-[13.5px] text-err">
          No code repo yet: agents need at least one to work on.
        </p>
      )}
      <Panel>
        {project.repos.map((r) => (
          <div
            key={r.name}
            className="flex flex-col gap-1.5 border-b border-line px-4 py-3 last:border-b-0"
          >
            {renaming?.name === r.name ? (
              <form
                className="flex items-center gap-2"
                onSubmit={(e) => {
                  e.preventDefault();
                  const to = renaming.to.trim();
                  if (to && to !== r.name)
                    change.mutate(() => client.home.renameRepo(project.id, r.name, to), {
                      onSuccess: () => setRenaming(undefined),
                    });
                }}
              >
                <input
                  value={renaming.to}
                  onChange={(e) => setRenaming({ name: r.name, to: e.target.value })}
                  aria-label={`New name for ${r.name}`}
                  spellCheck={false}
                  // biome-ignore lint/a11y/noAutofocus: opened by the Rename click, to type the name
                  autoFocus
                  className="min-w-0 flex-1 rounded-lg border border-line2 bg-term px-2.5 py-1 font-mono text-[13px] text-tx"
                />
                <Button type="submit" variant="small" disabled={change.isPending}>
                  Rename
                </Button>
                <Button variant="small" onClick={() => setRenaming(undefined)}>
                  Cancel
                </Button>
              </form>
            ) : (
              <span className="flex items-center gap-3">
                <span className="flex min-w-0 flex-1 flex-col">
                  {r.problem ? (
                    <span className="font-mono text-[14px] font-semibold text-tx">{r.name}/</span>
                  ) : (
                    <RepoLink project={project.id} repo={r.name} />
                  )}
                  <span className="font-mono text-[12.5px] break-all text-tx4">{r.path}</span>
                </span>
                <span className="text-[12.5px] text-tx4">{branches(r.name)}</span>
                {r.problem && (
                  <Button variant="small" onClick={() => setLocating(r.name)}>
                    Locate…
                  </Button>
                )}
                <Button
                  variant="small"
                  disabled={deployed}
                  title={stopFirst}
                  onClick={() => setRenaming({ name: r.name, to: r.name })}
                >
                  Rename
                </Button>
                <Button
                  variant="small"
                  disabled={deployed}
                  title={stopFirst}
                  onClick={() => setRemoving(r.name)}
                >
                  Remove
                </Button>
              </span>
            )}
            {r.problem && <span className="text-[13px] text-err">{r.problem}</span>}
            {!r.problem && (
              <RepoMain
                repo={r}
                onPick={(main) =>
                  change.mutate(() => client.home.setRepoMain(project.id, r.name, main))
                }
              />
            )}
          </div>
        ))}
        {read.isPending && <Loading rows={1} />}
        {projectRepo && (
          <div className="flex items-center gap-3 px-4 py-3">
            <span className="flex min-w-0 flex-1 flex-col">
              <RepoLink project={project.id} repo={projectRepo.name} />
              <span className="text-[12.5px] text-tx4">
                The project repo: project.toml and the wiki
              </span>
            </span>
            <span className="text-[12.5px] text-tx4">{branches(projectRepo.name)}</span>
          </div>
        )}
      </Panel>
      {read.isError && <p className="m-0 text-[13px] text-err">{read.error.message}</p>}
      <span>
        <Button variant="small" onClick={() => setAdding(true)}>
          Add a repo
        </Button>
      </span>
      <Modal open={adding} onClose={() => setAdding(false)} title={`Add a repo to ${project.id}`}>
        {adding && <AddRepo project={project.id} onAdded={() => setAdding(false)} />}
      </Modal>
      <Modal
        open={locating !== undefined}
        onClose={() => setLocating(undefined)}
        title={`Locate ${locating}`}
      >
        <div className="flex flex-col gap-3">
          <p className="m-0 text-[13.5px] text-tx2">
            Pick its folder now. It must be the same repo: Foreman checks they share a commit.
          </p>
          {locating && (
            <RepoPicker
              pick="repo"
              onPick={(path) =>
                change.mutate(() => client.home.locateRepo(project.id, locating, path), {
                  onSuccess: () => setLocating(undefined),
                })
              }
            />
          )}
        </div>
      </Modal>
      <Confirm
        open={removing !== undefined}
        title={`Remove ${removing} from ${project.id}?`}
        body="Its folder and its branches stay where they are; agents' worktrees of it go. Add it again any time."
        action="Remove"
        onConfirm={() =>
          removing && change.mutate(() => client.home.removeRepo(project.id, removing))
        }
        onClose={() => setRemoving(undefined)}
      />
    </div>
  );
}

/** A repo's name, opening its page. */
function RepoLink({ project, repo }: { project: string; repo: string }) {
  return (
    <Link
      to="/projects/$project/repos/$repo"
      params={{ project, repo }}
      className="self-start font-mono text-[14px] font-semibold text-tx hover:text-accTx"
    >
      {repo}/
    </Link>
  );
}

/** A repo's main branch, which its agents start from and its merges go to:
 * any of its branches. One it no longer has (renamed, or from before each repo
 * had its own) is flagged, since agents can't start without it. */
function RepoMain({
  repo,
  onPick,
}: {
  repo: Project["repos"][number];
  onPick: (main: string) => void;
}) {
  const branches = useBranches(repo.path);
  const list = branches.data?.branches ?? [];
  return (
    <span className="flex flex-wrap items-center gap-2 text-[13px] text-tx3">
      Main branch
      <Select
        label={`Main branch of ${repo.name}`}
        value={repo.main}
        mono
        options={list.map((b) => ({ value: b, label: b }))}
        onChange={(b) => b !== repo.main && onPick(b)}
      />
      {branches.data && !list.includes(repo.main) && (
        <span className="text-err">
          {repo.name} has no branch {repo.main}: pick its main branch, or agents can't start.
        </span>
      )}
    </span>
  );
}
