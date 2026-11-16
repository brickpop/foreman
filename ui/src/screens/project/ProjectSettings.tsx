import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { type Project, useClient } from "../../api/client";
import { harnessLabel, nameTakenBy, onOtherProfile, projectName, readable } from "../../app/model";
import {
  keys,
  useDeployments,
  useHomeFolder,
  useInstances,
  useProfiles,
  useProjects,
  useTeams,
} from "../../app/queries";
import { Confirm } from "../../components/Overlays";
import { Select } from "../../components/Select";
import { useToast } from "../../components/Toasts";
import { Button, GroupCard } from "../../components/ui";
import { useProject } from "../Project";

// A project's Settings section (wiki/product/features/project-settings.md):
// its configuration. The team on it is its Team section.

export function ProjectSettings() {
  return <Configuration project={useProject()} />;
}

function Configuration({ project }: { project: Project }) {
  const where = useHomeFolder().data;
  const home = where?.path ?? "~/.foreman";
  const folder = where?.folder ?? "~/foreman";
  const tree = [
    `${folder}/`,
    "├─ hq/                    Foreman's HQ (git, a wiki): teams, your instructions",
    `├─ projects/${project.id}/`,
    "│  ├─ hq/                 this project's HQ (git), its wiki at the root:",
    "│  │  ├─ project.toml     checks, gates",
    "│  │  └─ wiki.toml, …     knowledge base, boards, AGENTS.md",
    "│  └─ files/              inbox/ from you, shared/ by its agents",
    `└─ worktrees/${project.id}/  one sandbox per agent: a worktree of each repo`,
    `${home}/`,
    "├─ settings.toml          this machine's settings",
    `├─ harness/${project.id}/  each agent's harness config and conversations`,
    "└─ state.db               deployments, sessions, requests (runtime)",
  ].join("\n");
  return (
    <section className="flex max-w-[760px] flex-col gap-6 px-6 pt-6 pb-12">
      <GroupCard title="General">
        <ProjectNameField project={project} />
        <Folders project={project} folder={folder} />
        <ProfileChoice project={project} />
        <HideProject project={project} />
      </GroupCard>
      <GroupCard title="Rules" aside="project.toml">
        <Block title="What comes to you">
          <div className="grid grid-cols-[84px_1fr] gap-x-3 gap-y-2">
            {[
              [
                "approval",
                "text-attnTx",
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
          <span className="text-[13.5px] text-tx2">
            You merge requested branches into their target from their review, in the Inbox; Foreman
            does the merge. Nothing is pushed: you push with your own git.
          </span>
        </Block>
        <Block title="The file">
          <span className="font-mono text-[12.5px] text-tx3">
            {project.path}/project.toml · read-only
          </span>
          <p className="m-0 text-[13.5px] text-tx2">
            This file is config, committed in the HQ. Edit it in your editor.
          </p>
          {project.problems.length > 0 && (
            <p className="m-0 rounded-lg border border-err px-3 py-2 text-[13px] text-err">
              {project.problems.join(" · ")}
            </p>
          )}
          <pre className="m-0 overflow-x-auto rounded-[10px] border border-line bg-term p-3.5 font-mono text-[12.5px] leading-[1.6] text-tx2 select-text">
            {project.source}
          </pre>
        </Block>
      </GroupCard>
      <GroupCard title="How it works" collapsible>
        <Block title="Where things live">
          <pre className="m-0 overflow-x-auto font-mono text-[13px] leading-[1.7] whitespace-pre text-tx2">
            {tree}
          </pre>
        </Block>
        <Block title="How work flows">
          {[
            [
              "Team definition",
              "Lives once, in Foreman's HQ: souls, job descriptions and the playbook. You can deploy it to as many projects as you like.",
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
              "Routes ask, reply, handoff, contribute and escalate between sessions. Tracks handoffs (Team → In flight), wakes requesters when replies land, sends escalations to the Dashboard, and only lets the lead request merges.",
            ],
          ].map(([title, body]) => (
            <div key={title} className="flex flex-col gap-0.5">
              <span className="font-semibold">{title}</span>
              <span className="leading-[1.55] text-tx2">{body}</span>
            </div>
          ))}
        </Block>
      </GroupCard>
      <GroupCard title="Danger zone" danger>
        <RemoveProject project={project} />
      </GroupCard>
    </section>
  );
}

/** Where the project's HQ and its agents' worktrees are, to copy. */
function Folders({ project, folder }: { project: Project; folder: string }) {
  const worktrees = `${folder}/worktrees/${project.id}`;
  return (
    <Block title="Folders">
      {[
        { label: "HQ", path: project.path },
        { label: "Worktrees", path: worktrees },
      ].map((f) => (
        <span key={f.label} className="flex items-center gap-3">
          <span className="w-[76px] text-[13px] text-tx4">{f.label}</span>
          <span className="min-w-0 flex-1 truncate font-mono text-[13px] text-tx2 select-text">
            {f.path}
          </span>
          <Button variant="small" onClick={() => navigator.clipboard.writeText(f.path)}>
            Copy
          </Button>
        </span>
      ))}
      <span className="text-[13.5px] text-tx3">
        Its HQ holds its wiki and project.toml. In its worktrees, one folder per agent: look at and
        run their code there. Editing while an agent works turns into its uncommitted changes.
      </span>
    </Block>
  );
}

/** The profile this project's agents run on, a harness and its login. Kept in
 * Foreman's records, never project.toml, which the lead can write. */
/** The project's display name: any words, how it shows everywhere; empty, it
 * shows by its id. Its id stays (folders, branches, URLs are built from it). */
function ProjectNameField({ project }: { project: Project }) {
  const client = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  const projects = useProjects().data ?? [];
  const [draft, setDraft] = useState<string>();
  const value = draft ?? project.name;
  const taken = nameTakenBy(projects, project.id, value);
  const save = useMutation({
    mutationFn: () => client.home.setName(project.id, value.trim()),
    onSuccess: () => {
      setDraft(undefined);
      queries.invalidateQueries({ queryKey: keys.projects });
    },
    onError: (e: Error) => toast({ title: "Name not saved", body: e.message, color: "var(--err)" }),
  });
  const changed = value.trim() !== project.name;
  return (
    <Block title="Name">
      <form
        className="flex items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (changed && !taken && !save.isPending) save.mutate();
        }}
      >
        <input
          value={value}
          onChange={(e) => setDraft(e.target.value)}
          placeholder={readable(project.id)}
          aria-label="Project name"
          maxLength={80}
          className="min-w-0 flex-1 rounded-lg border border-line2 bg-term px-3 py-1.5 text-[13.5px] text-tx"
        />
        {changed && (
          <>
            <Button type="button" variant="small" onClick={() => setDraft(undefined)}>
              Cancel
            </Button>
            <Button type="submit" variant="small" disabled={save.isPending || !!taken}>
              Save
            </Button>
          </>
        )}
      </form>
      {changed && taken && (
        <span className="text-[13px] text-err">
          Another project shows as {taken} already: pick another name.
        </span>
      )}
      <span className="text-[13.5px] text-tx2">
        How it shows everywhere; empty, it shows as {readable(project.id)}. Its id,{" "}
        <span className="font-mono text-[12.5px]">{project.id}</span>, stays: its folder, its
        agents' branches and its links are built from it.
      </span>
    </Block>
  );
}

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
        <Link to="/settings/$tab" params={{ tab: "agents" }} className="text-accTx hover:underline">
          Settings → Agents
        </Link>
        .
      </span>
    </Block>
  );
}

