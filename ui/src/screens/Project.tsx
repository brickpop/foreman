import { Link, useParams, useSearch } from "@tanstack/react-router";
import { useState } from "react";
import { inbox } from "../app/model";
import { useInstances, useProjects, useRequests, useTeams } from "../app/queries";
import { deploymentTabs, mostRecentAgent } from "../app/sessions";
import { DeployDialog } from "../components/DeployDialog";
import { Button, CountBadge, Loading, outlineLink } from "../components/ui";
import { Activity } from "./project/Activity";
import { Board } from "./project/Board";
import { Overview } from "./project/Overview";
import { NeedsYou } from "./project/Pipeline";
import { ProjectSettings } from "./project/ProjectSettings";
import { Repositories } from "./project/Repositories";
import { Worktrees } from "./project/Worktrees";

export type ProjectTab =
  | "overview"
  | "board"
  | "repositories"
  | "worktrees"
  | "activity"
  | "settings";

const tabs: { tab: ProjectTab; label: string }[] = [
  { tab: "overview", label: "Overview" },
  { tab: "board", label: "Board" },
  { tab: "repositories", label: "Repositories" },
  { tab: "worktrees", label: "Worktrees" },
  { tab: "activity", label: "Activity" },
  { tab: "settings", label: "Settings" },
];

// A project's screen: its header and tabs (wiki/product/features/wiki-reader.md#project-header-and-tabs).
export function Project() {
  const { project: id } = useParams({ from: "/projects/$project" });
  const { tab, entry } = useSearch({ from: "/projects/$project" });
  const projects = useProjects();
  const requests = useRequests();
  const [deploy, setDeploy] = useState(false);
  const project = projects.data?.find((p) => p.id === id);
  if (projects.isPending) return <Loading rows={5} className="m-12" />;
  if (!project) return <p className="p-12 text-tx3">No project {id}.</p>;
  const open = inbox(requests.data ?? []).filter((r) => r.projectId === id);
  return (
    <div className="flex min-h-full flex-col">
      <header className="flex flex-col gap-4 px-12 pt-10">
        <div className="flex items-start gap-4">
          <div className="flex flex-1 flex-col gap-1">
            <h1 className="m-0 text-[30px] font-semibold tracking-[-0.02em]">{project.id}</h1>
            <span className="font-mono text-[12.5px] text-tx4">{project.path}</span>
            {project.config.description && (
              <p className="m-0 mt-1 text-[15px] text-tx3">{project.config.description}</p>
            )}
          </div>
          <ToTheWork project={id} onDeploy={() => setDeploy(true)} />
        </div>
        <nav className="flex border-b border-line">
          {tabs.map((t) => (
            <Link
              key={t.tab}
              to="/projects/$project"
              params={{ project: id }}
              search={{ tab: t.tab }}
              className={`-mb-px flex items-center gap-2 border-b-2 px-3.5 py-[9px] font-medium ${
                t.tab === tab ? "border-acc text-tx" : "border-transparent text-tx3 hover:text-tx2"
              }`}
            >
              {t.label}
              {t.tab === "overview" && <CountBadge count={open.length} small />}
            </Link>
          ))}
        </nav>
      </header>
      {/* Keyed by project: switching projects starts each tab fresh (tree, board, editors). */}
      {tab === "overview" && (
        <>
          <NeedsYou project={project} />
          <Overview key={id} project={id} entry={entry} />
        </>
      )}
      {tab === "activity" && <Activity key={id} project={id} />}
      {tab === "repositories" && <Repositories key={id} project={project} />}
      {tab === "worktrees" && <Worktrees key={id} project={project} />}
      {tab === "board" && <Board key={id} project={id} />}
      {tab === "settings" && (
        <ProjectSettings key={id} project={project} onDeploy={() => setDeploy(true)} />
      )}
      {/* Mounted only while open: each opening starts fresh, on this project. */}
      {deploy && <DeployDialog open onClose={() => setDeploy(false)} project={id} />}
    </div>
  );
}

/**
 * The way to the work on this project, without thinking: one link per team
 * with sessions here, as Sessions has one back to the project; Deploy when
 * there is none (wiki/product/features/wiki-reader.md#project-header-and-tabs).
 */
function ToTheWork({ project, onDeploy }: { project: string; onDeploy: () => void }) {
  const instances = useInstances();
  const teams = useTeams();
  if (!instances.data) return null;
  // The same tabs Sessions shows, so a link here always has sessions there.
  const here = deploymentTabs(instances.data, teams.data ?? [], []).filter(
    (d) => d.project === project,
  );
  if (here.length === 0)
    return (
      <Button variant="primary" onClick={onDeploy}>
        Deploy a team
      </Button>
    );
  return (
    <div className="flex gap-2">
      {here.map((d) => (
        <Link
          key={d.id}
          to="/sessions"
          // Straight to where the work is: the agent with the latest output.
          search={{ deployment: d.id, agent: mostRecentAgent(instances.data, d.id) }}
          className={outlineLink}
        >
          {d.team} · Sessions
        </Link>
      ))}
    </div>
  );
}
