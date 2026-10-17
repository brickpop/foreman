import { createContext, useContext } from "react";
import { type EventHandlers, subscribeEvents } from "./events";
import { openPty, type PtyConnection, type PtyHandlers } from "./pty";
import type { Account as ForgeAccount } from "./types/forge";
import type { Commit, TreeEntry } from "./types/git";
import type { Status as HarnessStatus } from "./types/harness";
import type { Draft } from "./types/messaging";
import type { LogPage, Repo, Branch as RepoBranch, File as RepoFile } from "./types/repos";
import type { ImageStatus, Maintenance } from "./types/runner";
import type { MachineSize, Step as SetupStep } from "./types/setup";
import type {
  ActionRepo,
  ActivityItem,
  ActivityPage,
  Deployment,
  Instance,
  MergeRequest,
  Request,
  TeamAction,
  TeamMessage,
  WorkItem,
} from "./types/store";
import type { Board, Card, Entry, Node } from "./types/wiki";
import type {
  Candidate,
  Missing,
  Project,
  Settings,
  SettingsFile,
  Soul,
  Team,
} from "./types/workspace";

// Everything the UI asks of the server goes through this interface, grouped
// in areas that match the backend's blocks. Screens never call fetch or
// WebSocket themselves, so the transport can change and tests can hand
// components an in-memory client. main.tsx hands the screens the real one
// (wiki/product/architecture/overview.md#in-the-ui).

export type {
  ActionRepo,
  ActivityItem,
  ActivityPage,
  Board,
  Candidate,
  Card,
  Commit,
  Deployment,
  Draft,
  Entry,
  ForgeAccount,
  HarnessStatus,
  ImageStatus,
  Instance,
  LogPage,
  MachineSize,
  Maintenance,
  MergeRequest,
  Missing,
  Node,
  Project,
  Repo,
  RepoBranch,
  RepoFile,
  Request,
  Settings,
  SettingsFile,
  SetupStep,
  Soul,
  Team,
  TeamAction,
  TeamMessage,
  TreeEntry,
  WorkItem,
};

export type Workspace = { path: string };

export interface WorkspaceArea {
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
  /** Turns a candidate folder into a project: its project repo and starter
   * wiki; with repo, also a first empty code repo of that name. */
  addProject(folder: string, repo?: string): Promise<Project>;
  /** Creates the folder name in the workspace, then makes it a project as
   * addProject does. */
  newProject(name: string, repo?: string): Promise<Project>;
  /** Creates an empty code repo in a project (Foreman never clones). */
  addRepo(project: string, name: string): Promise<Project>;
  /** The scaffold files the project repo lacks (wiki/wiki.toml…), and creating them. */
  missing(project: string): Promise<Missing[]>;
  scaffold(project: string): Promise<Missing[]>;
  /** Saves one agent's soul in its team file, guarded by the version read. */
  saveSoul(team: string, agent: string, soul: Soul, version: string): Promise<Team>;
  /** An agent's prompt before any deployment, with {placeholders}. */
  promptTemplate(team: string, agent: string): Promise<string>;
}

export interface AgentsArea {
  instances(): Promise<Instance[]>;
  /** The active deployments. */
  deployments(): Promise<Deployment[]>;
  /** The stopped deployments (a project's Activity tab lists their older sessions). */
  stoppedDeployments(): Promise<Deployment[]>;
  /** An instance's session prompt, exactly as composed for its harness. */
  prompt(instance: string): Promise<string>;
  deploy(team: string, project: string, goal: string): Promise<Deployment>;
  stop(deployment: string): Promise<void>;
  /** Starts an asleep instance; any other is left as it is. */
  wake(instance: string): Promise<void>;
  /** Types the CEO's line into the agent's session and sends it (the same
   * typing as a delivery, so the harness takes the Enter as sending). */
  line(instance: string, text: string): Promise<void>;
  /** Pauses or resumes the countdown of messages waiting on the CEO's draft. */
  pauseCountdown(instance: string, paused: boolean): Promise<void>;
  openTerminal(id: string, handlers: PtyHandlers): PtyConnection;
}

export interface HarnessesArea {
  list(): Promise<HarnessStatus[]>;
  /** Starts the harness's login; returns the terminal to open with agents.openTerminal. */
  connect(harness: string): Promise<string>;
  /** Ends the harness's login, if one runs. */
  endLogin(harness: string): Promise<void>;
}

export interface MaintenanceArea {
  /** The engine's state and its images. */
  status(): Promise<Maintenance>;
  /** Starts rebuilding the images without cache; status shows the progress. */
  rebuild(): Promise<void>;
}

/** The setup checklist on Home (first-run.md). */
export interface SetupArea {
  steps(): Promise<SetupStep[]>;
  /** Starts the stopped Podman machine; the steps follow its progress. */
  startPodman(): Promise<void>;
  /** Creates the Podman machine (macOS), memory in GiB. */
  createMachine(size: { cpus: number; memory: number }): Promise<void>;
}

