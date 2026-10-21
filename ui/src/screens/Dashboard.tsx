import { Link, useNavigate } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";
import type { Project, Request, Team } from "../api/client";
import { useOpenChiefOfStaff } from "../app/chiefOfStaff";
import {
  agentOf,
  dashboardSubtitle,
  greeting,
  inbox,
  type ProjectRow,
  projectRows,
  roleLabel,
  type SessionGroup,
  sessionGroups,
  type TeamCard,
  teamCards,
  when,
} from "../app/model";
import {
  useAnswer,
  useDeployments,
  useInstances,
  useProjects,
  useRequests,
  useSetup,
  useTeams,
} from "../app/queries";
import { setupPending } from "../app/setup";
import { Avatar } from "../components/Avatar";
import { DeployDialog } from "../components/DeployDialog";
import { NewProject } from "../components/NewProject";
import { RequestPanel } from "../components/RequestPanel";
import { SetupChecklist } from "../components/SetupChecklist";
import { Button, Dot, Empty, LeadTag, Loading, PageTitle, SectionLabel } from "../components/ui";

// The start page (wiki/product/features/dashboard.md): what is at work, the
// projects and the teams, each a click from where it's handled. Reviews live
// in Reviews, not here.
export function Dashboard() {
  const requests = useRequests();
  const instances = useInstances();
  const deployments = useDeployments();
  const projects = useProjects();
  const teams = useTeams();
  const setup = useSetup();
  const [deploy, setDeploy] = useState<{ team?: string }>();
  const [newProject, setNewProject] = useState(false);
  const openChiefOfStaff = useOpenChiefOfStaff();
  const asks = inbox(requests.data ?? []);
  const groups = sessionGroups(instances.data ?? [], deployments.data ?? [], teams.data ?? []);
  const rows = projectRows(
    projects.data ?? [],
    teams.data ?? [],
    instances.data ?? [],
    requests.data ?? [],
  );
  const cards = teamCards(teams.data ?? [], instances.data ?? []);
  const canDeploy = cards.length > 0 && rows.length > 0;
  return (
    <div className="mx-auto flex w-full max-w-[1080px] flex-col gap-10 px-12 pt-12 pb-16 max-[760px]:px-4">
      <div className="flex flex-wrap items-end gap-4">
        <PageTitle title={greeting()} subtitle={dashboardSubtitle(instances.data ?? [])} />
        <span className="flex-1" />
        <div className="flex gap-2">
          <Button onClick={() => setNewProject(true)}>New project</Button>
          <Button variant="primary" disabled={!canDeploy} onClick={() => setDeploy({})}>
            Deploy a team
          </Button>
        </div>
      </div>
      {setup.data && setupPending(setup.data) && <SetupChecklist steps={setup.data} />}
      {asks.length > 0 && (
        <section className="flex flex-col gap-2.5">
          <SectionLabel>Needs you</SectionLabel>
          {asks.map((r) => (
            <DashboardRequest key={r.id} request={r} teams={teams.data ?? []} />
          ))}
        </section>
      )}
      <Section
        title="At work"
        aside={
          groups.length > 0 && (
            <Link to="/sessions" className="text-[13px] text-tx3 hover:text-tx">
              All sessions →
            </Link>
          )
        }
      >
        {instances.isPending ? (
          <Loading />
        ) : groups.length === 0 ? (
          <Empty>
            <span className="flex flex-col items-center gap-3">
              {canDeploy
                ? "No team is at work. Deploy one on a project, with a goal, and its sessions show here."
                : "No team is at work yet. Add a project and a team, then deploy the team on it."}
              {canDeploy && (
                <Button variant="primary" onClick={() => setDeploy({})}>
                  Deploy a team
                </Button>
              )}
            </span>
          </Empty>
        ) : (
          <div className="grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-3">
            {groups.map((g) => (
              <Deployed key={g.deployment} group={g} />
            ))}
          </div>
        )}
      </Section>
      <Section title="Projects">
        <div className="grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-3">
          {projects.isPending && <Loading className="min-h-[132px]" />}
          {rows.map((row) => (
            <ProjectCard
              key={row.id}
              row={row}
              project={projects.data?.find((p) => p.id === row.id)}
            />
          ))}
          <AddCard onClick={() => setNewProject(true)}>
            <span className="font-semibold text-tx2">+ New project</span>
            <span>Your repos, any folder on this machine.</span>
          </AddCard>
        </div>
      </Section>
      <Section title="Teams">
        <div className="grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-3">
          {teams.isPending && <Loading className="min-h-[132px]" />}
          {cards.map((t) => (
            <TeamCardView
              key={t.id}
              team={t}
              onDeploy={rows.length > 0 ? () => setDeploy({ team: t.id }) : undefined}
            />
          ))}
          <AddCard onClick={() => openChiefOfStaff({ start: "create-team" })}>
            <span className="font-semibold text-tx2">+ New team</span>
            <span>The Chief of Staff drafts it with you.</span>
          </AddCard>
        </div>
      </Section>
      {deploy && <DeployDialog open onClose={() => setDeploy(undefined)} team={deploy.team} />}
      {newProject && <NewProject onClose={() => setNewProject(false)} />}
    </div>
  );
}

