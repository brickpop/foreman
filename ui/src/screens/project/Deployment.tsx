import { useMutation } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { type Instance, type Team, useClient } from "../../api/client";
import { agentOf, readable, reloadLine, reloadOf, roleLabel, when } from "../../app/model";
import { latestStopped, sessionOf } from "../../app/project";
import {
  useDeployments,
  useInstances,
  useRepos,
  useStoppedDeployments,
  useTeams,
} from "../../app/queries";
import { type Worktree, worktrees } from "../../app/repos";
import { rosterOrder } from "../../app/sessions";
import { DeployDialog } from "../../components/DeployDialog";
import { ReloadIcon, StopIcon } from "../../components/icons";
import { StopDialog } from "../../components/StopDialog";
import { useToast } from "../../components/Toasts";
import { Button, Dot, Empty, LeadTag, Loading, Panel } from "../../components/ui";
import { useProject } from "../Project";
import { InFlight } from "./Pipeline";

// A project's Deployment section (wiki/product/features/project-deployment.md):
// the team on it, its agents with what they do and the branch
// each has checked out (and the worktrees earlier teams left); then the work
// on its way to its targets, and what merged. Deploy (replacing the team), Reload
// (its agents left on an older prompt) and Stop.
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
  // Named as the team that stopped last there has them: the one that left them, usually.
  const earlier = latestStopped(project.id, stopped);
  const client = useClient();
  const toast = useToast();
  const reload = reloadOf(roster);
  // Each restart continues its agent's conversation, on the prompt as it is now.
  const restart = useMutation({
    mutationFn: () => Promise.all(reload.behind.map((i) => client.agents.restart(i.id))),
    onError: (e: Error) =>
      toast({ title: "Agents not reloaded", body: e.message, color: "var(--err)" }),
  });
  return (
    <div className="flex flex-col gap-8 px-6 pt-6 pb-12">
      <section className="flex flex-col gap-2.5">
        <div className="flex items-start gap-4">
          {d ? (
            <span className="flex min-w-0 flex-1 flex-col gap-0.5">
              <Link
                to="/teams/$team"
                params={{ team: d.teamId }}
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
          {reload.behind.length > 0 && (
            <Button
              className="inline-flex items-center gap-1.5"
              disabled={!!reload.busy || restart.isPending}
              onClick={() => restart.mutate()}
            >
              <ReloadIcon />
              Reload
            </Button>
          )}
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
        {reload.behind.length > 0 && (
          <p className="m-0 text-[13px] text-tx3">{reloadLine(reload, teams.data ?? [])}</p>
        )}
        {repos.isError && <p className="m-0 text-[13px] text-err">{repos.error.message}</p>}
        {!d && repos.isPending && <Loading />}
        {!d && left.length === 0 && repos.isSuccess ? (
          <Empty>
            No team is on {readable(project.id)}. Deploy one and each member gets its own worktree
            of the project's repos.
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
                  deployment={i.deploymentId}
                />
              ))}
              {left.map((w) => (
                <AgentRow
                  key={w.agent}
                  agent={w.agent}
                  worktree={w}
                  project={project.id}
                  teams={teams.data ?? []}
                  deployment={earlier?.id ?? ""}
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
          initialTeam={d ? undefined : earlier?.teamId}
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
  deployment,
}: {
  agent: string;
  instance?: Instance;
  worktree?: Worktree;
  project: string;
  teams: Team[];
  /** Whose team the agent is in (team@project). */
  deployment: string;
}) {
  const navigate = useNavigate();
  const agent = agentOf(teams, deployment, id);
  return (
    <div className="grid grid-cols-[auto_minmax(140px,220px)_1fr_auto] items-start gap-3 border-b border-line px-4 py-3 last:border-b-0">
      {/* An earlier team's worktree has no instance, so no status. */}
      <span className="mt-[6px] size-2">
        {instance && <Dot status={instance.status} size={8} title={instance.status} />}
      </span>
      <span className="flex min-w-0 flex-col">
        <span className="flex items-center gap-1.5">
          <span className="truncate font-medium">{agent?.soul.name ?? id}</span>
          {agent && agent.team.config.lead === agent.id && <LeadTag name={agent.soul.name} />}
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
            to="/projects/$project/repos/$repo"
            params={{ project, repo }}
            search={{ branch: b.name }}
            title={`Read ${b.name} in ${repo}`}
            className="flex min-w-0 items-baseline gap-2 rounded-md text-[13px] hover:underline"
          >
            <span className="shrink-0 font-mono text-[12px] text-tx4">{repo}</span>
            <span className="truncate font-mono text-[12.5px] text-accTx">{b.name}</span>
            <span className="shrink-0 text-[12px] text-tx4">
              {b.ahead > 0 && `${b.ahead} ahead`}
              {b.uncommitted && (
                <span className="text-attnTx">{b.ahead > 0 ? " · " : ""}uncommitted changes</span>
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
