import { createContext, useContext } from "react";
import { type EventHandlers, subscribeEvents } from "./events";
import { openPty, type PtyConnection, type PtyHandlers } from "./pty";
import type { Status as HarnessStatus } from "./types/harness";
import type {
  Deployment,
  Instance,
  MergeRequest,
  Request,
  TeamMessage,
  WorkItem,
} from "./types/store";
import type { Board, Entry, Node } from "./types/wiki";
import type { Project, SettingsFile, Team } from "./types/workspace";

// Everything the UI asks of the server goes through this interface, grouped
// in areas that match the backend's blocks. Screens never call fetch or
// WebSocket themselves, so the transport can change and tests can hand
// components an in-memory client. main.tsx picks the real or the demo client
// per area, one line each (wiki/product/architecture/overview.md#in-the-ui).

export type {
  Board,
  Deployment,
  Entry,
  HarnessStatus,
  Instance,
  MergeRequest,
  Node,
  Project,
  Request,
  SettingsFile,
  Team,
  TeamMessage,
  WorkItem,
};

/** Every area says whether it serves demo data, for the "demo data" badge. */
interface Area {
  readonly demo: boolean;
}

export type Workspace = { path: string };

export interface WorkspaceArea extends Area {
  workspace(): Promise<Workspace>;
  projects(): Promise<Project[]>;
  project(id: string): Promise<Project>;
  teams(): Promise<Team[]>;
  team(id: string): Promise<Team>;
  settings(): Promise<SettingsFile>;
}

export interface AgentsArea extends Area {
  instances(): Promise<Instance[]>;
  deploy(team: string, project: string, goal: string): Promise<Deployment>;
  stop(deployment: string): Promise<void>;
  openTerminal(id: string, handlers: PtyHandlers): PtyConnection;
}

export interface HarnessesArea extends Area {
  list(): Promise<HarnessStatus[]>;
  /** Starts the harness's login; returns the terminal to open with agents.openTerminal. */
  connect(harness: string): Promise<string>;
}

export interface RequestsArea extends Area {
  list(): Promise<Request[]>;
  resolve(id: string, answer: string): Promise<void>;
}

export interface MessagesArea extends Area {
  list(deployment: string): Promise<TeamMessage[]>;
}

export type Pipeline = { workItems: WorkItem[]; mergeRequests: MergeRequest[] };

export interface PipelineArea extends Area {
  list(project: string): Promise<Pipeline>;
  merge(mergeRequest: string): Promise<void>;
  createPr(mergeRequest: string): Promise<void>;
  reject(mergeRequest: string, reason: string): Promise<void>;
}

export interface WikiArea extends Area {
  tree(project: string): Promise<Node[]>;
  entry(project: string, path: string): Promise<Entry>;
  board(project: string, id: string): Promise<Board>;
  setCheckbox(
    project: string,
    path: string,
    line: number,
    done: boolean,
    version: string,
  ): Promise<void>;
  moveCard(
    project: string,
    board: string,
    path: string,
    column: string,
    lane: string,
    version: string,
  ): Promise<void>;
}

export interface EventsArea extends Area {
  /** Follows the events until the returned function is called. */
  subscribe(handlers: EventHandlers): () => void;
}

export interface ForemanClient {
  workspace: WorkspaceArea;
  agents: AgentsArea;
  harnesses: HarnessesArea;
  requests: RequestsArea;
  messages: MessagesArea;
  pipeline: PipelineArea;
  wiki: WikiArea;
  events: EventsArea;
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
  const send = async (method: string, path: string, body?: unknown): Promise<void> => {
    await request(method, path, body);
  };
  const wsOrigin = origin.replace(/^http/, "ws");
  const id = encodeURIComponent;
  return {
    workspace: {
      demo: false,
      workspace: () => getJSON("/api/workspace"),
      projects: () => getJSON("/api/projects"),
      project: (project) => getJSON(`/api/projects/${id(project)}`),
      teams: () => getJSON("/api/teams"),
      team: (team) => getJSON(`/api/teams/${id(team)}`),
      settings: () => getJSON("/api/settings"),
    },
    agents: {
      demo: false,
      instances: () => getJSON("/api/instances"),
      deploy: async (team, project, goal) =>
        (
          await request("POST", "/api/deployments", { team, project, goal })
        ).json() as Promise<Deployment>,
      stop: (deployment) => send("DELETE", `/api/deployments/${id(deployment)}`),
      openTerminal: (instance, handlers) => openPty(`${wsOrigin}/ws/pty/${id(instance)}`, handlers),
    },
    harnesses: {
      demo: false,
      list: () => getJSON("/api/harnesses"),
      connect: async (harness) => {
        const r = await request("POST", `/api/harnesses/${id(harness)}/connect`);
        return ((await r.json()) as { terminal: string }).terminal;
      },
    },
    requests: {
      demo: false,
      list: () => getJSON("/api/requests?status=open"),
      resolve: (req, answer) => send("POST", `/api/requests/${id(req)}/resolve`, { answer }),
    },
    messages: {
      demo: false,
      list: (deployment) => getJSON(`/api/messages?deployment=${id(deployment)}`),
    },
    pipeline: {
      demo: false,
      list: (project) => getJSON(`/api/pipeline?project=${id(project)}`),
      merge: (mr) => send("POST", `/api/merge-requests/${id(mr)}/merge`),
      createPr: (mr) => send("POST", `/api/merge-requests/${id(mr)}/create-pr`),
      reject: (mr, reason) => send("POST", `/api/merge-requests/${id(mr)}/reject`, { reason }),
    },
    wiki: {
      demo: false,
      tree: (project) => getJSON(`/api/wiki/${id(project)}/tree`),
      entry: (project, path) => getJSON(`/api/wiki/${id(project)}/entry/${path}`),
      board: (project, board) => getJSON(`/api/wiki/${id(project)}/board/${id(board)}`),
      setCheckbox: (project, path, line, done, version) =>
        send("PUT", `/api/wiki/${id(project)}/checkbox/${path}`, { line, done, version }),
      moveCard: (project, board, path, column, lane, version) =>
        send("PUT", `/api/wiki/${id(project)}/card/${id(board)}/${path}`, {
          value: column,
          lane,
          version,
        }),
    },
    events: {
      demo: false,
      subscribe: (handlers) => subscribeEvents(`${origin}/api/events`, handlers),
    },
  };
}

export const ClientContext = createContext<ForemanClient | null>(null);

export function useClient(): ForemanClient {
  const client = useContext(ClientContext);
  if (!client) throw new Error("useClient outside a ClientContext provider");
  return client;
}
