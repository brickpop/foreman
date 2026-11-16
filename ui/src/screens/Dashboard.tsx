import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";
import { type Backup, type Project, type Request, type Team, useClient } from "../api/client";
import {
  agentOf,
  byName,
  greeting,
  inbox,
  type ProjectRow,
  projectRows,
  workingSummary,
} from "../app/model";
import { sessionOf } from "../app/project";
import {
  keys,
  useAllProjects,
  useAnswer,
  useDismiss,
  useHomeFolder,
  useInstances,
  useProjectName,
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
import { useToast } from "../components/Toasts";
import {
  Button,
  Card,
  cover,
  Loading,
  PageTitle,
  Panel,
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
  const elsewhere = (useAllProjects().data ?? []).filter((p) => !p.here);
  const teams = useTeams();
  const setup = useSetup();
  const home = useHomeFolder().data;
  const problems = home?.projectsProblems ?? [];
  const [deploy, setDeploy] = useState(false);
  const [newProject, setNewProject] = useState(false);
  const asks = inbox(requests.data ?? []);
  const rows = projectRows(
    byName(projects.data ?? []),
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
      {problems.length > 0 && (
        <p className="m-0 rounded-lg border border-err px-3 py-2 text-[13px] text-err">
          projects.toml, in Foreman's HQ, needs fixing: the projects show as last read, and can't be
          changed until then. {problems.join(" · ")}
        </p>
      )}
      {setup.data && setupPending(setup.data) && <SetupChecklist steps={setup.data} />}
      {(home?.backups ?? []).length > 0 && <NotBackedUp backups={home?.backups ?? []} />}
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
      {elsewhere.length > 0 && <Elsewhere projects={byName(elsewhere)} />}
      <Section title="Teams">
        <TeamGrid />
      </Section>
      {deploy && <DeployDialog open onClose={() => setDeploy(false)} />}
      {newProject && <NewProject onClose={() => setNewProject(false)} />}
    </div>
  );
}

/** The HQs their origin doesn't back up: what travels to a new machine
 * (wiki/product/features/dashboard.md#not-backed-up). */
function NotBackedUp({ backups }: { backups: Backup[] }) {
  const projectNameOf = useProjectName();
  return (
    <section className="flex flex-col gap-2.5">
      <SectionTitle count={backups.length}>Not backed up</SectionTitle>
      {backups.map((b) => (
        <p
          key={b.path}
          className="m-0 rounded-lg border border-attn px-3 py-2 text-[13px] break-words text-tx2"
        >
          <span className="font-semibold text-tx">
            {b.project === "" ? "Foreman's HQ" : `${projectNameOf(b.project)}'s HQ`}
          </span>{" "}
          ({b.path}): {b.problem}
        </p>
      ))}
    </section>
  );
}

/** The projects in projects.toml whose HQ isn't on this machine: another
 * machine's, brought here by cloning their HQ
 * (wiki/product/features/dashboard.md#not-on-this-machine). */
function Elsewhere({ projects }: { projects: Project[] }) {
  return (
    <section className="flex flex-col gap-2.5">
      <SectionTitle count={projects.length}>Not on this machine</SectionTitle>
      <Panel>
        {projects.map((p, i) => (
          <BringHere key={p.id} project={p} first={i === 0} />
        ))}
      </Panel>
    </section>
  );
}

function BringHere({ project: p, first }: { project: Project; first: boolean }) {
  const client = useClient();
  const queries = useQueryClient();
  const navigate = useNavigate();
  const toast = useToast();
  const projectNameOf = useProjectName();
  const bring = useMutation({
    mutationFn: () => client.home.bringHere(p.id),
    onSuccess: () => {
      queries.invalidateQueries({ queryKey: keys.projects });
      // Its code repos next: each located or cloned there.
      navigate({ to: "/projects/$project/repos", params: { project: p.id } });
      toast({ title: `${projectNameOf(p.id)} is here`, body: p.path, color: "var(--ok)" });
    },
  });
  return (
    <div className={`flex flex-col gap-1 px-4 py-3 ${first ? "" : "border-t border-line"}`}>
      <div className="flex items-center gap-3">
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="text-[14px] font-medium text-tx">{projectNameOf(p.id)}</span>
          <span className="truncate font-mono text-[12.5px] text-tx3">
            {p.remote || "Its HQ has no origin: push it where it is, then start Foreman there once"}
          </span>
        </span>
        {p.remote && (
          <Button variant="small" busy={bring.isPending} onClick={() => bring.mutate()}>
            Bring here
          </Button>
        )}
      </div>
      {bring.error && <span className="text-[13px] text-err">{bring.error.message}</span>}
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
  const projectNameOf = useProjectName();
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
            {projectNameOf(row.id)}
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
  const projectNameOf = useProjectName();
  const navigate = useNavigate();
  const answer = useAnswer();
  const dismiss = useDismiss();
  const agent = agentOf(teams, r.deploymentId, r.instanceId);
  const where = `on ${projectNameOf(r.projectId)}`;
  const context = agent ? `${agent.team.config.name} ${where}` : where;
  const openSession = () => navigate(sessionOf(r.instanceId));
  return (
    <RequestPanel
      request={r}
      who={agent?.soul.name}
      context={context}
      onAnswer={(text) => answer.mutateAsync({ request: r, answer: text })}
      onDismiss={() => dismiss.mutate(r)}
      answering={answer.isPending || dismiss.isPending}
      onOpenSession={openSession}
    />
  );
}
