import { Link, useNavigate } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";
import type { Project, Request, Team } from "../api/client";
import {
  agentOf,
  greeting,
  inbox,
  type ProjectRow,
  projectRows,
  readable,
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
import { DeployDialog } from "../components/DeployDialog";
import { NewProject } from "../components/NewProject";
import { RequestPanel } from "../components/RequestPanel";
import { SetupChecklist } from "../components/SetupChecklist";
import { AddCard, TeamGrid } from "../components/TeamGrid";
import {
  Button,
  Card,
  cover,
  Loading,
  PageTitle,
  SectionTitle,
  StatusPill,
} from "../components/ui";

// The start page (wiki/product/features/dashboard.md): the projects, each
// opening where its work is, and the teams. Reviews live in each project's
// Inbox, not here.
export function Dashboard() {
  const requests = useRequests();
  const instances = useInstances();
  const projects = useProjects();
  const teams = useTeams();
  const setup = useSetup();
  const [deploy, setDeploy] = useState(false);
  const [newProject, setNewProject] = useState(false);
  const asks = inbox(requests.data ?? []);
  const latest = useLatestActivity();
  const rows = projectRows(
    latestFirst(projects.data ?? [], latest),
    teams.data ?? [],
    instances.data ?? [],
    requests.data ?? [],
  );
  const canDeploy = (teams.data ?? []).length > 0 && rows.length > 0;
  return (
    <div className="mx-auto flex w-full max-w-[1080px] flex-col gap-10 px-6 pt-6 pb-12 max-[760px]:px-4">
      <div className="flex flex-wrap items-end gap-4">
        <PageTitle title={greeting()} subtitle={workingSummary(instances.data ?? [])} />
        <span className="flex-1" />
        <div className="flex gap-2">
          <Button onClick={() => setNewProject(true)}>New project</Button>
          <Button variant="primary" disabled={!canDeploy} onClick={() => setDeploy(true)}>
            Deploy a team
          </Button>
        </div>
      </div>
      {setup.data && setupPending(setup.data) && <SetupChecklist steps={setup.data} />}
      {asks.length > 0 && (
        <section className="flex flex-col gap-2.5">
          <SectionTitle count={asks.length}>Needs you</SectionTitle>
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
        <TeamGrid />
      </Section>
      {deploy && <DeployDialog open onClose={() => setDeploy(false)} />}
      {newProject && <NewProject onClose={() => setNewProject(false)} />}
    </div>
  );
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3">
      <SectionTitle>{title}</SectionTitle>
      {children}
    </section>
  );
}

const stateColor = { "needs you": "var(--attn)", "on track": "var(--ok)", idle: "var(--tx4)" };

function ProjectCard({ row, project }: { row: ProjectRow; project?: Project }) {
  const repos = project?.repos ?? [];
  return (
    <Card
      opens
      head={
        <>
          <Link
            to="/projects/$project"
            params={{ project: row.id }}
            className={`min-w-0 flex-1 truncate text-[15px] font-semibold ${cover}`}
          >
            {readable(row.id)}
          </Link>
          <StatusPill color={stateColor[row.state]}>{row.state}</StatusPill>
        </>
      }
    >
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
        {repos.length === 0 && <span className="text-[12.5px] text-tx4">No code repo</span>}
      </span>
      <span className="text-[12.5px] text-tx4">{row.line}</span>
    </Card>
  );
}

function DashboardRequest({ request: r, teams }: { request: Request; teams: Team[] }) {
  const navigate = useNavigate();
  const answer = useAnswer();
  const agent = agentOf(teams, r.deploymentId, r.instanceId);
  const where = `on ${readable(r.projectId)}`;
  const context = agent ? `${agent.team.config.name} ${where}` : where;
  const openSession = () => navigate(sessionOf(r.instanceId));
  return (
    <RequestPanel
      request={r}
      who={agent?.soul.name}
      context={context}
      onAnswer={(text) => answer.mutate({ request: r, answer: text })}
      answering={answer.isPending}
      onOpenSession={openSession}
    />
  );
}
