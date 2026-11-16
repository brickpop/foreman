import { createContext, useContext } from "react";
import { type EventHandlers, subscribeEvents } from "./events";
import { openPty, type PtyConnection, type PtyHandlers } from "./pty";
import type { Proposal } from "./types/cos";
import type { Commit, TreeEntry } from "./types/git";
import type { Endpoint, EndpointChange, Status as HarnessStatus } from "./types/harness";
import type {
  Backup,
  Branches,
  FilesInbox,
  FilesShared,
  Folder,
  FolderEntry,
  Instructions,
  Missing,
  Project,
  Settings,
  SettingsFile,
  Soul,
  Team,
} from "./types/home";
import type { Draft } from "./types/messaging";
import type { Profile } from "./types/profiles";
import type {
  LogPage,
  Repo,
  Branch as RepoBranch,
  File as RepoFile,
  Link as RepoLink,
} from "./types/repos";
import type { ImageStatus, Maintenance } from "./types/runner";
import type { MachineSize, Step as SetupStep } from "./types/setup";
import type {
  ActionRepo,
  ActivityItem,
  ActivityPage,
  Deployment,
  Doing,
  Instance,
  MergeRequest,
  Request,
  TeamAction,
  TeamMessage,
  WorkItem,
} from "./types/store";
import type { Board, Card, Entry, Field, Node, Unread } from "./types/wiki";
import type {
  BranchPick,
  Cleaned,
  Cleanup,
  CleanupBranch,
  Dismiss,
  Dismissed,
  RepoDiff,
  Review,
  Comment as ReviewComment,
} from "./types/workflow";

// Everything the UI asks of the server goes through this interface, grouped
// in areas that match the backend's blocks. Screens never call fetch or
// WebSocket themselves, so the transport can change and tests can hand
// components an in-memory client. main.tsx hands the screens the real one
// (wiki/product/architecture/overview.md#in-the-ui).

export type {
  ActionRepo,
  ActivityItem,
  ActivityPage,
  Backup,
  Board,
  Branches,
  BranchPick,
  Card,
  Cleaned,
  Cleanup,
  CleanupBranch,
  Commit,
  Deployment,
  Dismiss,
  Dismissed,
  Doing,
  Draft,
  Entry,
  Field,
  Folder,
  FolderEntry,
  HarnessStatus,
  ImageStatus,
  Instance,
  Instructions,
  LogPage,
  MachineSize,
  Maintenance,
  MergeRequest,
  Missing,
  Node,
  Profile,
  Project,
  Proposal,
  Repo,
  RepoBranch,
  RepoDiff,
  RepoFile,
  RepoLink,
  Request,
  Review,
  ReviewComment,
  Settings,
  SettingsFile,
  SetupStep,
  Soul,
  Team,
  TeamAction,
  TeamMessage,
  TreeEntry,
  Unread,
  WorkItem,
};

/** Foreman's home on this machine (~/.foreman: its machinery) and its folder
 * (~/foreman: what the CEO owns). */
/** GET /api/home: Foreman's home and folder, what is wrong with
 * projects.toml (the projects stay as last read until it is fixed), and the
 * HQs their origin doesn't back up. */
export type HomeFolder = {
  path: string;
  folder: string;
  projectsProblems: string[];
  backups: Backup[];
};

