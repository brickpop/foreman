import { Link, useNavigate } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";
import type { Project, Request, Team } from "../api/client";
import { useOpenChiefOfStaff } from "../app/chiefOfStaff";
import {
  agentOf,
  greeting,
  inbox,
  type ProjectRow,
  projectRows,
  type TeamCard,
  teamCards,
  workingSummary,
} from "../app/model";
import { latestFirst, sessionOf } from "../app/project";
import {
  useAnswer,
  useInstances,
  useLatestActivity,
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
import { Button, card, cover, Dot, Loading, PageTitle, SectionLabel } from "../components/ui";

// The start page (wiki/product/features/dashboard.md): the projects, each
// opening where its work is, and the teams. Reviews live in each project's
// Inbox, not here.
export function Dashboard() {
  const requests = useRequests();
  const instances = useInstances();
  const projects = useProjects();
  const teams = useTeams();
  const setup = useSetup();
  const [deploy, setDeploy] = useState<{ team?: string }>();
  const [newProject, setNewProject] = useState(false);
  const openChiefOfStaff = useOpenChiefOfStaff();
  const asks = inbox(requests.data ?? []);
  const latest = useLatestActivity();
  const rows = projectRows(
    latestFirst(projects.data ?? [], latest),
    teams.data ?? [],
    instances.data ?? [],
    requests.data ?? [],
  );
  const cards = teamCards(teams.data ?? [], instances.data ?? []);
  const canDeploy = cards.length > 0 && rows.length > 0;
  return (
    <div className="mx-auto flex w-full max-w-[1080px] flex-col gap-10 px-6 pt-6 pb-12 max-[760px]:px-4">
      <div className="flex flex-wrap items-end gap-4">
        <PageTitle title={greeting()} subtitle={workingSummary(instances.data ?? [])} />
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

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3">
      <SectionLabel>{title}</SectionLabel>
      {children}
    </section>
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
  const openSession = () => navigate(sessionOf(r.instanceId));
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
