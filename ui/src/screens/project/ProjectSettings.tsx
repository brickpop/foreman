import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { type Project, useClient } from "../../api/client";
import { agentOf, forgeLabel, harnessLabel, profileOf, when } from "../../app/model";
import {
  keys,
  useDeployments,
  useInstances,
  useProfiles,
  useSaveSettings,
  useSettings,
  useTeams,
  useWorkspace,
} from "../../app/queries";
import { rosterOrder } from "../../app/sessions";
import { Avatar } from "../../components/Avatar";
import { useToast } from "../../components/Toasts";
import { Button, Empty, Panel } from "../../components/ui";

// The project's Settings tab (wiki/product/features/project-settings.md): the
// teams on it (runtime state), then its configuration (read-only).

export function ProjectSettings({ project, onDeploy }: { project: Project; onDeploy: () => void }) {
  return (
    <div className="flex flex-col">
      <Teams project={project} onDeploy={onDeploy} />
      <Configuration project={project} />
    </div>
  );
}

function Teams({ project, onDeploy }: { project: Project; onDeploy: () => void }) {
  const navigate = useNavigate();
  const deployments = useDeployments();
  const instances = useInstances();
  const teams = useTeams();
  const mine = (deployments.data ?? []).filter((d) => d.projectId === project.id);
  return (
    <section className="flex flex-col gap-5 px-12 pt-6 pb-16">
      <div className="flex items-start gap-4">
        <div className="flex flex-1 flex-col gap-1">
          <h2 className="m-0 text-base font-semibold">
            Teams on this project{" "}
            <span className="text-[12.5px] font-normal text-tx4">· runtime state, not config</span>
          </h2>
          <p className="m-0 text-[13.5px] text-tx3">
            Talk to a lead and they'll delegate. Open anyone else for a direct instruction.
          </p>
        </div>
        <Button onClick={onDeploy}>Deploy a team</Button>
      </div>
      {mine.length === 0 ? (
        <Empty>
          No team is deployed here yet. Deploy one and each member gets its own worktree of this
          project's repos.
        </Empty>
      ) : (
        mine.map((d) => {
          const team = teams.data?.find((t) => t.id === d.teamId);
          const roster = rosterOrder(d.id, instances.data ?? [], teams.data ?? []);
          return (
            <div key={d.id} className="flex flex-col gap-3">
              <div className="flex items-center gap-4">
                <div className="flex flex-1 flex-col">
                  <span className="text-base font-semibold">{team?.config.name ?? d.teamId}</span>
                  <span className={`text-[13.5px] ${d.goal ? "text-tx3" : "text-tx4"}`}>
                    {d.goal || "No goal set"}
                  </span>
                </div>
                <Button
                  variant="raised"
                  onClick={() => navigate({ to: "/sessions", search: { deployment: d.id } })}
                >
                  Open sessions
                </Button>
              </div>
              <Panel>
                {roster.map((i) => {
                  const agent = agentOf(teams.data ?? [], i.id);
                  const lead = i.agentId === team?.config.lead;
                  const role = agent?.soul.role ?? "";
                  return (
                    <button
                      key={i.id}
                      type="button"
                      onClick={() =>
                        navigate({ to: "/sessions", search: { deployment: d.id, agent: i.id } })
                      }
                      className="grid w-full grid-cols-[auto_150px_1fr_auto] items-center gap-3 border-b border-line px-4 py-3 text-left last:border-b-0 hover:bg-hover"
                    >
                      <Avatar
                        look={agent?.soul.look}
                        size={32}
                        status={i.status}
                        dotBorder="var(--panel)"
                      />
                      <span className="flex flex-col">
                        <span className="font-semibold">{agent?.soul.name ?? i.agentId}</span>
                        <span className="text-[12.5px] text-tx4">
                          {role.replaceAll("_", " ")}
                          {lead && " · talk here"}
                        </span>
                      </span>
                      <span className="flex min-w-0 flex-col">
                        {i.task && (
                          <span className="truncate text-[13.5px] text-tx2">{i.task}</span>
                        )}
                        {i.doing && (
                          <span className="truncate font-mono text-[12px] text-tx4">
                            {i.doing.line} · {when(i.doing.at)}
                          </span>
                        )}
                      </span>
                      <span className="text-[12.5px] text-tx3">Open →</span>
                    </button>
                  );
                })}
              </Panel>
            </div>
          );
        })
      )}
    </section>
  );
}

