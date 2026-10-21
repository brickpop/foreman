import { Link, useNavigate } from "@tanstack/react-router";
import type { Instance, Project, Team } from "../../api/client";
import { agentOf, roleLabel } from "../../app/model";
import { useInstances, useRepos, useTeams } from "../../app/queries";
import { type Worktree, worktrees } from "../../app/repos";
import { Avatar } from "../../components/Avatar";
import { Button, Empty, LeadTag, Loading, Panel, SectionLabel } from "../../components/ui";
import { InFlight } from "./Pipeline";

// The project's Worktrees tab (wiki/product/features/worktrees.md): what each
// agent has checked out, each branch a link to its code in Repositories; then
// the work on its way to main, and what merged.
export function Worktrees({ project }: { project: Project }) {
  const repos = useRepos(project.id);
  const teams = useTeams();
  const instances = useInstances();
  const list = worktrees(repos.data ?? []);
  return (
    <div className="flex flex-col gap-9 px-12 pt-6 pb-16">
      <section className="flex flex-col gap-2.5">
        <div className="flex items-baseline gap-2">
          <SectionLabel>Agent worktrees</SectionLabel>
          <span className="text-xs text-tx4">the branch each agent has checked out</span>
        </div>
        {repos.isError && <p className="m-0 text-[13px] text-err">{repos.error.message}</p>}
        {repos.isPending && <Loading />}
        {repos.isSuccess && list.length === 0 ? (
          <Empty>No agent has a worktree here yet: each gets one when its team is deployed.</Empty>
        ) : (
          list.length > 0 && (
            <Panel>
              {list.map((w) => (
                <WorktreeRow
                  key={w.agent}
                  worktree={w}
                  project={project.id}
                  teams={teams.data ?? []}
                  instance={(instances.data ?? []).find((i) => i.id === `${w.agent}@${project.id}`)}
                />
              ))}
            </Panel>
          )
        )}
      </section>
      <InFlight project={project} />
    </div>
  );
}

function WorktreeRow({
  worktree: w,
  project,
  teams,
  instance,
}: {
  worktree: Worktree;
  project: string;
  teams: Team[];
  /** Its instance while its team is deployed. */
  instance?: Instance;
}) {
  const navigate = useNavigate();
  const agent = agentOf(teams, w.agent);
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
          <span className="truncate font-medium">{agent?.soul.name ?? w.agent}</span>
          {agent && agent.team.config.lead === agent.id && <LeadTag />}
        </span>
        <span className="truncate text-[12.5px] text-tx4">
          {agent ? `${agent.team.config.name} · ${roleLabel(agent.soul.role)}` : w.agent}
        </span>
      </span>
      <span className="flex min-w-0 flex-col gap-1">
        {w.branches.map(({ repo, branch: b }) => (
          <Link
            key={repo}
            to="/projects/$project"
            params={{ project }}
            search={{ tab: "repositories", repo, branch: b.name }}
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
      </span>
      {instance ? (
        <Button
          variant="small"
          onClick={() =>
            navigate({
              to: "/sessions",
              search: { deployment: instance.deploymentId, agent: instance.id },
            })
          }
        >
          Session
        </Button>
      ) : (
        <span className="text-xs text-tx4">not deployed</span>
      )}
    </div>
  );
}