/** Hides the project (out of the sidebar's projects, into More, and off the Dashboard), or
 * shows it again: nothing else changes, a team on it keeps working. */
function HideProject({ project }: { project: Project }) {
  const client = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  const hide = useMutation({
    mutationFn: () => client.home.setHidden(project.id, !project.hidden),
    onSettled: () => queries.invalidateQueries({ queryKey: keys.projects }),
    onError: (e: Error) =>
      toast({
        title: project.hidden ? "Project not shown" : "Project not hidden",
        body: e.message,
        color: "var(--err)",
      }),
  });
  return (
    <Block title="Visibility">
      <span className="text-[13.5px] text-tx2">
        {project.hidden
          ? "Hidden: it is under More in the sidebar, and off the Dashboard. Everything else works as usual."
          : "Hide it to keep the sidebar and the Dashboard to what you work on: it moves under More. Everything else works as usual, a team on it included."}
      </span>
      <span>
        <Button variant="small" disabled={hide.isPending} onClick={() => hide.mutate()}>
          {project.hidden ? "Show project" : "Hide project"}
        </Button>
      </span>
    </Block>
  );
}

/** Removes the project and its history; its HQ and its agents' worktrees
 * only when asked. Never while a team is deployed on it: stopped first. */
function RemoveProject({ project }: { project: Project }) {
  const client = useClient();
  const teams = useTeams().data ?? [];
  const running = (useDeployments().data ?? []).filter((d) => d.projectId === project.id);
  const names = running.map((d) => teams.find((t) => t.id === d.teamId)?.config.name || d.teamId);
  const queries = useQueryClient();
  const navigate = useNavigate();
  const toast = useToast();
  const [open, setOpen] = useState(false);
  const [deleteHQ, setDeleteHQ] = useState(false);
  const [deleteWorktrees, setDeleteWorktrees] = useState(false);
  const remove = useMutation({
    mutationFn: () => client.home.removeProject(project.id, { deleteHQ, deleteWorktrees }),
    onSuccess: () => {
      queries.invalidateQueries({ queryKey: keys.projects });
      navigate({ to: "/" });
      toast({
        title: `${projectName([project], project.id)} removed`,
        body: "",
        color: "var(--tx4)",
      });
    },
    onError: (e: Error) =>
      toast({ title: "Project not removed", body: e.message, color: "var(--err)" }),
  });
  return (
    <Block title="Remove this project">
      <span className="text-[13.5px] text-tx2">
        {running.length > 0
          ? `Stop ${names.join(", ")} on it first (its Team section).`
          : "Its record and its history go. Its HQ and files stay. Your code repos keep their branches."}
      </span>
      <span>
        <Button
          variant="small"
          disabled={running.length > 0 || remove.isPending}
          onClick={() => {
            setDeleteHQ(false);
            setDeleteWorktrees(false);
            setOpen(true);
          }}
        >
          Remove project
        </Button>
      </span>
      <Confirm
        open={open}
        title={`Remove ${projectName([project], project.id)}?`}
        body="Its record and its history (deployments, messages, requests, merge requests, session logs, its agents' conversations) go for good. Its files, your code repos and their branches stay."
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
            checked={deleteHQ}
            onChange={(e) => setDeleteHQ(e.target.checked)}
            className="mt-1"
          />
          <span>
            Also delete its HQ, <span className="font-mono">{project.path}</span>: its wiki and
            project.toml. Unless you pushed it, it has no other copy. Kept, a new project of the
            same name takes it over.
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