function Configuration({ project }: { project: Project }) {
  const workspace = useWorkspace();
  const root = workspace.data?.path ?? "~/agents-ws";
  const forge = project.config.forge;
  const repos = project.repos;
  const tree = [
    `${root}/`,
    "├─ .foreman/          workspace repo (git)",
    "│  ├─ teams/          team definitions (souls, playbooks)",
    "│  ├─ cos/            Chief of Staff (its own folder)",
    "│  └─ state.db        deployments, sessions, requests (runtime)",
    `└─ ${project.id}/`,
    "   ├─ project/        project repo (git)",
    "   │  ├─ project.toml repos, checks, gates, forge",
    "   │  └─ wiki/        knowledge base + wiki.toml (boards)",
    ...repos.map((r) => `   ├─ ${r}/`),
    "   └─ .sandboxes/     one git worktree per agent, per repo",
  ].join("\n");
  return (
    <section className="grid grid-cols-[repeat(auto-fit,minmax(340px,1fr))] gap-9 border-t border-line px-12 pt-7 pb-16">
      <div className="flex min-w-0 max-w-[520px] flex-col gap-6">
        <Block title="Workspace">
          <pre className="m-0 overflow-x-auto font-mono text-[13px] leading-[1.7] whitespace-pre text-tx2">
            {tree}
          </pre>
        </Block>
        <Block title="How work flows">
          {[
            [
              "Team definition",
              "Lives once in the workspace: souls, job descriptions and the playbook. You can deploy it to as many projects as you like.",
            ],
            [
              "Deployment",
              "A team on this project, with an optional goal. It’s runtime state kept by Foreman, not config. It gets its own sessions, worktrees and message channel. Agents in two deployments never mix.",
            ],
            [
              "Session prompt",
              "Generated per agent per deployment: soul plus roster, tools, playbook and this project’s gates. Open any agent to read the exact prompt.",
            ],
            [
              "Enforced by Foreman",
              "Routes ask, reply, handoff, contribute and escalate between sessions. Tracks handoffs (Repositories → In flight), wakes requesters when replies land, sends escalations to Home, and only lets the lead request merges to main.",
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
        <ProfileChoice project={project.id} />
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
                "The lead calls request_merge() with the card that explains the work. Foreman runs the checks, then you choose: Merge to main, Create PR on your forge, or Reject.",
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
              ? "No forge. You merge requested branches into main from the Overview, under Needs you; Foreman does the merge."
              : `Merge to main from the Overview, or create a PR on ${forge.url} and merge it there; Foreman notices. Nothing is pushed until you choose. The token lives in your workspace Settings, never in this file.`}
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
            {project.id}/project/project.toml · read-only
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
        </Block>
      </div>
    </section>
  );
}

/** The profile this project's agents run on, a harness and its login. Kept in
 * the workspace's settings, never project.toml, which the lead can write. */
function ProfileChoice({ project }: { project: string }) {
  const settings = useSettings().data?.settings;
  const profiles = useProfiles();
  const save = useSaveSettings();
  const toast = useToast();
  const current = profileOf(settings, project);
  const found = profiles.data?.find((p) => p.name === current);
  const choose = (name: string) =>
    save((s) => {
      // The default is no entry at all: the file names only what differs.
      const { [project]: _, ...others } = s.projectProfiles ?? {};
      return {
        ...s,
        projectProfiles: name === "default" ? others : { ...others, [project]: name },
      };
    }).catch((e: Error) =>
      toast({ title: "Profile not saved", body: e.message, color: "var(--err)" }),
    );
  return (
    <Block title="Profile">
      <span className="flex items-center gap-3">
        <select
          aria-label="Profile"
          value={current}
          onChange={(e) => choose(e.target.value)}
          className="rounded-md border border-line2 bg-panel px-2 py-1 font-mono text-[13px]"
        >
          {(profiles.data ?? []).map((p) => (
            <option key={p.name} value={p.name}>
              {p.name} · {harnessLabel(p.harness)}
            </option>
          ))}
          {profiles.data && !found && <option value={current}>{current}</option>}
        </select>
        {profiles.data && (
          <span className={`text-[13px] ${found?.connected ? "text-ok" : "text-err"}`}>
            {!found ? "doesn’t exist" : found.connected ? "connected" : "not connected"}
          </span>
        )}
      </span>
      <span className="text-[13.5px] text-tx2">
        Its agents run on this harness and login, from their next start. Profiles are yours on this
        machine, in{" "}
        <Link to="/settings" search={{ tab: "global" }} className="text-accTx hover:underline">
          Settings → Global
        </Link>
        .
      </span>
    </Block>
  );
}

/** The project's code repos, and a new empty one (Foreman never clones). */
function CodeRepos({ project }: { project: Project }) {
  const client = useClient();
  const queries = useQueryClient();
  const [name, setName] = useState("");
  const add = useMutation({
    mutationFn: () => client.workspace.addRepo(project.id, name.trim()),
    onSuccess: () => {
      setName("");
      queries.invalidateQueries({ queryKey: keys.projects });
    },
  });
  const listed = project.config.repos.length > 0;
  return (
    <Block title="Code repos">
      {project.repos.length === 0 ? (
        <p className="m-0 text-[13.5px] text-err">
          None yet: agents need at least one repo to work on. Clone one into {project.id}/, or
          create an empty one here.
        </p>
      ) : (
        <pre className="m-0 overflow-x-auto font-mono text-[13px] leading-[1.7] whitespace-pre text-tx2">
          {repoTree(project.id, project.repos)}
        </pre>
      )}
      {listed ? (
        <p className="m-0 text-[13px] text-tx3">
          project.toml lists its repos: add a new one there, after creating it in {project.id}/.
        </p>
      ) : (
        <form
          className="flex items-center gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            add.mutate();
          }}
        >
          <input
            aria-label="New repo name"
            placeholder="new-repo"
            value={name}
            onChange={(e) => setName(e.target.value)}
            spellCheck={false}
            className="w-48 rounded-[7px] border border-line2 bg-transparent px-2.5 py-1.5 font-mono text-[13px] text-tx placeholder:text-tx4"
          />
          <Button type="submit" variant="small" disabled={!name.trim() || add.isPending}>
            New repo
          </Button>
        </form>
      )}
      {add.error && <p className="m-0 text-[13px] text-err">{add.error.message}</p>}
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

/** The project's code repos as a folder tree, like the Workspace tree. */
export function repoTree(project: string, repos: string[]): string {
  return [
    `${project}/`,
    ...repos.map((r, i) => `${i === repos.length - 1 ? "└─" : "├─"} ${r}/`),
  ].join("\n");
}
