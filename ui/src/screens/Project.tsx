import { Link, useParams, useSearch } from "@tanstack/react-router";
import { useState } from "react";
import { useProjects, useRequests, useWorkspace } from "../app/queries";
import { DeployDialog } from "../components/DeployDialog";
import { CountBadge } from "../components/ui";
import { Board } from "./project/Board";
import { Changes } from "./project/Changes";
import { Overview } from "./project/Overview";
import { ProjectSettings } from "./project/ProjectSettings";

export type ProjectTab = "overview" | "changes" | "board" | "settings";

const tabs: { tab: ProjectTab; label: string }[] = [
  { tab: "overview", label: "Overview" },
  { tab: "changes", label: "Changes" },
  { tab: "board", label: "Board" },
  { tab: "settings", label: "Settings" },
];

// A project's screen: its header and tabs (wiki/product/features/wiki-reader.md#project-header-and-tabs).
export function Project() {
  const { project: id } = useParams({ from: "/projects/$project" });
  const { tab, entry } = useSearch({ from: "/projects/$project" });
  const projects = useProjects();
  const requests = useRequests();
  const workspace = useWorkspace();
  const [deploy, setDeploy] = useState(false);
  const project = projects.data?.find((p) => p.id === id);
  if (projects.isPending) return null;
  if (!project) return <p className="p-12 text-tx3">No project {id} in this workspace.</p>;
  const open = (requests.data ?? []).filter((r) => r.status === "open" && r.projectId === id);
  return (
    <div className="flex flex-col">
      <header className="flex flex-col gap-4 px-12 pt-10">
        <div className="flex items-end gap-4">
          <div className="flex flex-1 flex-col gap-1">
            <h1 className="m-0 text-[30px] font-semibold tracking-[-0.02em]">{project.id}</h1>
            {project.config.description && (
              <p className="m-0 text-[15px] text-tx3">{project.config.description}</p>
            )}
          </div>
          <span className="font-mono text-[12.5px] text-tx4">
            {workspace.data?.path ?? "~"}/{project.id}
          </span>
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
              {t.tab === "changes" && <CountBadge count={open.length} small />}
            </Link>
          ))}
        </nav>
      </header>
      {/* Keyed by project: switching projects starts each tab fresh (tree, board, editors). */}
      {tab === "overview" && <Overview key={id} project={id} entry={entry} />}
      {tab === "changes" && <Changes key={id} project={project} />}
      {tab === "board" && <Board key={id} project={id} />}
      {tab === "settings" && (
        <ProjectSettings key={id} project={project} onDeploy={() => setDeploy(true)} />
      )}
      {/* Mounted only while open: each opening starts fresh, on this project. */}
      {deploy && <DeployDialog open onClose={() => setDeploy(false)} project={id} />}
    </div>
  );
}
