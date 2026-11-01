import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { type Project, useClient } from "../../api/client";
import { forgeLabel, harnessLabel, onOtherProfile } from "../../app/model";
import {
  keys,
  useBranches,
  useDeployments,
  useHomeFolder,
  useInstances,
  useProfiles,
  useTeams,
} from "../../app/queries";
import { AddRepo } from "../../components/AddRepo";
import { Confirm, Modal } from "../../components/Overlays";
import { RepoPicker } from "../../components/RepoPicker";
import { Select } from "../../components/Select";
import { useToast } from "../../components/Toasts";
import { Button } from "../../components/ui";
import { useProject } from "../Project";

// A project's Settings section (wiki/product/features/project-settings.md):
// its configuration. The team on it is its Deployment section.

export function ProjectSettings() {
  return <Configuration project={useProject()} />;
}

function Configuration({ project }: { project: Project }) {
  const home = useHomeFolder().data?.path ?? "~/.foreman";
  const forge = project.config.forge;
  const tree = (
    project.workspace
      ? [
          `${project.workspace}/`,
          "├─ project/           project repo (git)",
          "│  ├─ project.toml    checks, gates, forge",
          "│  └─ wiki/           knowledge base + wiki.toml (boards)",
          "└─ agents/            one sandbox per agent: a worktree of each repo",
          `${home}/`,
          "├─ config/            global repo (git): teams, settings",
          `├─ harness/${project.id}/  each agent's harness config and conversations`,
          "└─ state.db           deployments, sessions, requests (runtime)",
        ]
      : [
          `${home}/`,
          "├─ config/              global repo (git): teams, settings",
          "├─ projects/",
          `│  └─ ${project.id}/`,
          "│     ├─ project.toml    checks, gates, forge",
          "│     └─ wiki/           knowledge base + wiki.toml (boards)",
          "├─ sandboxes/",
          `│  └─ ${project.id}/      one git worktree per agent, per repo`,
          "└─ state.db             deployments, sessions, requests (runtime)",
        ]
  ).join("\n");
  return (
    <section className="grid grid-cols-[repeat(auto-fit,minmax(340px,1fr))] gap-9 px-6 pt-6 pb-12">
      <div className="flex min-w-0 max-w-[520px] flex-col gap-6">
        <Workspace project={project} />
        <Block title="Where things live">
          <pre className="m-0 overflow-x-auto font-mono text-[13px] leading-[1.7] whitespace-pre text-tx2">
            {tree}
          </pre>
        </Block>
        <Block title="How work flows">
          {[
            [
              "Team definition",
              "Lives once on this machine, in the global repo: souls, job descriptions and the playbook. You can deploy it to as many projects as you like.",
            ],
            [
              "Deployment",
              "A team on this project, one at most. It’s runtime state kept by Foreman, not config. It gets its own sessions, worktrees and message channel. Agents in two deployments never mix.",
            ],
            [
              "Session prompt",
              "Generated per agent per deployment: soul plus roster, tools, playbook and this project’s gates. Open any agent to read the exact prompt.",
            ],
            [
              "Enforced by Foreman",
              "Routes ask, reply, handoff, contribute and escalate between sessions. Tracks handoffs (Deployment → In flight), wakes requesters when replies land, sends escalations to the Dashboard, and only lets the lead request merges to main.",
            ],
          ].map(([title, body]) => (
            <div key={title} className="flex flex-col gap-0.5">
              <span className="font-semibold">{title}</span>
              <span className="leading-[1.55] text-tx2">{body}</span>
            </div>
          ))}
        </Block>
      </div>
      <div className="flex min-w-0 flex-col gap-6">
        <ProfileChoice project={project} />
        <CodeRepos project={project} />
        <Block title="What comes to you">
          <div className="grid grid-cols-[84px_1fr] gap-x-3 gap-y-2">
            {[
              [
                "approval",
                "text-accTx",
                "An agent hits one of this project’s gates (always_ask_ceo below) and pauses until you answer.",
              ],
              [
                "question",
                "text-info",
                "An agent needs information only you have. It keeps working on other things meanwhile.",
              ],
              [
                "review",
                "text-ok",
                "The lead calls request_merge() with the card that explains the work. Foreman runs the checks, then you review it: merge it, request changes, or dismiss it.",
              ],
              [
                "finding",
                "text-team",
                "A challenge team rates something high severity. You decide who fixes it, and when.",
              ],
              [
                "blocked",
                "text-err",
                "Raised by Foreman, not an agent: a session crashed, hit a rate limit, or has been stuck for 20 minutes.",
              ],
            ].map(([kind, color, line]) => (
              <div key={kind} className="contents">
                <span
                  className={`font-mono text-[10.5px] font-semibold uppercase tracking-[.05em] ${color}`}
                >
                  {kind}
                </span>
                <span className="text-[13.5px] text-tx2">{line}</span>
              </div>
            ))}
          </div>
        </Block>
        <Block title="Merges">
          <span className="font-medium">{forgeLabel(forge.kind)}</span>
          <span className="text-[13.5px] text-tx2">
            {forge.kind === "none"
              ? "No forge. You merge requested branches into main from their review, in the Inbox; Foreman does the merge."
              : `Merge to main from a branch's review, in the Inbox, or create a PR on ${forge.url} and merge it there; Foreman notices. Nothing is pushed until you choose. The token lives in Settings, never in this file.`}
          </span>
          {forge.projectRepo && (
            <span className="flex items-center gap-3 text-[13px] text-tx3">
              <span className="font-mono">project repo → {forge.projectRepo}</span>
              <Button variant="small" disabled title="Comes with the Git epic">
                Sync
              </Button>
            </span>
          )}
        </Block>
        <Block title="project.toml">
          <span className="font-mono text-[12.5px] text-tx3">
            {project.path}/project.toml · read-only
          </span>
          <p className="m-0 text-[13.5px] text-tx2">
            This file is config, committed in the project repo. Edit it in your editor.
          </p>
          {project.problems.length > 0 && (
            <p className="m-0 rounded-lg border border-err px-3 py-2 text-[13px] text-err">
              {project.problems.join(" · ")}
            </p>
          )}
          <pre className="m-0 overflow-x-auto rounded-[10px] border border-line bg-term p-3.5 font-mono text-[12.5px] leading-[1.6] text-tx2 select-text">
            {project.source}
          </pre>
          <ArchiveProject project={project} />
          <RemoveProject project={project} />
        </Block>
      </div>
    </section>
  );
}

