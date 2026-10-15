import { useNavigate } from "@tanstack/react-router";
import type { Project } from "../../api/client";
import { useOpenChiefOfStaff } from "../../app/chiefOfStaff";
import { agentOf } from "../../app/model";
import { useDeployments, useInstances, useTeams, useWorkspace } from "../../app/queries";
import { rosterOrder } from "../../app/sessions";
import { Avatar } from "../../components/Avatar";
import { Button, Empty, Panel } from "../../components/ui";

// The project's Settings tab (wiki/product/features/project-settings.md): the
// teams on it (runtime state), then its configuration (read-only).

const forgeNames: Record<string, string> = {
  github: "GitHub",
  forgejo: "Forgejo",
  gitea: "Gitea",
  none: "No forge",
};

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
    <section className="flex flex-col gap-5 px-10 pt-6 pb-16">
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
                  <span className="text-[13.5px] text-tx3">{d.goal}</span>
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
                      <span className="truncate text-[13.5px] text-tx2">{i.task}</span>
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
  const openChiefOfStaff = useOpenChiefOfStaff();
  const root = workspace.data?.path ?? "~/agents-ws";
  const forge = project.config.forge;
  const repos = project.repos;
  const tree = [
    `${root}/`,
    "├─ .foreman/          workspace repo (git)",
    "│  ├─ teams/          team definitions (souls, playbooks)",
    "│  ├─ cos/            Chief of Staff session",
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
      <div className="flex max-w-[520px] flex-col gap-6">
        <Block title="Workspace">
          <pre className="m-0 font-mono text-[13px] leading-[1.7] whitespace-pre text-tx2">
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
              "A team on this project with a goal. It’s runtime state kept by Foreman, not config. It gets its own sessions, worktrees and message channel. Agents in two deployments never mix.",
            ],
            [
              "Session prompt",
              "Generated per agent per deployment: soul plus roster, tools, playbook and this project’s gates. Open any agent to read the exact prompt.",
            ],
            [
              "Enforced by Foreman",
              "Routes ask, reply, handoff, contribute and escalate between sessions. Tracks handoffs (that’s Changes), wakes requesters when replies land, sends escalations to Home, and only lets the lead request merges to main.",
            ],
          ].map(([title, body]) => (
            <div key={title} className="flex flex-col gap-0.5">
              <span className="font-semibold">{title}</span>
              <span className="leading-[1.55] text-tx2">{body}</span>
            </div>
          ))}
        </Block>
      </div>
      <div className="flex flex-col gap-6">
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
          <span className="font-medium">{forgeNames[forge.kind] ?? forge.kind}</span>
          <span className="text-[13.5px] text-tx2">
            {forge.kind === "none"
              ? "No forge. You merge requested branches into main from Changes; Foreman does the merge."
              : `Merge to main from Changes, or create a PR on ${forge.url} and merge it there; Foreman notices. Nothing is pushed until you choose. The token lives in your workspace Settings, never in this file.`}
          </span>
          {forge.projectRepo && (
            <span className="flex items-center gap-3 text-[13px] text-tx3">
              <span className="font-mono">project repo → {forge.projectRepo}</span>
              <Button variant="tiny" disabled title="Comes with the Git epic">
                Sync
              </Button>
            </span>
          )}
        </Block>
        <Block title="project.toml">
          <span className="flex items-center gap-3">
            <span className="font-mono text-[12.5px] text-tx3">
              {project.id}/project/project.toml · read-only
            </span>
            <Button
              variant="tiny"
              onClick={() => openChiefOfStaff({ flow: "project-config", project: project.id })}
            >
              Change with Chief of Staff
            </Button>
          </span>
          <p className="m-0 text-[13.5px] text-tx2">
            This file is config, committed in the project repo. Edit it in your editor, or have the
            Chief of Staff edit it. Its change then comes back to you in Changes as a review.
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

function Block({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-2.5">
      <h3 className="m-0 text-[13px] font-medium text-tx4">{title}</h3>
      {children}
    </div>
  );
}