export interface ForgeArea {
  accounts(): Promise<ForgeAccount[]>;
  /** Checks the token against the forge, then stores it with the host. */
  connect(kind: string, url: string, token: string): Promise<void>;
}

export interface RequestsArea {
  list(): Promise<Request[]>;
  resolve(id: string, answer: string): Promise<void>;
}

/** Whose activity: a project's active deployments, or one deployment. */
export type ActivityScope = { project: string } | { deployment: string };

export interface MessagesArea {
  list(deployment: string): Promise<TeamMessage[]>;
  /** The git tools the deployment's agents called, oldest first. */
  actions(deployment: string): Promise<TeamAction[]>;
  /** A page of team traffic, messages and git actions together, newest first:
   * a project's active deployments, or one deployment; before is the previous
   * page's next. */
  activity(scope: ActivityScope, before?: string): Promise<ActivityPage>;
}

export type Pipeline = { workItems: WorkItem[]; mergeRequests: MergeRequest[] };

export interface PipelineArea {
  list(project: string): Promise<Pipeline>;
  /** One merge request, wherever it is (a Chief of Staff change to the workspace repo has no project). */
  mergeRequest(id: string): Promise<MergeRequest>;
  /** What it changes: a unified diff of its branch against main. */
  diff(mergeRequest: string): Promise<string>;
  merge(mergeRequest: string): Promise<void>;
  createPr(mergeRequest: string): Promise<void>;
  reject(mergeRequest: string, reason: string): Promise<void>;
}