/** The project's workspace, its folder of the CEO's, to copy; or, for a
 * project made before workspaces, the offer to move it into one (legacy:
 * wiki/backlog/remove-workspace-migration.md). */
function Workspace({ project }: { project: Project }) {
  const client = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  const [edited, setEdited] = useState<string>();
  const suggestion = useQuery({
    queryKey: ["workspace", project.id],
    queryFn: () => client.home.suggestWorkspace(project.id),
    enabled: !project.workspace,
  }).data;
  const target = edited ?? suggestion?.path ?? "";
  const migrate = useMutation({
    mutationFn: () => client.home.migrate(project.id, target.trim()),
    onSuccess: (p) => {
      queries.invalidateQueries({ queryKey: keys.projects });
      toast({ title: `${p.id} moved`, body: `Into ${p.workspace}.`, color: "var(--ok)" });
    },
    onError: (e: Error) =>
      toast({ title: "Project not moved", body: e.message, color: "var(--err)" }),
  });
  if (project.workspace) {
    return (
      <Block title="Workspace">
        <span className="flex items-center gap-3">
          <span className="min-w-0 flex-1 truncate font-mono text-[13px] text-tx2 select-text">
            {project.workspace}
          </span>
          <Button variant="small" onClick={() => navigator.clipboard.writeText(project.workspace)}>
            Copy
          </Button>
        </span>
        <span className="text-[13.5px] text-tx3">
          Its project repo, and in agents/ each agent's worktrees: look at and run their code there.
          Editing while an agent works turns into its uncommitted changes.
        </span>
      </Block>
    );
  }
  return (
    <Block title="Move into a workspace">
      <span className="text-[13.5px] text-tx2">
        This project lives in Foreman's home. Moved into a folder of yours, its project repo and its
        agents' worktrees are where you can find them and run their code. Uncommitted work and
        conversations come along. Stop the teams on it first.
      </span>
      <input
        value={target}
        onChange={(e) => setEdited(e.target.value)}
        aria-label="Workspace"
        spellCheck={false}
        className="rounded-lg border border-line2 bg-term px-3 py-2 font-mono text-[13px] text-tx"
      />
      <span>
        <Button
          variant="small"
          disabled={!target.trim() || migrate.isPending}
          onClick={() => migrate.mutate()}
        >
          {migrate.isPending ? "Moving…" : "Move"}
        </Button>
      </span>
    </Block>
  );
}

/** The profile this project's agents run on, a harness and its login. Kept in
 * Foreman's records, never project.toml, which the lead can write. */
