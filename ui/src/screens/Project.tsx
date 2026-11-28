import { Navigate, Outlet, useLocation, useParams } from "@tanstack/react-router";
import { useEffect } from "react";
import type { Project as ProjectData } from "../api/client";
import { projectName } from "../app/model";
import { projectHome, sectionOf, windowTitle } from "../app/project";
import { useInstances, useProjects } from "../app/queries";
import { Loading } from "../components/ui";

// A project's page (wiki/product/features/project-page.md): a bar naming the
// project and the section, the rail showing its sections; each section is a
// route under it.

export function Project() {
  const { project: id } = useParams({ from: "/projects/$project" });
  const projects = useProjects();
  const name = projectName(projects.data, id);
  const section = sectionOf(useLocation({ select: (l) => l.pathname }));
  // The window says which project it is; any other page is just Foreman.
  useEffect(() => {
    document.title = windowTitle(name);
    return () => {
      document.title = windowTitle();
    };
  }, [name]);
  if (projects.isPending) return <Loading rows={5} className="m-6 max-[760px]:mt-14" />;
  if (!projects.data?.some((p) => p.id === id))
    return <p className="p-6 text-tx3 max-[760px]:pt-14">No project {id}.</p>;
  // Keyed by project: switching projects starts each section fresh (tree,
  // board, editors, terminal). The project's bar heads every section but
  // Session, which draws its own header, or this bar while no team is on it.
  return (
    <div key={id} className="flex h-full min-h-0 flex-col">
      {section && section.path !== "session" && <ProjectBar section={section.label} />}
      <div className="min-h-0 flex-1 overflow-auto">
        <Outlet />
      </div>
    </div>
  );
}

/** The bar heading a project's section: "<project> · <section>". */
export function ProjectBar({ section }: { section: string }) {
  const { project: id } = useParams({ from: "/projects/$project" });
  const name = projectName(useProjects().data, id);
  return (
    // Under 760px, room on the left for the menu button.
    <header className="flex shrink-0 items-center border-b border-line px-4 py-2.5 max-[760px]:pl-14">
      <h1 className="m-0 truncate text-[15px] font-semibold">
        {name} <span className="font-normal text-tx3">· {section}</span>
      </h1>
    </header>
  );
}

/** The project shown, for its sections: the page renders them only once it
 * is known. */
export function useProject(): ProjectData {
  const { project: id } = useParams({ from: "/projects/$project" });
  const project = useProjects().data?.find((p) => p.id === id);
  if (!project) throw new Error(`no project ${id}`);
  return project;
}

/** /projects/<id>: its Session while a team is on it, else its Wiki. */
export function ProjectHome() {
  const { project } = useParams({ from: "/projects/$project" });
  const instances = useInstances();
  if (!instances.data) return <Loading rows={5} className="m-6 max-[760px]:mt-14" />;
  return (
    <Navigate
      to={`/projects/$project/${projectHome(project, instances.data)}`}
      params={{ project }}
      replace
    />
  );
}
