import { createContext, useContext } from "react";
import { type EventHandlers, subscribeEvents } from "./events";
import { openPty, type PtyConnection, type PtyHandlers } from "./pty";
import type { Account as ForgeAccount } from "./types/forge";
import type { Status as HarnessStatus } from "./types/harness";
import type { ImageStatus, Maintenance } from "./types/runner";
import type {
  Deployment,
  Instance,
  MergeRequest,
  Request,
  TeamMessage,
  WorkItem,
} from "./types/store";
import type { Board, Card, Entry, Node } from "./types/wiki";
import type { Candidate, Project, Settings, SettingsFile, Soul, Team } from "./types/workspace";

// Everything the UI asks of the server goes through this interface, grouped
// in areas that match the backend's blocks. Screens never call fetch or
// WebSocket themselves, so the transport can change and tests can hand
// components an in-memory client. main.tsx picks the real or the demo client
// per area, one line each (wiki/product/architecture/overview.md#in-the-ui).

export type {
  Board,
  Candidate,
  Card,
  Deployment,
  Entry,
  ForgeAccount,
  HarnessStatus,
  ImageStatus,
  Instance,
  Maintenance,
  MergeRequest,
  Node,
  Project,
  Request,
  Settings,
  SettingsFile,
  Soul,
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
  /** Saves settings.toml, guarded by the version read. */
  saveSettings(settings: Settings, version: string): Promise<SettingsFile>;
  /** Folders that aren't projects yet. */
  candidates(): Promise<Candidate[]>;
  /** Turns a candidate folder into a project: its project repo and starter wiki. */
  addProject(folder: string): Promise<Project>;
  /** Saves one agent's soul in its team file, guarded by the version read. */
  saveSoul(team: string, agent: string, soul: Soul, version: string): Promise<Team>;
  /** An agent's prompt before any deployment, with {placeholders}. */
  promptTemplate(team: string, agent: string): Promise<string>;
}

export interface AgentsArea extends Area {
  instances(): Promise<Instance[]>;
  /** The active deployments. */
  deployments(): Promise<Deployment[]>;
  /** An instance's session prompt, exactly as composed for its harness. */
  prompt(instance: string): Promise<string>;
  deploy(team: string, project: string, goal: string): Promise<Deployment>;
  stop(deployment: string): Promise<void>;
  openTerminal(id: string, handlers: PtyHandlers): PtyConnection;
}

export interface HarnessesArea extends Area {
  list(): Promise<HarnessStatus[]>;
  /** Starts the harness's login; returns the terminal to open with agents.openTerminal. */
  connect(harness: string): Promise<string>;
}

export interface MaintenanceArea extends Area {
  /** The engine's state and its images. */
  status(): Promise<Maintenance>;
  /** Starts rebuilding the images without cache; status shows the progress. */
  rebuild(): Promise<void>;
}

export interface ForgeArea extends Area {
  accounts(): Promise<ForgeAccount[]>;
  /** Checks the token against the forge, then stores it with the host. */
  connect(kind: string, url: string, token: string): Promise<void>;
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
  /** One merge request, wherever it is (a Chief of Staff change to the workspace repo has no project). */
  mergeRequest(id: string): Promise<MergeRequest>;
  /** What it changes: a unified diff of its branch against main. */
  diff(mergeRequest: string): Promise<string>;
  merge(mergeRequest: string): Promise<void>;
  createPr(mergeRequest: string): Promise<void>;
  reject(mergeRequest: string, reason: string): Promise<void>;
}