export interface HomeArea {
  folder(): Promise<HomeFolder>;
  /** Replaces Foreman's HQ, while still as its first start made it, with a
   * clone of url: another machine's. */
  restore(url: string): Promise<void>;
  projects(): Promise<Project[]>;
  project(id: string): Promise<Project>;
  teams(): Promise<Team[]>;
  team(id: string): Promise<Team>;
  settings(): Promise<SettingsFile>;
  /** Saves settings.toml, guarded by the version read. */
  saveSettings(settings: Settings, version: string): Promise<SettingsFile>;
  /** The CEO's instructions for every agent: instructions.md in Foreman's HQ. */
  instructions(): Promise<Instructions>;
  /** Saves them, guarded by the version read: one commit in Foreman's HQ. */
  saveInstructions(text: string, version: string): Promise<Instructions>;
  /** Creates a project and its HQ: new (a starter wiki), or cloned from
   * clone (a URL or a local path, with the CEO's own git). created is false
   * when it took over an HQ a removed project left there. */
  newProject(name: string, clone?: string): Promise<{ project: Project; created: boolean }>;
  /** Clones the HQ of a project not on this machine from the origin its
   * record keeps; its code repos are then located or cloned on Repos. */
  bringHere(project: string): Promise<Project>;
  /** Removes a project and its history, keeping its folder: its HQ too when
   * deleteHQ, its agents' worktrees when deleteWorktrees. */
  removeProject(
    project: string,
    opts: { deleteHQ: boolean; deleteWorktrees: boolean },
  ): Promise<void>;
  /** Adds an existing repo by its folder, with that default branch (none:
   * main, which it must have); newRepo creates an empty one at a new folder,
   * on main. */
  addRepo(project: string, name: string, path: string, defaultBranch?: string): Promise<Project>;
  newRepo(project: string, name: string, path: string): Promise<Project>;
  /** Adds a repo cloned from remote into Foreman's repos/<its URL's path>
   * (or the clone already there); cloneMissingRepo locates one by cloning
   * the remote its record keeps. */
  cloneRepo(
    project: string,
    name: string,
    remote: string,
    defaultBranch?: string,
  ): Promise<Project>;
  cloneMissingRepo(project: string, name: string): Promise<Project>;
  /** Sets a repo's default branch, one it has. */
  setRepoDefault(project: string, name: string, branch: string): Promise<Project>;
  /** Points a missing repo at its new folder (the same repo). */
  locateRepo(project: string, name: string, path: string): Promise<Project>;
  renameRepo(project: string, name: string, newName: string): Promise<Project>;
  removeRepo(project: string, name: string): Promise<Project>;
  /** A folder for picking repos, inside the CEO's home folder; none: the
   * folder Foreman was started in. With a project, which of its repos the
   * folder already is. */
  folders(path?: string, project?: string): Promise<Folder>;
  /** A repo folder's branches, to pick its default branch. */
  branches(path: string): Promise<Branches>;
  /** Sets the profile a project's agents run on, from their next start. */
  setProfile(project: string, profile: string): Promise<Project>;
  /** Gives a project its display name ("": shown by its id). Its id stays. */
  setName(project: string, name: string): Promise<Project>;
  /** Hides a project (out of the sidebar's projects and the Dashboard), or shows it again. */
  setHidden(project: string, hidden: boolean): Promise<Project>;
  /** The scaffold files the HQ lacks (wiki.toml…), and creating them. */
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
  /** Deploys team on project; fresh starts every agent on a new conversation. */
  deploy(team: string, project: string, fresh?: boolean): Promise<Deployment>;
  /** The team's agents that would continue an earlier conversation on project. */
  continuing(team: string, project: string): Promise<string[]>;
  stop(deployment: string): Promise<void>;
  /** Starts an asleep instance; any other is left as it is. */
  wake(instance: string): Promise<void>;
  /** Stops a running instance and starts it again, on its project's profile
   * as it is now; any other is left as it is. */
  restart(instance: string): Promise<void>;
  /** Types the CEO's line into the agent's session and sends it (the same
   * typing as a delivery, so the harness takes the Enter as sending). */
  line(instance: string, text: string): Promise<void>;
  /** Pauses or resumes the countdown of messages waiting on the CEO's draft. */
  pauseCountdown(instance: string, paused: boolean): Promise<void>;
  /** Releases messages held on a draft, marking it unfinished. */
  sendNow(instance: string): Promise<void>;
  openTerminal(id: string, handlers: PtyHandlers): PtyConnection;
}

export interface HarnessesArea {
  /** The supported harnesses' facts (the keystroke for a new line). */
  list(): Promise<HarnessStatus[]>;
}

/** The CEO's profiles: each a harness and its credentials, on this machine
 * (wiki/product/features/settings.md#profiles). */
