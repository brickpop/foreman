import { Link, useNavigate, useSearch } from "@tanstack/react-router";
import { useState } from "react";
import type { Cleaned, Instance, Team } from "../../api/client";
import { agentOf, projectName, reloadLine, reloadOf, roleLabel, when } from "../../app/model";
import { latestStopped, sessionOf, teamView, teamViews } from "../../app/project";
import {
  useCleanUp,
  useDeployments,
  useInstances,
  useRepos,
  useStoppedDeployments,
  useTeams,
} from "../../app/queries";
import { uncommittedWork, type Worktree, worktrees } from "../../app/repos";
import { rosterOrder } from "../../app/sessions";
import { DeployDialog } from "../../components/DeployDialog";
import { StopIcon } from "../../components/icons";
import { Confirm } from "../../components/Overlays";
import { ReloadButton } from "../../components/ReloadButton";
import { StopDialog } from "../../components/StopDialog";
import { useToast } from "../../components/Toasts";
import {
  Button,
  Dot,
  Empty,
  LeadTag,
  Loading,
  Panel,
  SectionTitle,
  Segmented,
} from "../../components/ui";
import { useProject } from "../Project";
import { Activity } from "./Activity";
import { InFlight, Merged } from "./Pipeline";

// A project's Team section (wiki/product/features/project-team.md):
// the team on it, its agents with what they do and the branch
// each has checked out, or the repos it has open on none (and the worktrees earlier teams left); then the work
// on its way to its targets, and what merged; the team's activity between. Deploy (replacing the team), Reload
// (its agents left on an older prompt) and Stop.
export function ProjectTeam() {
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
  const search = useSearch({ from: "/projects/$project/team" });
  const navigate = useNavigate({ from: "/projects/$project/team" });
  const view = teamView(search.view, roster.length + left.length > 0);
  const reload = reloadOf(roster);
  return (
    <div className="flex flex-col gap-8 px-6 pt-6 pb-12">
      <section className="flex flex-col gap-2.5">
        <div className="flex items-start gap-4">
          {d ? (
            <span className="flex min-w-0 flex-1 flex-col gap-0.5">
              <Link
                to="/settings/teams/$team"
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
          {d && <ReloadButton instances={roster} />}
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
      </section>
      <Segmented
        label="Show"
        value={view}
        onChange={(v) => navigate({ search: { view: v } })}
        options={teamViews.map((v) => ({ value: v.view, label: v.label }))}
      />
      {view === "agents" && (
        <>
          {repos.isError && <p className="m-0 text-[13px] text-err">{repos.error.message}</p>}
          {repos.isPending && <Loading />}
          {roster.length > 0 && (
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
            </Panel>
          )}
          {!d && repos.isSuccess && (
            <Empty>
              No team is on {projectName([project], project.id)}. Deploy one and each member gets
              its own sandbox, where it opens the project's repos as it needs them.
            </Empty>
          )}
          {left.length > 0 && (
            <EarlierTeams
              project={project.id}
              left={left}
              teams={teams.data ?? []}
              deployment={earlier?.id ?? ""}
            />
          )}
        </>
      )}
      {view === "activity" && <Activity />}
      {view === "inflight" && <InFlight project={project} />}
      {view === "merged" && (
        <div className="flex flex-col gap-3">
          <Merged project={project} />
        </div>
      )}
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
        {w?.idle.map(({ repo, uncommitted }) => (
          <span key={repo} className="flex min-w-0 items-baseline gap-2 text-[13px]">
            <span className="shrink-0 font-mono text-[12px] text-tx4">{repo}</span>
            <span className="truncate text-[12.5px] text-tx3">idle, on no branch</span>
            {uncommitted && (
              <span className="shrink-0 text-[12px] text-attnTx">uncommitted changes</span>
            )}
          </span>
        ))}
        {!w && (
          <span className="text-[12.5px] text-tx4">
            No repo open yet: it opens one when it needs it.
          </span>
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

/** The worktrees earlier teams' agents left, kept by their Stop so a redeploy
 * resumes them. Clean up removes only what is safe (a worktree with nothing
 * uncommitted, a wt/<project>/ branch a target holds) and lists what it kept, with why;
 * worktrees with uncommitted work too when asked, the box off at each opening
 * (the CEO, 2026-11-05), then listing the files they held. */
function EarlierTeams({
  project,
  left,
  teams,
  deployment,
}: {
  project: string;
  left: Worktree[];
  teams: Team[];
  deployment: string;
}) {
  const clean = useCleanUp(project);
  const toast = useToast();
  const [asking, setAsking] = useState(false);
  const [uncommitted, setUncommitted] = useState(false);
  const [done, setDone] = useState<{ kept: Cleaned[]; discarded: Cleaned[] }>({
    kept: [],
    discarded: [],
  });
  const unsaved = uncommittedWork(left);
  const run = () =>
    clean.mutate(uncommitted, {
      onSuccess: (done) => {
        setDone({ kept: done.kept, discarded: done.removed.filter((c) => c.backup) });
        toast({
          title:
            done.removed.length > 0
              ? `Cleaned up: ${done.removed.length} removed`
              : "Nothing was safe to remove",
          body:
            done.kept.length > 0
              ? `${done.kept.length} kept, each with why, under the agents.`
              : "Nothing was kept.",
          color: "var(--ok)",
        });
      },
      onError: (e) => toast({ title: "Not cleaned up", body: e.message, color: "var(--err)" }),
    });
  return (
    <section className="flex flex-col gap-3">
      <div className="flex items-center gap-3">
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <SectionTitle count={left.length}>Left by earlier teams</SectionTitle>
          <span className="text-[13px] text-tx3">
            Their worktrees stay after a Stop, so a redeploy picks them up.
          </span>
        </span>
        <Button
          disabled={clean.isPending}
          onClick={() => {
            setUncommitted(false);
            setAsking(true);
          }}
        >
          Clean up
        </Button>
      </div>
      <Panel>
        {left.map((w) => (
          <AgentRow
            key={w.agent}
            agent={w.agent}
            worktree={w}
            project={project}
            teams={teams}
            deployment={deployment}
          />
        ))}
      </Panel>
      {done.discarded.length > 0 && (
        <div className="flex flex-col gap-1 text-[13px] text-tx3">
          <span>Removed with their uncommitted work by the last clean up:</span>
          {done.discarded.map((c) => (
            <span key={`${c.agent} ${c.repo}`}>
              <span className="font-mono text-[12.5px] text-tx2">
                {c.agent}'s {c.repo}: {c.files?.join(", ")}
              </span>
              . Kept a while under{" "}
              <span className="font-mono text-[12.5px] text-tx2 select-text">{c.backup}</span>.
            </span>
          ))}
        </div>
      )}
      {done.kept.length > 0 && (
        <div className="flex flex-col gap-1 text-[13px] text-tx3">
          <span>Kept by the last clean up:</span>
          {done.kept.map((c) => (
            <span key={`${c.agent} ${c.repo} ${c.what} ${c.branch}`}>
              <span className="font-mono text-[12.5px] text-tx2">
                {c.repo} {c.what === "branch" ? c.branch : `${c.agent}'s worktree`}
              </span>
              : {c.why}.
            </span>
          ))}
        </div>
      )}
      <Confirm
        open={asking}
        onClose={() => setAsking(false)}
        title="Clean up what earlier teams left?"
        body="Removes their worktrees that hold nothing uncommitted, and their wt/<project>/ branches whose work is all merged (kept aside a while, recoverable by hand). Everything else stays, and is listed with why."
        action="Clean up"
        onConfirm={run}
      >
        {unsaved.length > 0 && (
          <label className="flex items-start gap-2 text-[13.5px] text-tx2">
            <input
              type="checkbox"
              checked={uncommitted}
              onChange={(e) => setUncommitted(e.target.checked)}
              className="mt-1"
            />
            <span>
              Also remove worktrees with uncommitted work: {unsaved.join(", ")}. Their changes are
              kept aside a while, recoverable by hand.
            </span>
          </label>
        )}
      </Confirm>
    </section>
  );
}
