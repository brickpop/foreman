import { createContext, useContext } from "react";
import { type EventHandlers, subscribeEvents } from "./events";
import { openPty, type PtyConnection, type PtyHandlers } from "./pty";
import type { Project, SettingsFile, Team } from "./types/workspace";

// Everything the UI asks of the server goes through this interface. Screens
// never call fetch or WebSocket themselves, so the transport can change and
// tests can hand components an in-memory client
// (wiki/product/architecture/overview.md#in-the-ui).

export type Workspace = { path: string };
export type Instance = { id: string };

export interface ForemanClient {
  workspace(): Promise<Workspace>;
  projects(): Promise<Project[]>;
  project(id: string): Promise<Project>;
  teams(): Promise<Team[]>;
  team(id: string): Promise<Team>;
  settings(): Promise<SettingsFile>;
  instances(): Promise<Instance[]>;
  openTerminal(instance: string, handlers: PtyHandlers): PtyConnection;
  /** Follows the server's events until the returned function is called. */
  events(handlers: EventHandlers): () => void;
}

/** The client that talks to the Foreman server the page was loaded from. */
export function httpClient(origin: string = window.location.origin): ForemanClient {
  const getJSON = async <T>(path: string): Promise<T> => {
    const r = await fetch(origin + path);
    if (!r.ok) throw new Error(`${path}: HTTP ${r.status}`);
    return r.json() as Promise<T>;
  };
  const wsOrigin = origin.replace(/^http/, "ws");
  const id = encodeURIComponent;
  return {
    workspace: () => getJSON("/api/workspace"),
    projects: () => getJSON("/api/projects"),
    project: (project) => getJSON(`/api/projects/${id(project)}`),
    teams: () => getJSON("/api/teams"),
    team: (team) => getJSON(`/api/teams/${id(team)}`),
    settings: () => getJSON("/api/settings"),
    instances: () => getJSON("/api/instances"),
    openTerminal: (instance, handlers) => openPty(`${wsOrigin}/ws/pty/${id(instance)}`, handlers),
    events: (handlers) => subscribeEvents(`${origin}/api/events`, handlers),
  };
}

export const ClientContext = createContext<ForemanClient | null>(null);

export function useClient(): ForemanClient {
  const client = useContext(ClientContext);
  if (!client) throw new Error("useClient outside a ClientContext provider");
  return client;
}
