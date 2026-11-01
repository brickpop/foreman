import * as Menu from "@radix-ui/react-dropdown-menu";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";
import { type Project, type Repo, useClient } from "../../api/client";
import { readable } from "../../app/model";
import { keys, useDeployments, useRepos } from "../../app/queries";
import { AddRepo } from "../../components/AddRepo";
import { Confirm, Modal } from "../../components/Overlays";
import { RepoPicker } from "../../components/RepoPicker";
import { Select } from "../../components/Select";
import { useToast } from "../../components/Toasts";
import { Button, cover, GridCard, Loading } from "../../components/ui";
import { useProject } from "../Project";

// A project's Repos section (wiki/product/features/repos.md): a card per
// code repo, wherever it is on disk, each opening its own page (Repo), with
// what keeps agents from using it; then its project repo. Added, created,
// located, renamed, removed and given their main branch here, from each
// card's ⋯ menu; renaming and removing wait until no team is deployed.

type Changing = { what: "main" | "rename" | "locate" | "remove"; repo: string };

export function Repos() {
  const project = useProject();
  const client = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  // The branches, and the project repo, from the repos read.
  const read = useRepos(project.id);
  const deployed = (useDeployments().data ?? []).some((d) => d.projectId === project.id);
  const [adding, setAdding] = useState(false);
  const [changing, setChanging] = useState<Changing>();
  const [renameTo, setRenameTo] = useState("");
  const close = () => setChanging(undefined);
  const change = useMutation({
    mutationFn: (run: () => Promise<Project>) => run(),
    onSuccess: () => queries.invalidateQueries({ queryKey: keys.projects }),
    onError: (e: Error) =>
      toast({ title: "Repos not changed", body: e.message, color: "var(--err)" }),
  });
  const readOf = (name: string) => read.data?.find((r) => r.name === name);
  const projectRepo = read.data?.find((r) => r.project);
  const open = (what: Changing["what"], repo: string) => {
    if (what === "rename") setRenameTo(repo);
    setChanging({ what, repo });
  };
  const target = project.repos.find((r) => r.name === changing?.repo);
  return (
    <div className="flex max-w-[920px] flex-col gap-4 px-6 pt-6 pb-12">
      {project.repos.length === 0 && (
        <p className="m-0 text-[13.5px] text-err">
          No code repo yet: agents need at least one to work on.
        </p>
      )}
      <div className="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-3">
        {project.repos.map((r) => (
          <RepoCard
            key={r.name}
            project={project.id}
            repo={r}
            read={readOf(r.name)}
            deployed={deployed}
            onChange={open}
          />
        ))}
        {read.isPending && <Loading rows={2} />}
        {projectRepo && (
          <Card project={project.id} name={projectRepo.name} title="Project repo">
            <span className="text-[12.5px] text-tx4">project.toml and the wiki</span>
            <span className="text-[12.5px] text-tx3">{count(projectRepo)}</span>
          </Card>
        )}
      </div>
      {read.isError && <p className="m-0 text-[13px] text-err">{read.error.message}</p>}
      <span>
        <Button variant="small" onClick={() => setAdding(true)}>
          Add a repo
        </Button>
      </span>
      <Modal
        open={adding}
        onClose={() => setAdding(false)}
        title={`Add a repo to ${readable(project.id)}`}
      >
        {adding && <AddRepo project={project.id} onAdded={() => setAdding(false)} />}
      </Modal>
      <Modal
        open={changing?.what === "main"}
        onClose={close}
        title={`Main branch of ${target ? readable(target.name) : ""}`}
      >
        {target && (
          <div className="flex flex-col gap-3">
            <p className="m-0 text-[13.5px] text-tx2">
              Its agents start from it, and its merges go to it.
            </p>
            <Select
              label={`Main branch of ${target.name}`}
              value={target.main}
              mono
              className="self-start"
              options={(readOf(target.name)?.branches ?? []).map((b) => ({
                value: b.name,
                label: b.name,
              }))}
              onChange={(main) =>
                main !== target.main &&
                change.mutate(() => client.home.setRepoMain(project.id, target.name, main), {
                  onSuccess: close,
                })
              }
            />
          </div>
        )}
      </Modal>
      <Modal
        open={changing?.what === "rename"}
        onClose={close}
        title={`Rename ${changing?.repo ?? ""}`}
      >
        {changing && (
          <form
            className="flex items-center gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              const to = renameTo.trim();
              if (to && to !== changing.repo)
                change.mutate(() => client.home.renameRepo(project.id, changing.repo, to), {
                  onSuccess: close,
                });
            }}
          >
            <input
              value={renameTo}
              onChange={(e) => setRenameTo(e.target.value)}
              aria-label={`New name for ${changing.repo}`}
              spellCheck={false}
              // biome-ignore lint/a11y/noAutofocus: opened by Rename, to type the name
              autoFocus
              className="min-w-0 flex-1 rounded-lg border border-line2 bg-term px-2.5 py-1 font-mono text-[13px] text-tx"
            />
            <Button type="submit" variant="small" disabled={change.isPending}>
              Rename
            </Button>
          </form>
        )}
      </Modal>
      <Modal
        open={changing?.what === "locate"}
        onClose={close}
        title={`Locate ${changing?.repo ?? ""}`}
      >
        <div className="flex flex-col gap-3">
          <p className="m-0 text-[13.5px] text-tx2">
            Pick its folder now. It must be the same repo: Foreman checks they share a commit.
          </p>
          {changing?.what === "locate" && (
            <RepoPicker
              pick="repo"
              onPick={(path) =>
                change.mutate(() => client.home.locateRepo(project.id, changing.repo, path), {
                  onSuccess: close,
                })
              }
            />
          )}
        </div>
      </Modal>
      <Confirm
        open={changing?.what === "remove"}
        title={`Remove ${changing?.repo ?? ""} from ${readable(project.id)}?`}
        body="Its folder and its branches stay where they are; agents' worktrees of it go. Add it again any time."
        action="Remove"
        onConfirm={() =>
          changing && change.mutate(() => client.home.removeRepo(project.id, changing.repo))
        }
        onClose={close}
      />
    </div>
  );
}