function ProfileChoice({ project }: { project: Project }) {
  const client = useClient();
  const queries = useQueryClient();
  const profiles = useProfiles();
  const toast = useToast();
  const current = project.profile;
  const found = profiles.data?.find((p) => p.name === current);
  const choose = (name: string) =>
    client.home
      .setProfile(project.id, name)
      .then(() => queries.invalidateQueries({ queryKey: keys.projects }))
      .catch((e: Error) =>
        toast({ title: "Profile not saved", body: e.message, color: "var(--err)" }),
      );
  return (
    <Block title="Profile">
      <span className="flex items-center gap-3">
        <Select
          label="Profile"
          mono
          value={current}
          onChange={choose}
          options={[
            ...(profiles.data ?? []).map((p) => ({
              value: p.name,
              label: p.name,
              detail: harnessLabel(p.harness),
            })),
            // The one it names, even when that profile is gone.
            ...(profiles.data && !found ? [{ value: current, label: current }] : []),
          ]}
        />
        {profiles.data && (
          <span className={`text-[13px] ${found?.connected ? "text-ok" : "text-err"}`}>
            {!found ? "doesn’t exist" : found.connected ? "connected" : "not connected"}
          </span>
        )}
      </span>
      {found && <StillOnAnother project={project.id} profile={found} />}
      <span className="text-[13.5px] text-tx2">
        Its agents run on this harness and login, from their next start. Profiles are yours on this
        machine, in{" "}
        <Link to="/settings" className="text-accTx hover:underline">
          Settings → Agents
        </Link>
        .
      </span>
    </Block>
  );
}

/** The project's code repos, wherever they are on disk, each with what keeps
 * agents from using it; added, created, located, renamed and removed here.
 * Renaming and removing wait until no team is deployed on the project. */
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

function CodeRepos({ project }: { project: Project }) {
  const client = useClient();
  const queries = useQueryClient();
  const toast = useToast();
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
  return (
    <Block title="Code repos">
      {project.repos.length === 0 ? (
        <p className="m-0 text-[13.5px] text-err">
          None yet: agents need at least one repo to work on.
        </p>
      ) : (
        <ul className="m-0 flex list-none flex-col gap-2 p-0">
          {project.repos.map((r) => (
            <li key={r.name} className="flex flex-col gap-0.5">
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
                <span className="flex items-center gap-2">
                  <span className="min-w-0 flex-1 font-mono text-[13px] break-all">
                    <span className="text-tx">{r.name}</span>{" "}
                    <span className="text-tx3">{r.path}</span>
                  </span>
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
            </li>
          ))}
        </ul>
      )}
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
    </Block>
  );
}

/** Archives the project (out of the sidebar's projects and the Dashboard, not
 * deployable; everything kept), stopping the teams on it after a confirmation;
 * or unarchives it. */
function ArchiveProject({ project }: { project: Project }) {
  const client = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  const teams = useTeams().data ?? [];
  const running = (useDeployments().data ?? []).filter((d) => d.projectId === project.id);
  const names = running.map((d) => teams.find((t) => t.id === d.teamId)?.config.name || d.teamId);
  const [confirming, setConfirming] = useState(false);
  const archive = useMutation({
    mutationFn: async () => {
      if (!project.archived) {
        for (const d of running) await client.agents.stop(d.id);
      }
      return client.home.setArchived(project.id, !project.archived);
    },
    onSettled: () => {
      queries.invalidateQueries({ queryKey: keys.projects });
      queries.invalidateQueries({ queryKey: keys.deployments });
      queries.invalidateQueries({ queryKey: keys.instances });
    },
    onError: (e: Error) =>
      toast({
        title: project.archived ? "Project not unarchived" : "Project not archived",
        body: e.message,
        color: "var(--err)",
      }),
  });
  return (
    <Block title={project.archived ? "Archived" : "Archive this project"}>
      <span className="text-[13.5px] text-tx2">
        {project.archived
          ? "It is out of the sidebar's projects and the Dashboard, and takes no deployment, until you unarchive it."
          : `It leaves the sidebar's projects and the Dashboard, and takes no deployment until you unarchive it. Everything is kept.${running.length > 0 ? ` ${names.join(", ")} ${running.length === 1 ? "stops" : "stop"} first.` : ""}`}
      </span>
      <span>
        <Button
          variant="small"
          disabled={archive.isPending}
          onClick={() =>
            running.length > 0 && !project.archived ? setConfirming(true) : archive.mutate()
          }
        >
          {project.archived ? "Unarchive project" : "Archive project"}
        </Button>
      </span>
      <Confirm
        open={confirming}
        title={`Stop ${names.join(", ")} and archive ${project.id}?`}
        body={`${names.join(", ")} ${running.length === 1 ? "stops" : "stop"} on ${project.id}: ${running.length === 1 ? "its" : "their"} sessions end; worktrees and branches are kept. Then ${project.id} is archived.`}
        action="Stop and archive"
        onConfirm={() => archive.mutate()}
        onClose={() => setConfirming(false)}
      />
    </Block>
  );
}