export interface ProfilesArea {
  /** With their state, the default first. */
  list(): Promise<Profile[]>;
  create(name: string, harness: string): Promise<Profile>;
  /** Deletes it and its credentials; never the default. */
  remove(name: string): Promise<void>;
  /** Switches it to another harness: the old one's credentials go. */
  setHarness(name: string, harness: string): Promise<Profile>;
  /** Starts its harness's login; returns the terminal to open with agents.openTerminal. */
  connect(name: string): Promise<string>;
  /** Ends its login, if one runs. */
  endLogin(name: string): Promise<void>;
  /** A Pi profile's model server and model (the key is never sent back). */
  endpoint(name: string): Promise<Endpoint>;
  /** Saves them; an empty apiKey keeps the one set. */
  setEndpoint(name: string, change: EndpointChange): Promise<void>;
  /** The models a server offers; with no apiKey, the profile's. */
  models(name: string, url: string, apiKey: string): Promise<string[]>;
}

export interface MaintenanceArea {
  /** The engine's state and its images. */
  status(): Promise<Maintenance>;
  /** Starts rebuilding the images without cache; status shows the progress. */
  rebuild(): Promise<void>;
}

/** The setup checklist on the Dashboard (first-run.md). */
export interface SetupArea {
  steps(): Promise<SetupStep[]>;
  /** Starts the stopped Podman machine; the steps follow its progress. */
  startPodman(): Promise<void>;
  /** Creates the Podman machine (macOS), memory in GiB. */
  createMachine(size: { cpus: number; memory: number }): Promise<void>;
}

export interface RequestsArea {
  list(): Promise<Request[]>;
  resolve(id: string, answer: string): Promise<void>;
  /** Closes it unanswered; an agent's escalation tells its lead so. */
  dismiss(id: string): Promise<void>;
}

export interface MessagesArea {
  list(deployment: string): Promise<TeamMessage[]>;
  /** A page of a deployment's team traffic, messages and git actions
   * together, newest first; before is the previous page's next. */
  activity(deployment: string, before?: string): Promise<ActivityPage>;
}

export type Pipeline = { workItems: WorkItem[]; mergeRequests: MergeRequest[] };

export interface PipelineArea {
  list(project: string): Promise<Pipeline>;
  /** Removes what agents no longer running left on the project, only what
   * is safe; says what it removed and what it kept, with why. */
  /** uncommitted: worktrees holding changes not committed go too, the changes kept aside. */
  cleanUp(project: string, uncommitted: boolean): Promise<Cleanup>;
  /** What the branch cleanup offers in the project's code repos: the agents'
   * branches and legacy ones, and the CEO's merged or superseded ones, each
   * with its repo, where its work stands and what keeps it. */
  cleanupBranches(project: string): Promise<CleanupBranch[]>;
  /** Deletes the CEO's pick of them, each checked again; those that stay, stay. */
  deleteBranches(project: string, branches: BranchPick[]): Promise<Cleanup>;
}

/** A comment the CEO writes: on a line of a repo's diff (its side, line, and
 * the head it was read at), or general (no repo). */
export type NewComment = {
  repo: string;
  path: string;
  side: "" | "new" | "old";
  line: number;
  commit: string;
  snippet: string;
  body: string;
};