function Section({
  title,
  aside,
  children,
}: {
  title: string;
  aside?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="flex flex-col gap-3">
      <div className="flex items-baseline justify-between">
        <SectionLabel>{title}</SectionLabel>
        {aside}
      </div>
      {children}
    </section>
  );
}

const card = "relative flex flex-col gap-3 rounded-xl border border-line bg-panel px-4 py-3.5";
// A card's title link covers the whole card; its other controls sit above it.
const cover = "after:absolute after:inset-0 after:rounded-xl after:content-['']";

/** One deployment at work: its team, goal and awake agents, each opening its
 * session; the asleep ones only counted. */
function Deployed({ group: g }: { group: SessionGroup }) {
  const navigate = useNavigate();
  const working = g.agents.filter((a) => a.instance.status === "running").length;
  return (
    <article className={`${card} !gap-2.5 !px-0 !pb-2 hover:border-line3`}>
      <header className="flex flex-col gap-0.5 px-4">
        <span className="flex items-baseline gap-2">
          <Link
            to="/sessions"
            search={{ deployment: g.deployment }}
            className={`font-semibold ${cover}`}
          >
            {g.team?.config.name || g.deployment.split("@")[0]}
          </Link>
          <span className="text-[13px] text-tx4">on {g.project}</span>
          <span className="flex-1" />
          <span className={`text-[12.5px] ${working > 0 ? "text-ok" : "text-tx4"}`}>
            {working > 0 ? `${working} working` : g.agents.length > 0 ? "idle" : "asleep"}
          </span>
        </span>
        {g.goal && <p className="line-clamp-2 text-[13.5px] text-tx3">{g.goal}</p>}
      </header>
      <div className="flex flex-col">
        {g.agents.map(({ instance: i, agent, lead }) => (
          <button
            key={i.id}
            type="button"
            onClick={() =>
              navigate({ to: "/sessions", search: { deployment: g.deployment, agent: i.id } })
            }
            className="relative z-10 grid grid-cols-[auto_1fr] items-center gap-x-2.5 px-4 py-1.5 text-left hover:bg-hover"
          >
            <Avatar look={agent?.soul.look} size={28} status={i.status} dotBorder="var(--panel)" />
            <span className="flex min-w-0 flex-col">
              <span className="flex items-center gap-1.5 text-[13.5px]">
                <span className="font-medium">{agent?.soul.name ?? i.agentId}</span>
                {lead && <LeadTag />}
                <span className="truncate text-tx4">{agent ? roleLabel(agent.soul.role) : ""}</span>
              </span>
              <span className="truncate font-mono text-[11.5px] text-tx4">
                {i.doing ? `${i.doing.line} · ${when(i.doing.at)}` : i.status}
              </span>
            </span>
          </button>
        ))}
        {g.asleep.length > 0 && (
          <span className="flex items-center gap-2.5 px-4 py-1.5 text-[12.5px] text-tx4">
            <span className="flex">
              {g.asleep.slice(0, 5).map(({ instance: i, agent }, n) => (
                <span
                  key={i.id}
                  className={n > 0 ? "-ml-1.5" : ""}
                  title={agent?.soul.name ?? i.agentId}
                >
                  <Avatar look={agent?.soul.look} size={20} />
                </span>
              ))}
            </span>
            {g.asleep.length} asleep · woken when needed
          </span>
        )}
      </div>
    </article>
  );
}

