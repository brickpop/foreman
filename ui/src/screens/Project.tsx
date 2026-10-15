import { Link, useParams, useSearch } from "@tanstack/react-router";
import { useState } from "react";
import { useProjects, useRequests, useWorkspace } from "../app/queries";
import { DemoBadge } from "../components/DemoBadge";
import { Modal } from "../components/Overlays";
import { CountBadge } from "../components/ui";
import { ComingWith } from "./Placeholder";
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
            <h1 className="m-0 flex items-center gap-3 text-[30px] font-semibold tracking-[-0.02em]">
              {project.id}
              <DemoBadge areas={["workspace", "wiki", "pipeline", "requests"]} />
            </h1>
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
      {tab === "overview" && <Overview project={id} entry={entry} />}
      {tab === "changes" && <Changes project={project} />}
      {tab === "board" && <Board project={id} />}
      {tab === "settings" && <ProjectSettings project={project} onDeploy={() => setDeploy(true)} />}
      <Modal open={deploy} onClose={() => setDeploy(false)} title="Deploy a team">
        <ComingWith card="Team and Agent pages, and the Deploy dialog">
          The Deploy dialog
        </ComingWith>
      </Modal>
    </div>
  );
}