/** The CEO's reviews of the leads' branches (wiki/product/features/reviews.md). */
export interface ReviewsArea {
  /** Every review, newest first, without their comments. */
  /** A project's open reviews and its last decided ones, newest first. */
  list(project: string): Promise<Review[]>;
  get(review: string): Promise<Review>;
  /** Each repo's change: against its target, or since the CEO last requested changes. */
  diff(review: string, since: boolean): Promise<RepoDiff[]>;
  addComment(review: string, c: NewComment): Promise<void>;
  editComment(review: string, comment: string, body: string): Promise<void>;
  deleteComment(review: string, comment: string): Promise<void>;
  /** Sends the draft comments and the note to the lead. */
  requestChanges(review: string, note: string): Promise<void>;
  /** Merges the repos named, each at the head the CEO read (repo → commit);
   * the rest stay open. A repo whose branch moved since isn't merged. landing: the CEO's name
   * for the branch they land on (made from each target, or one at its head); else the lead's,
   * else the target. */
  merge(review: string, heads: Record<string, string>, landing?: string): Promise<void>;
  /** Ends it unmerged, the reason going to the lead. */
  /** Dismisses it as the CEO chose; answers with the branches it couldn't drop. */
  dismiss(review: string, c: Dismiss): Promise<Dismissed>;
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

/** The Chief of Staff's session, with its page (wiki/product/features/chief-of-staff.md). */
export interface ChiefOfStaffArea {
  /** Starts it unless it runs, typing the opener into it ("" for none). */
  start(opener: string): Promise<{ terminal: string; proposal: Proposal | null }>;
  /** Ends it: the CEO left the page. Sent even as the page unloads. */
  stop(): Promise<void>;
  /** Writes the proposal on screen, with the faces shown for members proposed without one. */
  create(id: number): Promise<Team>;
}

export interface EventsArea {
  /** Follows the events until the returned function is called. */
  subscribe(handlers: EventHandlers): () => void;
}

/** A project's repos, read-only, as committed (the Repos section). */
export interface ReposArea {
  /** The code repos, then the HQ, each with its branches. */
  list(project: string): Promise<Repo[]>;
  /** A folder at a ref ("" is the root). */
  tree(project: string, repo: string, ref: string, path: string): Promise<TreeEntry[]>;
  file(project: string, repo: string, ref: string, path: string): Promise<RepoFile>;
  /** Where an image of the repo at a ref loads from (an <img> src). */
  imageUrl(project: string, repo: string, ref: string, path: string): string;
  /** A page of a ref's history, or with all every branch's, newest first;
   * skip is the previous page's next. */
  log(project: string, repo: string, ref: string, skip?: number, all?: boolean): Promise<LogPage>;
}

/** A folder to open on the desktop: an agent's (its worktrees), or one of a
 * project's file folders. The server works out its path: the page never
 * names one. */
export type OpenTarget =
  | { instance: string }
  | { project: string; folder: typeof FilesInbox | typeof FilesShared };

/** Opening folders on the desktop of Foreman's machine, which is the CEO's. */
export interface DesktopArea {
  /** Whether this machine can open folders: without, no Open button shows. */
  canOpen(): Promise<{ canOpen: boolean }>;
  open(target: OpenTarget): Promise<void>;
}

export interface ForemanClient {
  home: HomeArea;
  agents: AgentsArea;
  harnesses: HarnessesArea;
  profiles: ProfilesArea;
  maintenance: MaintenanceArea;
  setup: SetupArea;
  requests: RequestsArea;
  messages: MessagesArea;
  pipeline: PipelineArea;
  reviews: ReviewsArea;
  repos: ReposArea;
  wiki: WikiArea;
  chiefOfStaff: ChiefOfStaffArea;
  desktop: DesktopArea;
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
    home: {
      folder: () => getJSON("/api/home"),
      restore: (url) => send("POST", "/api/home/restore", { url }),
      projects: () => getJSON("/api/projects"),
      project: (project) => getJSON(`/api/projects/${id(project)}`),
      teams: () => getJSON("/api/teams"),
      team: (team) => getJSON(`/api/teams/${id(team)}`),
      settings: () => getJSON("/api/settings"),
      saveSettings: async (settings, version) =>
        (
          await request("PUT", "/api/settings", { settings, version })
        ).json() as Promise<SettingsFile>,
      instructions: () => getJSON("/api/instructions"),
      saveInstructions: async (text, version) =>
        (
          await request("PUT", "/api/instructions", { text, version })
        ).json() as Promise<Instructions>,
      newProject: async (name, clone) => {
        const r = await request("POST", "/api/projects", { name, clone });
        // 201: a new HQ; 200: one taken over.
        return { project: (await r.json()) as Project, created: r.status === 201 };
      },
      bringHere: async (project) =>
        (await request("POST", `/api/projects/${id(project)}/bring`)).json() as Promise<Project>,
      removeProject: (project, { deleteHQ, deleteWorktrees }) => {
        const q = new URLSearchParams();
        if (deleteHQ) q.set("deleteHQ", "true");
        if (deleteWorktrees) q.set("deleteWorktrees", "true");
        return send("DELETE", `/api/projects/${id(project)}${q.size ? `?${q}` : ""}`);
      },
      addRepo: async (project, name, path, defaultBranch) =>
        (
          await request("POST", `/api/projects/${id(project)}/repos`, { name, path, defaultBranch })
        ).json() as Promise<Project>,
      newRepo: async (project, name, path) =>
        (
          await request("POST", `/api/projects/${id(project)}/repos`, { name, path, new: true })
        ).json() as Promise<Project>,
      cloneRepo: async (project, name, remote, defaultBranch) =>
        (
          await request("POST", `/api/projects/${id(project)}/repos`, {
            name,
            clone: remote,
            defaultBranch,
          })
        ).json() as Promise<Project>,
      cloneMissingRepo: async (project, name) =>
        (
          await request("PUT", `/api/projects/${id(project)}/repos/${id(name)}`, { clone: true })
        ).json() as Promise<Project>,
      setRepoDefault: async (project, name, branch) =>
        (
          await request("PUT", `/api/projects/${id(project)}/repos/${id(name)}/default`, { branch })
        ).json() as Promise<Project>,
      locateRepo: async (project, name, path) =>
        (
          await request("PUT", `/api/projects/${id(project)}/repos/${id(name)}`, { path })
        ).json() as Promise<Project>,
      renameRepo: async (project, name, newName) =>
        (
          await request("PUT", `/api/projects/${id(project)}/repos/${id(name)}/name`, {
            name: newName,
          })
        ).json() as Promise<Project>,
      removeRepo: async (project, name) =>
        (
          await request("DELETE", `/api/projects/${id(project)}/repos/${id(name)}`)
        ).json() as Promise<Project>,
      folders: (path, project) => {
        const q = new URLSearchParams();
        if (path) q.set("path", path);
        if (project) q.set("project", project);
        return getJSON(`/api/folders${q.size ? `?${q}` : ""}`);
      },
      branches: (path) => getJSON(`/api/folders/branches?${new URLSearchParams({ path })}`),
      setProfile: async (project, profile) =>
        (
          await request("PUT", `/api/projects/${id(project)}/profile`, { profile })
        ).json() as Promise<Project>,
      setName: async (project, name) =>
        (
          await request("PUT", `/api/projects/${id(project)}/name`, { name })
        ).json() as Promise<Project>,
      setHidden: async (project, hidden) =>
        (
          await request("PUT", `/api/projects/${id(project)}/hidden`, { hidden })
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
      deploy: async (team, project, fresh = false) =>
        (
          await request("POST", "/api/deployments", { team, project, fresh })
        ).json() as Promise<Deployment>,
      continuing: async (team, project) =>
        (
          await getJSON<{ agents: string[] }>(
            `/api/deployments/continuing?${new URLSearchParams({ team, project })}`,
          )
        ).agents,
      stop: (deployment) => send("DELETE", `/api/deployments/${id(deployment)}`),
      wake: (instance) => send("POST", `/api/instances/${id(instance)}/wake`),
      restart: (instance) => send("POST", `/api/instances/${id(instance)}/restart`),
      line: (instance, text) => send("POST", `/api/instances/${id(instance)}/line`, { text }),
      sendNow: (instance) => send("POST", `/api/instances/${id(instance)}/send-now`),
      pauseCountdown: (instance, paused) =>
        send("PUT", `/api/instances/${id(instance)}/countdown`, { paused }),
      openTerminal: (instance, handlers) => openPty(`${wsOrigin}/ws/pty/${id(instance)}`, handlers),
    },
    harnesses: {
      list: () => getJSON("/api/harnesses"),
    },
    profiles: {
      list: () => getJSON("/api/profiles"),
      create: async (name, harness) =>
        (await request("POST", "/api/profiles", { name, harness })).json(),
      remove: (name) => send("DELETE", `/api/profiles/${id(name)}`),
      setHarness: async (name, harness) =>
        (await request("PUT", `/api/profiles/${id(name)}/harness`, { harness })).json(),
      connect: async (name) => {
        const r = await request("POST", `/api/profiles/${id(name)}/connect`);
        return ((await r.json()) as { terminal: string }).terminal;
      },
      endLogin: (name) => send("DELETE", `/api/profiles/${id(name)}/connect`),
      endpoint: (name) => getJSON(`/api/profiles/${id(name)}/endpoint`),
      setEndpoint: (name, change) => send("PUT", `/api/profiles/${id(name)}/endpoint`, change),
      models: async (name, url, apiKey) =>
        (
          await request("POST", `/api/profiles/${id(name)}/endpoint/models`, { url, apiKey })
        ).json(),
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
      cleanUp: async (project, uncommitted) =>
        (
          await request("POST", `/api/projects/${id(project)}/cleanup`, { uncommitted })
        ).json() as Promise<Cleanup>,
      cleanupBranches: (project) => getJSON(`/api/projects/${id(project)}/branch-cleanup`),
      deleteBranches: async (project, branches) =>
        (
          await request("POST", `/api/projects/${id(project)}/branch-cleanup`, { branches })
        ).json() as Promise<Cleanup>,
    },
    reviews: {
      list: (project) => getJSON(`/api/reviews?project=${id(project)}`),
      get: (review) => getJSON(`/api/reviews/${id(review)}`),
      diff: (review, since) => getJSON(`/api/reviews/${id(review)}/diff${since ? "?since=1" : ""}`),
      addComment: (review, c) => send("POST", `/api/reviews/${id(review)}/comments`, c),
      editComment: (review, comment, body) =>
        send("PUT", `/api/reviews/${id(review)}/comments/${id(comment)}`, { body }),
      deleteComment: (review, comment) =>
        send("DELETE", `/api/reviews/${id(review)}/comments/${id(comment)}`),
      requestChanges: (review, note) =>
        send("POST", `/api/reviews/${id(review)}/changes`, { note }),
      merge: (review, heads, landing) =>
        send("POST", `/api/reviews/${id(review)}/merge`, { heads, landing }),
      dismiss: async (review, c) =>
        (
          await request("POST", `/api/reviews/${id(review)}/dismiss`, c)
        ).json() as Promise<Dismissed>,
    },
    requests: {
      list: () => getJSON("/api/requests"),
      // An agent's escalation; a review is decided on its page, a stuck agent restarted.
      resolve: (request, answer) =>
        send("POST", `/api/requests/${id(request)}/resolve`, { answer }),
      dismiss: (request) => send("POST", `/api/requests/${id(request)}/dismiss`),
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
      imageUrl: (project, repo, ref, path) =>
        `${origin}/api/projects/${id(project)}/repos/${id(repo)}/image?${new URLSearchParams({ ref, path })}`,
      log: (project, repo, ref, skip = 0, all = false) => {
        const q = new URLSearchParams({ ref, skip: String(skip) });
        if (all) q.set("all", "1");
        return getJSON(`/api/projects/${id(project)}/repos/${id(repo)}/log?${q}`);
      },
    },
    messages: {
      list: (deployment) => getJSON(`/api/messages?deployment=${id(deployment)}`),
      activity: (deployment, before) => {
        const q = new URLSearchParams({ deployment });
        if (before) q.set("before", before);
        return getJSON(`/api/activity?${q}`);
      },
    },
    chiefOfStaff: {
      start: async (opener) => (await request("POST", "/api/cos", { opener })).json(),
      // keepalive: the request survives the page being closed.
      stop: async () => {
        await fetch(`${origin}/api/cos`, { method: "DELETE", keepalive: true });
      },
      create: async (id) => (await request("POST", "/api/cos/proposal", { id })).json(),
    },
    desktop: {
      canOpen: () => getJSON("/api/desktop"),
      open: (target) => send("POST", "/api/desktop/open", target),
    },
    events: {
      subscribe: (handlers) => subscribeEvents(`${origin}/api/events`, handlers),
    },
  };
}

/** An area the server doesn't serve yet: every call is refused with NOT_BUILT. */
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
