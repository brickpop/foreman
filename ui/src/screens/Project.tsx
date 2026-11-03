import { Navigate, Outlet, useParams } from "@tanstack/react-router";
import { useEffect } from "react";
import type { Project as ProjectData } from "../api/client";
import { projectHome, windowTitle } from "../app/project";
import { useInstances, useProjects } from "../app/queries";
import { Loading } from "../components/ui";

// A project's page (wiki/product/features/project-page.md): no header of its
// own, the sidebar lists its sections; each section is a route under it.

export function Project() {
  const { project: id } = useParams({ from: "/projects/$project" });
  const projects = useProjects();
  // The window says which project it is; any other page is just Foreman.
  useEffect(() => {
    document.title = windowTitle(id);
    return () => {
      document.title = windowTitle();
    };
  }, [id]);
  if (projects.isPending) return <Loading rows={5} className="m-6" />;
  if (!projects.data?.some((p) => p.id === id))
    return <p className="p-6 text-tx3">No project {id}.</p>;
  // Keyed by project: switching projects starts each section fresh (tree,
  // board, editors, terminal).
  return (
    <div key={id} className="contents">
      <Outlet />
    </div>
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
  if (!instances.data) return <Loading rows={5} className="m-6" />;
  return (
    <Navigate
      to={`/projects/$project/${projectHome(project, instances.data)}`}
      params={{ project }}
      replace
    />
  );
}