export interface WikiArea extends Area {
  tree(project: string): Promise<Node[]>;
  entry(project: string, path: string): Promise<Entry>;
  /** Every board declared in the project's wiki.toml, with its cards. */
  boards(project: string): Promise<Board[]>;
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

// The Chief of Staff's conversation, as the drawer shows it
// (wiki/product/features/chief-of-staff.md#how-the-drawer-is-fed). Typed here,
// not generated: its server side waits on how the Chief of Staff runs
// (wiki/technical-debt.md).

/** A result panel: show_panel(title, rows). */
export type CosPanel = { title: string; rows: [string, string][] };

/** A bubble (say, or the CEO's line), or a panel. */
export type CosItem = { id: number; from: "cos" | "ceo"; text: string; panel?: CosPanel };

export type CosConversation = {
  /** `<harness> · <folder>`. */
  session: string;
  items: CosItem[];
  /** present_choices: chips under the last message. */
  choices: string[];
  /** The Chief of Staff is working on an answer. */
  working: boolean;
};

/** Where the drawer was opened from: each entry point starts its flow. */
export type CosEntry =
  | { flow: "open" }
  | { flow: "hire"; team?: string }
  | { flow: "create-team" }
  | { flow: "refine"; team: string; agent: string }
  | { flow: "project-config"; project: string };

export interface ChiefOfStaffArea extends Area {
  conversation(): Promise<CosConversation>;
  /** Starts an entry point's flow in the same conversation. */
  start(entry: CosEntry): Promise<void>;
  /** The CEO's line, typed or a chip. */
  send(text: string): Promise<void>;
}

export interface EventsArea extends Area {
  /** Follows the events until the returned function is called. */
  subscribe(handlers: EventHandlers): () => void;
}

export interface ForemanClient {
  workspace: WorkspaceArea;
  agents: AgentsArea;
  harnesses: HarnessesArea;
  maintenance: MaintenanceArea;
  forge: ForgeArea;
  requests: RequestsArea;
  messages: MessagesArea;
  pipeline: PipelineArea;
  wiki: WikiArea;
  chiefOfStaff: ChiefOfStaffArea;
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
      const e = (await r.json().catch(() => ({}))) as { error?: string; code?: string };
      throw new ApiError(e.error ?? `${path}: HTTP ${r.status}`, e.code ?? "");
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
      saveSettings: async (settings, version) =>
        (
          await request("PUT", "/api/settings", { settings, version })
        ).json() as Promise<SettingsFile>,
      candidates: () => getJSON("/api/candidates"),
      addProject: async (folder) =>
        (await request("POST", "/api/projects", { folder })).json() as Promise<Project>,
      saveSoul: async (team, agent, soul, version) =>
        (
          await request("PUT", `/api/teams/${id(team)}/agents/${id(agent)}`, { soul, version })
        ).json() as Promise<Team>,
      promptTemplate: async (team, agent) =>
        (await getJSON<{ prompt: string }>(`/api/teams/${id(team)}/agents/${id(agent)}/prompt`))
          .prompt,
    },
    agents: {
      demo: false,
      instances: () => getJSON("/api/instances"),
      deployments: () => getJSON("/api/deployments"),
      prompt: async (instance) =>
        (await getJSON<{ prompt: string }>(`/api/instances/${id(instance)}/prompt`)).prompt,
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
    maintenance: {
      demo: false,
      status: () => getJSON("/api/maintenance"),
      rebuild: () => send("POST", "/api/maintenance/rebuild"),
    },
    forge: {
      demo: false,
      accounts: () => getJSON("/api/secrets/forge"),
      connect: (kind, url, token) => send("PUT", `/api/secrets/forge/${id(kind)}`, { url, token }),
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
      mergeRequest: (mr) => getJSON(`/api/merge-requests/${id(mr)}`),
      diff: async (mr) =>
        (await getJSON<{ diff: string }>(`/api/merge-requests/${id(mr)}/diff`)).diff,
      merge: (mr) => send("POST", `/api/merge-requests/${id(mr)}/merge`),
      createPr: (mr) => send("POST", `/api/merge-requests/${id(mr)}/create-pr`),
      reject: (mr, reason) => send("POST", `/api/merge-requests/${id(mr)}/reject`, { reason }),
    },
    wiki: {
      demo: false,
      tree: (project) => getJSON(`/api/wiki/${id(project)}/tree`),
      entry: (project, path) => getJSON(`/api/wiki/${id(project)}/entry/${path}`),
      boards: (project) => getJSON(`/api/wiki/${id(project)}/boards`),
      setCheckbox: (project, path, line, done, version) =>
        send("PUT", `/api/wiki/${id(project)}/checkbox/${path}`, { line, done, version }),
      moveCard: (project, board, path, column, lane, version) =>
        send("PUT", `/api/wiki/${id(project)}/card/${id(board)}/${path}`, {
          value: column,
          lane,
          version,
        }),
    },
    chiefOfStaff: {
      demo: false,
      conversation: () => Promise.reject(noChiefOfStaff),
      start: () => Promise.reject(noChiefOfStaff),
      send: () => Promise.reject(noChiefOfStaff),
    },
    events: {
      demo: false,
      subscribe: (handlers) => subscribeEvents(`${origin}/api/events`, handlers),
    },
  };
}

const noChiefOfStaff = new Error("The Chief of Staff doesn't run on the server yet");

/** A refused request, with the API's machine code (CONFLICT, INVALID, …). */
export class ApiError extends Error {
  constructor(
    message: string,
    readonly code: string,
  ) {
    super(message);
  }
}

/** Whether a save was refused because the file changed since it was read. */
export function isConflict(e: unknown): boolean {
  return e instanceof ApiError && e.code === "CONFLICT";
}

export const ClientContext = createContext<ForemanClient | null>(null);

export function useClient(): ForemanClient {
  const client = useContext(ClientContext);
  if (!client) throw new Error("useClient outside a ClientContext provider");
  return client;
}