/** Removes an archived project and its history; its project repo folder only
 * when asked. One not archived is archived first. */
function RemoveProject({ project }: { project: Project }) {
  const client = useClient();
  const queries = useQueryClient();
  const navigate = useNavigate();
  const toast = useToast();
  const [open, setOpen] = useState(false);
  const [deleteRepo, setDeleteRepo] = useState(false);
  const [deleteWorktrees, setDeleteWorktrees] = useState(false);
  const remove = useMutation({
    mutationFn: () => client.home.removeProject(project.id, { deleteRepo, deleteWorktrees }),
    onSuccess: () => {
      queries.invalidateQueries({ queryKey: keys.projects });
      navigate({ to: "/" });
      toast({ title: `${project.id} removed`, body: "", color: "var(--tx4)" });
    },
    onError: (e: Error) =>
      toast({ title: "Project not removed", body: e.message, color: "var(--err)" }),
  });
  return (
    <Block title="Remove this project">
      <span className="text-[13.5px] text-tx2">
        {project.archived
          ? "Its record and its history go. Its workspace stays. Your code repos keep their branches."
          : "Archive it first: archiving keeps everything, removing deletes its history."}
      </span>
      <span>
        <Button
          variant="small"
          disabled={!project.archived || remove.isPending}
          onClick={() => {
            setDeleteRepo(false);
            setDeleteWorktrees(false);
            setOpen(true);
          }}
        >
          Remove project
        </Button>
      </span>
      <Confirm
        open={open}
        title={`Remove ${project.id}?`}
        body="Its record and its history (deployments, messages, requests, merge requests, session logs, its agents' conversations) go for good. Its workspace, your code repos and their branches stay."
        action="Remove"
        onConfirm={() => remove.mutate()}
        onClose={() => setOpen(false)}
      >
        <label className="flex items-start gap-2 text-[13.5px] text-tx2">
          <input
            type="checkbox"
            checked={deleteWorktrees}
            onChange={(e) => setDeleteWorktrees(e.target.checked)}
            className="mt-1"
          />
          <span>
            Also delete its agents' worktrees, their uncommitted work included. Their branches stay
            in your code repos.
          </span>
        </label>
        <label className="flex items-start gap-2 text-[13.5px] text-tx2">
          <input
            type="checkbox"
            checked={deleteRepo}
            onChange={(e) => setDeleteRepo(e.target.checked)}
            className="mt-1"
          />
          <span>
            Also delete its project repo folder, <span className="font-mono">{project.path}</span>:
            its wiki and project.toml. Unless you pushed it, it has no other copy. Kept, it's taken
            over by a new project in the same workspace.
          </span>
        </label>
      </Confirm>
    </Block>
  );
}

function Block({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-2.5">
      <h3 className="m-0 text-[13px] font-medium text-tx4">{title}</h3>
      {children}
    </div>
  );
}

/** The project's running agents still on another profile than its own: they
 * switch at their next start, or now with Restart them. */
function StillOnAnother({
  project,
  profile,
}: {
  project: string;
  profile: { name: string; harness: string };
}) {
  const client = useClient();
  const toast = useToast();
  const stale = onOtherProfile(useInstances().data ?? [], project, profile);
  const restart = useMutation({
    mutationFn: () => Promise.all(stale.map((i) => client.agents.restart(i.id))),
    onError: (e: Error) =>
      toast({ title: "Agents not restarted", body: e.message, color: "var(--err)" }),
  });
  if (stale.length === 0) return null;
  const on = [...new Set(stale.map((i) => i.profile || harnessLabel(i.harness)))].join(", ");
  const switching = stale.some((i) => i.harness !== profile.harness);
  return (
    <span className="flex items-center gap-3 rounded-[8px] border border-line2 px-3 py-2 text-[13px] text-tx2">
      <span className="flex-1">
        {stale.length === 1 ? "1 agent still runs" : `${stale.length} agents still run`} on {on}:{" "}
        {stale.length === 1 ? "it switches" : "they switch"} at the next start.
        {switching &&
          ` On ${harnessLabel(profile.harness)}, ${stale.length === 1 ? "it starts" : "each starts"} a new conversation.`}
      </span>
      <Button variant="small" disabled={restart.isPending} onClick={() => restart.mutate()}>
        {stale.length === 1 ? "Restart it" : "Restart them"}
      </Button>
    </span>
  );
}