const stateColor = { "needs you": "var(--acc)", "on track": "var(--ok)", idle: "var(--tx4)" };

function ProjectCard({ row, project }: { row: ProjectRow; project?: Project }) {
  const repos = project?.repos ?? [];
  return (
    <article className={`${card} hover:border-line3`}>
      <span className="flex items-center gap-2">
        <Link
          to="/projects/$project"
          params={{ project: row.id }}
          search={{ tab: "overview" }}
          className={`text-[15px] font-semibold ${cover}`}
        >
          {row.id}
        </Link>
        <span className="flex-1" />
        <Dot color={stateColor[row.state]} title={row.state} />
        <span className="text-[12.5px] text-tx3">{row.state}</span>
      </span>
      <p className="line-clamp-2 min-h-[2lh] text-[13.5px] text-tx3">
        {project?.config.description || "No description yet."}
      </p>
      <span className="flex flex-wrap gap-1.5">
        {repos.slice(0, 3).map((r) => (
          <span
            key={r.name}
            className={`rounded-[5px] bg-raised px-1.5 py-px font-mono text-[11.5px] ${r.problem ? "text-err" : "text-tx3"}`}
            title={r.problem || r.path}
          >
            {r.name}
          </span>
        ))}
        {repos.length > 3 && (
          <span className="font-mono text-[11.5px] text-tx4">+{repos.length - 3}</span>
        )}
        {repos.length === 0 && <span className="text-[12.5px] text-tx4">No code repo yet</span>}
      </span>
      <span className="text-[12.5px] text-tx4">{row.line}</span>
    </article>
  );
}

function TeamCardView({ team: t, onDeploy }: { team: TeamCard; onDeploy?: () => void }) {
  const shown = t.members.slice(0, 6);
  return (
    <article className={`${card} hover:border-line3`}>
      <Link
        to="/teams/$team"
        params={{ team: t.id }}
        search={{ tab: "deployments" }}
        className={`text-[15px] font-semibold ${cover}`}
      >
        {t.name}
      </Link>
      <p className="line-clamp-2 min-h-[2lh] text-[13.5px] text-tx3">
        {t.purpose || "No purpose written yet."}
      </p>
      <span className="flex items-center">
        {shown.map((soul, n) => (
          <span key={soul.name} className={n > 0 ? "-ml-1.5" : ""} title={soul.name}>
            <Avatar look={soul.look} size={26} />
          </span>
        ))}
        {t.members.length > shown.length && (
          <span className="ml-1.5 font-mono text-[11.5px] text-tx4">
            +{t.members.length - shown.length}
          </span>
        )}
        <span className="ml-2 text-[12.5px] text-tx4">
          {t.members.length} {t.members.length === 1 ? "agent" : "agents"}
        </span>
      </span>
      <span className="flex items-center gap-2">
        <span className="min-w-0 flex-1 truncate text-[12.5px] text-tx4">
          {t.on.length > 0 ? `On ${t.on.join(", ")}` : "Not deployed"}
        </span>
        {onDeploy && (
          <Button variant="ghost" className="relative z-10" onClick={onDeploy}>
            Deploy
          </Button>
        )}
      </span>
    </article>
  );
}

function AddCard({ onClick, children }: { onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex min-h-[132px] flex-col items-start justify-center gap-1 rounded-xl border border-dashed border-line2 px-4 py-3.5 text-left text-[13px] text-tx4 hover:border-line3 hover:text-tx3"
    >
      {children}
    </button>
  );
}

function DashboardRequest({ request: r, teams }: { request: Request; teams: Team[] }) {
  const navigate = useNavigate();
  const answer = useAnswer();
  const agent = agentOf(teams, r.instanceId);
  const where = `on ${r.projectId}`;
  const context = agent ? `${agent.team.config.name} ${where}` : where;
  const openSession = () =>
    navigate({ to: "/sessions", search: { deployment: r.deploymentId, agent: r.instanceId } });
  return (
    <RequestPanel
      request={r}
      who={agent && { name: agent.soul.name, look: agent.soul.look }}
      context={context}
      onAnswer={(text) => answer.mutate({ request: r, answer: text })}
      answering={answer.isPending}
      onOpenSession={openSession}
    />
  );
}