const count = (r: Repo) =>
  `${r.branches.length} ${r.branches.length === 1 ? "branch" : "branches"}`;

/** A code repo's card: its name as words, its folder, its main branch and
 * branch count, and what keeps agents from using it. */
function RepoCard({
  project,
  repo: r,
  read,
  deployed,
  onChange,
}: {
  project: string;
  repo: Project["repos"][number];
  read?: Repo;
  deployed: boolean;
  onChange: (what: Changing["what"], repo: string) => void;
}) {
  const noMain = read && !read.branches.some((b) => b.name === r.main);
  return (
    <Card
      project={project}
      name={r.name}
      title={readable(r.name)}
      opens={!r.problem}
      menu={
        <RepoMenu repo={r.name} problem={!!r.problem} deployed={deployed} onChange={onChange} />
      }
    >
      <span className="font-mono text-[12px] break-all text-tx4">{r.path}</span>
      {r.problem ? (
        <span className="text-[13px] text-err">{r.problem}</span>
      ) : (
        <span className="text-[12.5px] text-tx3">
          <span className="font-mono">{r.main}</span>
          {read && ` · ${count(read)}`}
        </span>
      )}
      {noMain && (
        <span className="text-[13px] text-err">
          No branch {r.main}: pick its main branch, or agents can't start.
        </span>
      )}
    </Card>
  );
}

/** A card, all of it opening the repo's page (unless it can't be read); its
 * menu sits above that link. */
function Card({
  project,
  name,
  title,
  opens = true,
  menu,
  children,
}: {
  project: string;
  name: string;
  title: string;
  opens?: boolean;
  menu?: ReactNode;
  children: ReactNode;
}) {
  return (
    <GridCard
      opens={opens}
      head={
        <>
          {opens ? (
            <Link
              to="/projects/$project/repos/$repo"
              params={{ project, repo: name }}
              title={name}
              className={`min-w-0 flex-1 truncate text-[15px] font-semibold text-tx ${cover}`}
            >
              {title}
            </Link>
          ) : (
            <span
              title={name}
              className="min-w-0 flex-1 truncate text-[15px] font-semibold text-tx"
            >
              {title}
            </span>
          )}
          {menu && <span className="relative z-10">{menu}</span>}
        </>
      }
    >
      <div className="flex flex-col gap-1.5">{children}</div>
    </GridCard>
  );
}

const item =
  "cursor-pointer rounded px-2 py-1 text-[13px] text-tx2 outline-none data-[highlighted]:bg-hover data-[disabled]:cursor-default data-[disabled]:text-tx4";

function RepoMenu({
  repo,
  problem,
  deployed,
  onChange,
}: {
  repo: string;
  problem: boolean;
  deployed: boolean;
  onChange: (what: Changing["what"], repo: string) => void;
}) {
  return (
    <Menu.Root>
      <Menu.Trigger
        aria-label={`Change ${repo}`}
        className="grid size-7 place-items-center rounded-md text-tx3 hover:bg-hover hover:text-tx data-[state=open]:bg-hover"
      >
        ⋯
      </Menu.Trigger>
      <Menu.Portal>
        <Menu.Content
          align="end"
          sideOffset={4}
          className="z-50 flex min-w-44 flex-col rounded-lg border border-line2 bg-panel p-1 shadow-overlay"
        >
          {problem ? (
            <Menu.Item className={item} onSelect={() => onChange("locate", repo)}>
              Locate…
            </Menu.Item>
          ) : (
            <Menu.Item className={item} onSelect={() => onChange("main", repo)}>
              Main branch…
            </Menu.Item>
          )}
          <Menu.Item className={item} disabled={deployed} onSelect={() => onChange("rename", repo)}>
            Rename…
          </Menu.Item>
          <Menu.Item className={item} disabled={deployed} onSelect={() => onChange("remove", repo)}>
            Remove…
          </Menu.Item>
          {deployed && (
            <Menu.Label className="max-w-56 px-2 py-1 text-[11.5px] text-tx4">
              Stop the team on this project first to rename or remove it.
            </Menu.Label>
          )}
        </Menu.Content>
      </Menu.Portal>
    </Menu.Root>
  );
}
