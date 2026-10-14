import { createContext, useContext } from "react";
import { type EventHandlers, subscribeEvents } from "./events";
import { openPty, type PtyConnection, type PtyHandlers } from "./pty";
import type { Status as HarnessStatus } from "./types/harness";
import type { Deployment, Instance } from "./types/store";
import type { Project, SettingsFile, Team } from "./types/workspace";

// Everything the UI asks of the server goes through this interface. Screens
// never call fetch or WebSocket themselves, so the transport can change and
// tests can hand components an in-memory client
// (wiki/product/architecture/overview.md#in-the-ui).

export type Workspace = { path: string };
export type { Deployment, HarnessStatus, Instance };

export interface ForemanClient {
  workspace(): Promise<Workspace>;
  projects(): Promise<Project[]>;
  project(id: string): Promise<Project>;
  teams(): Promise<Team[]>;
  team(id: string): Promise<Team>;
  settings(): Promise<SettingsFile>;
  instances(): Promise<Instance[]>;
  deploy(team: string, project: string, goal: string): Promise<Deployment>;
  stop(deployment: string): Promise<void>;
  harnesses(): Promise<HarnessStatus[]>;
  /** Starts the harness's login; returns the terminal to open with openTerminal. */
  connect(harness: string): Promise<string>;
  openTerminal(instance: string, handlers: PtyHandlers): PtyConnection;
  /** Follows the server's events until the returned function is called. */
  events(handlers: EventHandlers): () => void;
}

/** The client that talks to the Foreman server the page was loaded from. */
export function httpClient(origin: string = window.location.origin): ForemanClient {
  const request = async (method: string, path: string, body?: unknown): Promise<Response> => {
    const r = await fetch(origin + path, {
      method,
      headers: body === undefined ? undefined : { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (!r.ok) {
      // The API's error format: {error, code}.
      const e = (await r.json().catch(() => ({}))) as { error?: string };
      throw new Error(e.error ?? `${path}: HTTP ${r.status}`);
    }
    return r;
  };
  const getJSON = async <T>(path: string): Promise<T> =>
    (await request("GET", path)).json() as Promise<T>;
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
    deploy: async (team, project, goal) =>
      (
        await request("POST", "/api/deployments", { team, project, goal })
      ).json() as Promise<Deployment>,
    stop: async (deployment) => {
      await request("DELETE", `/api/deployments/${id(deployment)}`);
    },
    harnesses: () => getJSON("/api/harnesses"),
    connect: async (harness) => {
      const r = await request("POST", `/api/harnesses/${id(harness)}/connect`);
      return ((await r.json()) as { terminal: string }).terminal;
    },
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
