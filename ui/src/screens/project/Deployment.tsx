import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import type { Instance, Team } from "../../api/client";
import { agentOf, roleLabel, when } from "../../app/model";
import { lastTeam, sessionOf } from "../../app/project";
import {
  useDeployments,
  useInstances,
  useRepos,
  useStoppedDeployments,
  useTeams,
} from "../../app/queries";
import { type Worktree, worktrees } from "../../app/repos";
import { rosterOrder } from "../../app/sessions";
import { Avatar } from "../../components/Avatar";
import { DeployDialog } from "../../components/DeployDialog";
import { StopIcon } from "../../components/icons";
import { StopDialog } from "../../components/StopDialog";
import { Button, Empty, LeadTag, Loading, Panel } from "../../components/ui";
import { useProject } from "../Project";
import { InFlight } from "./Pipeline";

// A project's Deployment section (wiki/product/features/project-deployment.md):
// the team on it, its agents with what they do and the branch
// each has checked out (and the worktrees earlier teams left); then the work
// on its way to main, and what merged. Deploy (replacing the team) and Stop.
export function Deployment() {
  const project = useProject();
  const repos = useRepos(project.id);
  const teams = useTeams();
  const instances = useInstances();
  const d = useDeployments().data?.find((x) => x.projectId === project.id);
  const stopped = useStoppedDeployments().data ?? [];
  const [deploy, setDeploy] = useState(false);
  const [stopping, setStopping] = useState(false);
  const team = teams.data?.find((t) => t.id === d?.teamId);
  const teamName = team?.config.name || d?.teamId || "";
  const roster = d ? rosterOrder(d.id, instances.data ?? [], teams.data ?? []) : [];
  const trees = worktrees(repos.data ?? []);
  // The worktrees no agent of the team holds: an earlier team's, kept by its Stop.
  const left = trees.filter((w) => !roster.some((i) => i.agentId === w.agent));
  return (
    <div className="flex flex-col gap-8 px-6 pt-6 pb-12">
      <section className="flex flex-col gap-2.5">
        <div className="flex items-start gap-4">
          {d ? (
            <span className="flex min-w-0 flex-1 flex-col gap-0.5">
              <Link
                to="/teams/$team"
                params={{ team: d.teamId }}
                search={{ tab: "deployments" }}
                className="self-start text-base font-semibold hover:underline"
              >
                {teamName}
              </Link>
              <span className="text-[13.5px] text-tx4">deployed {when(d.createdAt)}</span>
            </span>
          ) : (
            <span className="flex-1" />
          )}
          <Button onClick={() => setDeploy(true)}>
            {d ? "Deploy another team" : "Deploy a team"}
          </Button>
          {d && (
            <Button
              variant="danger"
              className="inline-flex items-center gap-1.5"
              onClick={() => setStopping(true)}
            >
              <StopIcon />
              Stop
            </Button>
          )}
        </div>
        {repos.isError && <p className="m-0 text-[13px] text-err">{repos.error.message}</p>}
        {!d && repos.isPending && <Loading />}
        {!d && left.length === 0 && repos.isSuccess ? (
          <Empty>
            No team is on {project.id}. Deploy one and each member gets its own worktree of the
            project's repos.
          </Empty>
        ) : (
          (roster.length > 0 || left.length > 0) && (
            <Panel>
              {roster.map((i) => (
                <AgentRow
                  key={i.id}
                  agent={i.agentId}
                  instance={i}
                  worktree={trees.find((w) => w.agent === i.agentId)}
                  project={project.id}
                  teams={teams.data ?? []}
                />
              ))}
              {left.map((w) => (
                <AgentRow
                  key={w.agent}
                  agent={w.agent}
                  worktree={w}
                  project={project.id}
                  teams={teams.data ?? []}
                />
              ))}
            </Panel>
          )
        )}
      </section>
      <InFlight project={project} />
      {/* Mounted only while open: each opening starts fresh. */}
      {deploy && (
        <DeployDialog
          open
          onClose={() => setDeploy(false)}
          project={project.id}
          initialTeam={d ? undefined : lastTeam(project.id, stopped)}
        />
      )}
      {stopping && d && (
        <StopDialog deployment={d} teamName={teamName} onClose={() => setStopping(false)} />
      )}
    </div>
  );
}

/** An agent on the project: the team's (its instance), with what it is doing,
 * or an earlier team's, by the worktree it left. Its branches link to their
 * code in Repos; a team member's row opens its session. */
function AgentRow({
  agent: id,
  instance,
  worktree: w,
  project,
  teams,
}: {
  agent: string;
  instance?: Instance;
  worktree?: Worktree;
  project: string;
  teams: Team[];
}) {
  const navigate = useNavigate();
  const agent = agentOf(teams, `${id}@${project}`);
  return (
    <div className="grid grid-cols-[auto_minmax(140px,220px)_1fr_auto] items-start gap-3 border-b border-line px-4 py-3 last:border-b-0">
      <Avatar
        look={agent?.soul.look}
        size={26}
        status={instance?.status}
        dotBorder="var(--panel)"
      />
      <span className="flex min-w-0 flex-col">
        <span className="flex items-center gap-1.5">
          <span className="truncate font-medium">{agent?.soul.name ?? id}</span>
          {agent && agent.team.config.lead === agent.id && <LeadTag />}
        </span>
        <span className="truncate text-[12.5px] text-tx4">
          {agent ? `${agent.team.config.name} · ${roleLabel(agent.soul.role)}` : id}
        </span>
      </span>
      <span className="flex min-w-0 flex-col gap-1">
        {instance?.doing && (
          <span className="truncate font-mono text-[12px] text-tx3">
            {instance.doing.line} · {when(instance.doing.at)}
          </span>
        )}
        {w?.branches.map(({ repo, branch: b }) => (
          <Link
            key={repo}
            to="/projects/$project/repos"
            params={{ project }}
            search={{ repo, branch: b.name }}
            title={`Read ${b.name} in ${repo}`}
            className="flex min-w-0 items-baseline gap-2 rounded-md text-[13px] hover:underline"
          >
            <span className="shrink-0 font-mono text-[12px] text-tx4">{repo}</span>
            <span className="truncate font-mono text-[12.5px] text-accTx">{b.name}</span>
            <span className="shrink-0 text-[12px] text-tx4">
              {b.ahead > 0 && `${b.ahead} ahead`}
              {b.uncommitted && (
                <span className="text-accTx">{b.ahead > 0 ? " · " : ""}uncommitted changes</span>
              )}
            </span>
          </Link>
        ))}
        {!w && (
          <span className="text-[12.5px] text-tx4">No worktree yet: made at its first start.</span>
        )}
      </span>
      {instance ? (
        <Button variant="small" onClick={() => navigate(sessionOf(instance.id))}>
          Session
        </Button>
      ) : (
        <span className="text-xs text-tx4">not on the team</span>
      )}
    </div>
  );
}