export interface WikiArea {
  tree(project: string): Promise<Node[]>;
  entry(project: string, path: string): Promise<Entry>;
  /** Where the browser loads an image of the wiki from (only one an entry links to is served). */
  imageUrl(project: string, path: string): string;
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

export interface ChiefOfStaffArea {
  conversation(): Promise<CosConversation>;
  /** Starts an entry point's flow in the same conversation. */
  start(entry: CosEntry): Promise<void>;
  /** The CEO's line, typed or a chip. */
  send(text: string): Promise<void>;
}

export interface EventsArea {
  /** Follows the events until the returned function is called. */
  subscribe(handlers: EventHandlers): () => void;
}

/** A project's repos, read-only, as committed (the Repositories tab). */
export interface ReposArea {
  /** The code repos, then the project repo, each with its branches. */
  list(project: string): Promise<Repo[]>;
  /** A folder at a ref ("" is the root). */
  tree(project: string, repo: string, ref: string, path: string): Promise<TreeEntry[]>;
  file(project: string, repo: string, ref: string, path: string): Promise<RepoFile>;
  /** A page of a ref's history, newest first; skip is the previous page's next. */
  log(project: string, repo: string, ref: string, skip?: number): Promise<LogPage>;
}

export interface ForemanClient {
  workspace: WorkspaceArea;
  agents: AgentsArea;
  harnesses: HarnessesArea;
  maintenance: MaintenanceArea;
  setup: SetupArea;
  forge: ForgeArea;
  requests: RequestsArea;
  messages: MessagesArea;
  pipeline: PipelineArea;
  repos: ReposArea;
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
  // A wiki path keeps its slashes: each segment is escaped on its own.
  const wikiPath = (path: string) => path.split("/").map(id).join("/");
  return {
    workspace: {
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
      addProject: async (folder, repo) =>
        (await request("POST", "/api/projects", { folder, repo })).json() as Promise<Project>,
      newProject: async (name, repo) =>
        (
          await request("POST", "/api/projects", { folder: name, repo, new: true })
        ).json() as Promise<Project>,
      addRepo: async (project, name) =>
        (
          await request("POST", `/api/projects/${id(project)}/repos`, { name })
        ).json() as Promise<Project>,
      missing: (project) => getJSON(`/api/projects/${id(project)}/scaffold`),
      scaffold: async (project) =>
        (await request("POST", `/api/projects/${id(project)}/scaffold`)).json() as Promise<
          Missing[]
        >,
      saveSoul: async (team, agent, soul, version) =>
        (
          await request("PUT", `/api/teams/${id(team)}/agents/${id(agent)}`, { soul, version })
        ).json() as Promise<Team>,
      promptTemplate: async (team, agent) =>
        (await getJSON<{ prompt: string }>(`/api/teams/${id(team)}/agents/${id(agent)}/prompt`))
          .prompt,
    },
    agents: {
      instances: () => getJSON("/api/instances"),
      deployments: () => getJSON("/api/deployments"),
      stoppedDeployments: () => getJSON("/api/deployments?status=stopped"),
      prompt: async (instance) =>
        (await getJSON<{ prompt: string }>(`/api/instances/${id(instance)}/prompt`)).prompt,
      deploy: async (team, project, goal) =>
        (
          await request("POST", "/api/deployments", { team, project, goal })
        ).json() as Promise<Deployment>,
      stop: (deployment) => send("DELETE", `/api/deployments/${id(deployment)}`),
      wake: (instance) => send("POST", `/api/instances/${id(instance)}/wake`),
      line: (instance, text) => send("POST", `/api/instances/${id(instance)}/line`, { text }),
      pauseCountdown: (instance, paused) =>
        send("PUT", `/api/instances/${id(instance)}/countdown`, { paused }),
      openTerminal: (instance, handlers) => openPty(`${wsOrigin}/ws/pty/${id(instance)}`, handlers),
    },
    harnesses: {
      list: () => getJSON("/api/harnesses"),
      connect: async (harness) => {
        const r = await request("POST", `/api/harnesses/${id(harness)}/connect`);
        return ((await r.json()) as { terminal: string }).terminal;
      },
      endLogin: (harness) => send("DELETE", `/api/harnesses/${id(harness)}/connect`),
    },
    maintenance: {
      status: () => getJSON("/api/maintenance"),
      rebuild: () => send("POST", "/api/maintenance/rebuild"),
    },
    setup: {
      steps: () => getJSON("/api/setup"),
      startPodman: () => send("POST", "/api/setup/podman/start"),
      createMachine: (size) => send("POST", "/api/setup/podman/machine", size),
    },
    wiki: {
      tree: (project) => getJSON(`/api/wiki/${id(project)}/tree`),
      entry: (project, path) => getJSON(`/api/wiki/${id(project)}/entry/${wikiPath(path)}`),
      imageUrl: (project, path) => `${origin}/api/wiki/${id(project)}/image/${wikiPath(path)}`,
      boards: (project) => getJSON(`/api/wiki/${id(project)}/boards`),
      setCheckbox: (project, path, line, done, version) =>
        send("PUT", `/api/wiki/${id(project)}/checkbox/${wikiPath(path)}`, { line, done, version }),
      moveCard: (project, board, path, column, lane, version) =>
        send("PUT", `/api/wiki/${id(project)}/card/${id(board)}/${wikiPath(path)}`, {
          value: column,
          lane,
          version,
        }),
    },
    pipeline: {
      list: (project) => getJSON(`/api/pipeline?project=${id(project)}`),
      mergeRequest: (mergeRequest) => getJSON(`/api/merge-requests/${id(mergeRequest)}`),
      diff: async (mergeRequest) =>
        (await getJSON<{ diff: string }>(`/api/merge-requests/${id(mergeRequest)}/diff`)).diff,
      merge: (mergeRequest) => send("POST", `/api/merge-requests/${id(mergeRequest)}/merge`),
      // Create PR comes with the Git epic's git-forge card.
      createPr: notBuilt("pull requests"),
      reject: (mergeRequest, reason) =>
        send("POST", `/api/merge-requests/${id(mergeRequest)}/reject`, { reason }),
    },
    requests: {
      list: () => getJSON("/api/requests"),
      // Answering comes with escalate (the Workflow epic): today's requests are
      // reviews, decided on their merge request.
      resolve: notBuilt("answering requests"),
    },
    // The areas below wait on their server side: each screen says "Not built yet".
    forge: {
      accounts: notBuilt("forge accounts"),
      connect: notBuilt("forge accounts"),
    },
    repos: {
      list: (project) => getJSON(`/api/projects/${id(project)}/repos`),
      tree: (project, repo, ref, path) =>
        getJSON(
          `/api/projects/${id(project)}/repos/${id(repo)}/tree?${new URLSearchParams({ ref, path })}`,
        ),
      file: (project, repo, ref, path) =>
        getJSON(
          `/api/projects/${id(project)}/repos/${id(repo)}/file?${new URLSearchParams({ ref, path })}`,
        ),
      log: (project, repo, ref, skip = 0) =>
        getJSON(
          `/api/projects/${id(project)}/repos/${id(repo)}/log?${new URLSearchParams({ ref, skip: String(skip) })}`,
        ),
    },
    messages: {
      list: (deployment) => getJSON(`/api/messages?deployment=${id(deployment)}`),
      actions: (deployment) => getJSON(`/api/actions?deployment=${id(deployment)}`),
      activity: (scope, before) => {
        const q = new URLSearchParams(
          "project" in scope ? { project: scope.project } : { deployment: scope.deployment },
        );
        if (before) q.set("before", before);
        return getJSON(`/api/activity?${q}`);
      },
    },
    chiefOfStaff: {
      conversation: notBuilt("the Chief of Staff"),
      start: notBuilt("the Chief of Staff"),
      send: notBuilt("the Chief of Staff"),
    },
    events: {
      subscribe: (handlers) => subscribeEvents(`${origin}/api/events`, handlers),
    },
  };
}

/** An area the server doesn't serve yet: every call is refused with NOT_BUILT. */
const notBuilt = (what: string) => (): Promise<never> =>
  Promise.reject(new ApiError(`The server doesn't serve ${what} yet`, "NOT_BUILT"));

/** Whether a failure is an area that isn't built yet, which a screen shows as "Not built yet". */
export const isNotBuilt = (e: unknown): boolean => e instanceof ApiError && e.code === "NOT_BUILT";

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
