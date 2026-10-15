import type {
  Deployment,
  ForemanClient,
  Instance,
  MergeRequest,
  Request,
  TeamMessage,
} from "../client";
import type { EventHandlers } from "../events";
import type { PtyConnection, PtyHandlers } from "../pty";
import type { Event } from "../types/events";
import type { Seed } from "./seed";

// The demo client (wiki/backlog/demo-client.md): the prototype's data in
// memory, behind the same interface as the real client. Writes change that
// memory and announce themselves as events, like the server would; a timer
// simulates the team at work. Every area says demo: true, for the badge.

/** How a demo agent's terminal is shown: by default, a notice; the composition
 * root may pass the real server's demo session instead. */
export type DemoTerminal = (id: string, handlers: PtyHandlers) => PtyConnection;

export type DemoOptions = {
  terminal?: DemoTerminal;
  /** Milliseconds between simulated events; 0 turns the simulation off. */
  tick?: number;
  /** The clock, for tests. */
  now?: () => Date;
};

class Conflict extends Error {
  constructor(what: string) {
    super(`${what} changed since you opened it`);
  }
}

class NotFound extends Error {
  constructor(what: string) {
    super(`no such ${what}`);
  }
}

export function demoClient(seed: Seed, options: DemoOptions = {}): ForemanClient {
  const state: Seed = structuredClone(seed);
  const now = options.now ?? (() => new Date());
  const iso = () => now().toISOString();
  const handlers = new Set<EventHandlers>();
  const emit = (type: string, data: unknown) => {
    const e: Event = { type, data };
    for (const h of handlers) h.event(e);
  };
  const reply = <T>(value: T): Promise<T> => Promise.resolve(structuredClone(value));
  const find = <T>(list: T[], match: (t: T) => boolean, what: string): T => {
    const found = list.find(match);
    if (!found) throw new NotFound(what);
    return found;
  };
  const version = () => Math.random().toString(16).slice(2, 10);
  const setStatus = (inst: Instance, status: string) => {
    inst.status = status;
    emit("instance.status", { id: inst.id, status });
  };

  let timer: ReturnType<typeof setInterval> | undefined;
  let step = 0;
  const simulate = () => simulateTeam(state, setStatus, emit, iso, step++);

  return {
    workspace: {
      demo: true,
      workspace: () => reply(state.workspace),
      projects: () => reply(state.projects),
      project: async (id) => reply(find(state.projects, (p) => p.id === id, "project")),
      teams: () => reply(state.teams),
      team: async (id) => reply(find(state.teams, (t) => t.id === id, "team")),
      settings: () => reply(state.settings),
      candidates: () => reply(state.candidates),
      addProject: async (folder) => {
        const candidate = find(state.candidates, (c) => c.folder === folder, "folder");
        const project = newProject(folder, candidate.repos);
        state.projects = [...state.projects, project];
        state.candidates = state.candidates.filter((c) => c.folder !== folder);
        state.wiki[folder] = starterWiki(folder);
        emit("workspace.changed", {});
        return structuredClone(project);
      },
    },
    agents: {
      demo: true,
      instances: () =>
        reply(
          state.instances.filter((i) =>
            state.deployments.some((d) => d.id === i.deploymentId && d.status === "active"),
          ),
        ),
      deployments: () => reply(state.deployments.filter((d) => d.status === "active")),
      deploy: async (team, project, goal) => {
        const t = find(state.teams, (x) => x.id === team, "team");
        const id = `${team}@${project}`;
        if (state.deployments.some((d) => d.id === id && d.status === "active")) {
          throw new Error("This team is already deployed on this project");
        }
        const dep: Deployment = {
          id,
          teamId: team,
          projectId: project,
          goal,
          status: "active",
          createdAt: iso(),
          stoppedAt: zero,
        };
        state.deployments = [...state.deployments.filter((d) => d.id !== id), dep];
        emit("deployment.created", dep);
        for (const agent of Object.keys(t.config.agents)) {
          const inst: Instance = {
            id: `${agent}@${project}`,
            deploymentId: id,
            agentId: agent,
            status: "starting",
            task: "",
            sandboxPath: `${state.workspace.path}/${project}/.sandboxes/${agent}`,
            harness: t.config.agents[agent].harness,
            startedAt: iso(),
            lastOutputAt: zero,
            restarts: 0,
          };
          state.instances = [...state.instances.filter((i) => i.id !== inst.id), inst];
          emit("instance.status", { id: inst.id, status: "starting" });
          setTimeout(() => setStatus(inst, agent === t.config.lead ? "running" : "idle"), 1500);
        }
        return structuredClone(dep);
      },
      stop: async (deployment) => {
        const dep = find(state.deployments, (d) => d.id === deployment, "deployment");
        dep.status = "stopped";
        dep.stoppedAt = iso();
        for (const i of state.instances.filter((x) => x.deploymentId === deployment)) {
          setStatus(i, "stopped");
        }
        emit("deployment.stopped", dep);
      },
      openTerminal: (id, h) => (options.terminal ?? noticeTerminal)(id, h),
    },
    harnesses: {
      demo: true,
      list: () => reply(state.harnesses),
      connect: async (harness) => `connect:${harness}`,
    },
    requests: {
      demo: true,
      list: () => reply(state.requests.filter((r) => r.status === "open")),
      resolve: async (id, answer) => {
        const r = find(state.requests, (x) => x.id === id, "request");
        r.status = "resolved";
        r.answer = answer;
        r.resolvedAt = iso();
        emit("request.resolved", r);
      },
    },
    messages: {
      demo: true,
      list: (deployment) => reply(state.messages.filter((m) => m.deploymentId === deployment)),
    },
    pipeline: {
      demo: true,
      list: (project) =>
        reply({
          workItems: state.workItems.filter((w) =>
            state.deployments.some((d) => d.id === w.deploymentId && d.projectId === project),
          ),
          mergeRequests: state.mergeRequests.filter((m) => m.projectId === project),
        }),
      merge: async (id) => {
        const mr = mergeRequest(state, id);
        mr.state = "merged";
        mr.mergedAt = iso();
        emit("pipeline.changed", mr);
      },
      createPr: async (id) => {
        const mr = mergeRequest(state, id);
        mr.forgePrNumber = 100 + state.mergeRequests.indexOf(mr);
        mr.forgePrUrl = `https://forge.example/pulls/${mr.forgePrNumber}`;
        emit("pipeline.changed", mr);
      },
      reject: async (id, reason) => {
        const mr = mergeRequest(state, id);
        mr.state = "rejected";
        mr.note = reason;
        mr.rejectedAt = iso();
        emit("pipeline.changed", mr);
      },
    },
    wiki: {
      demo: true,
      tree: async (project) => reply(wiki(state, project).tree),
      entry: async (project, path) => {
        const w = wiki(state, project);
        const e = w.entries[path];
        if (!e) throw new NotFound("entry");
        markSeen(w.tree, path); // what the reader shows counts as seen
        return reply(e);
      },
      boards: async (project) => reply(wiki(state, project).boards),
      setCheckbox: async (project, path, line, done, v) => {
        const e = wiki(state, project).entries[path];
        if (!e) throw new NotFound("entry");
        if (e.version !== v) throw new Conflict(path);
        find(e.checkboxes, (c) => c.line === line, "checkbox").done = done;
        e.version = version();
        emit("wiki.changed", { project, paths: [path], versions: [e.version] });
      },
      moveCard: async (project, boardId, path, column, lane, v) => {
        const board = find(wiki(state, project).boards, (b) => b.id === boardId, "board");
        const card = find(board.cards, (c) => c.path === path, "card");
        if (card.version !== v) throw new Conflict(path);
        card.column = column;
        if (board.lanes.length > 0) card.lane = lane;
        card.version = version();
        emit("wiki.changed", { project, paths: [path], versions: [card.version] });
      },
    },
    events: {
      demo: true,
      subscribe: (h) => {
        handlers.add(h);
        if (options.tick !== 0 && !timer) timer = setInterval(simulate, options.tick ?? 20000);
        return () => {
          handlers.delete(h);
          if (handlers.size === 0 && timer) {
            clearInterval(timer);
            timer = undefined;
          }
        };
      },
    },
  };
}

const zero = "0001-01-01T00:00:00Z";

function mergeRequest(state: Seed, id: string): MergeRequest {
  const mr = state.mergeRequests.find((m) => m.id === id);
  if (!mr) throw new NotFound("merge request");
  return mr;
}

function wiki(state: Seed, project: string) {
  const w = state.wiki[project];
  if (!w) throw new NotFound("wiki");
  return w;
}

/** A demo agent's terminal when no real one is wired: a notice, no session. */
function noticeTerminal(id: string, h: PtyHandlers): PtyConnection {
  const text = `\x1b[2m${id} is demo data: its terminal is not a real session.\x1b[0m\r\n`;
  setTimeout(() => h.output(new TextEncoder().encode(text)), 0);
  return { send: () => {}, resize: () => {}, close: () => {} };
}

/** One simulated moment of the team at work, in turn: a status change, a
 * team message, a new request, a hand-off. */
function simulateTeam(
  state: Seed,
  setStatus: (i: Instance, s: string) => void,
  emit: (type: string, data: unknown) => void,
  iso: () => string,
  step: number,
) {
  const live = state.instances.filter((i) =>
    state.deployments.some((d) => d.id === i.deploymentId && d.status === "active"),
  );
  if (live.length === 0) return;
  const inst = live[step % live.length];
  switch (step % 4) {
    case 0:
      setStatus(inst, inst.status === "running" ? "idle" : "running");
      break;
    case 1: {
      const to = live.find((i) => i.deploymentId === inst.deploymentId && i.id !== inst.id);
      if (!to) break;
      const m: TeamMessage = {
        id: state.messages.length + 1,
        deploymentId: inst.deploymentId,
        fromInstance: inst.id,
        toInstance: to.id,
        kind: "ask",
        requestRef: `req-${state.messages.length + 1}`,
        body: "Can you check the edge cases in the new handler?",
        artifactRef: "",
        createdAt: iso(),
        deliveredAt: iso(),
      };
      state.messages.push(m);
      emit("team.message", m);
      break;
    }
    case 2: {
      const project = inst.id.split("@")[1];
      const r: Request = {
        id: `demo-${state.requests.length + 1}`,
        kind: "question",
        projectId: project,
        deploymentId: inst.deploymentId,
        instanceId: inst.id,
        summary: "Should the retry budget be per merchant or global?",
        options: ["Per merchant", "Global"],
        source: "agent",
        status: "open",
        answer: "",
        createdAt: iso(),
        resolvedAt: "0001-01-01T00:00:00Z",
        mergeRequestId: "",
        links: [],
      };
      state.requests.push(r);
      setStatus(inst, "awaiting");
      emit("request.created", r);
      break;
    }
    case 3:
      setStatus(inst, "done");
      break;
  }
}

/** A new project's defaults (wiki/product/features/add-project.md#effect). */
function newProject(folder: string, repos: string[]): Seed["projects"][number] {
  const source =
    `project = "${folder}"\n` +
    `repos = [${repos.map((r) => `"${r}"`).join(", ")}]\n` +
    `main = "main"\nmerge_to_main = "ceo"\nchecks = ""\n` +
    `# always_ask_ceo = ["new dependency", "touches shared staging"]\n\n` +
    `[forge]\nkind = "none"\n`;
  return {
    id: folder,
    config: {
      project: folder,
      description: "",
      repos,
      main: "main",
      checks: "",
      mergeToMain: "ceo",
      alwaysAskCeo: [],
      forge: { kind: "none", url: "", projectRepo: "" },
    },
    repos,
    version: Math.random().toString(16).slice(2, 10),
    problems: [],
    source,
  };
}

/** The starter wiki: an index and an empty Backlog board. */
function starterWiki(folder: string): Seed["wiki"][string] {
  return {
    tree: [
      {
        path: "index.md",
        title: folder,
        folder: false,
        hasIndex: false,
        changed: false,
        children: [],
      },
    ],
    entries: {
      "index.md": {
        path: "index.md",
        title: folder,
        version: "v1",
        frontmatter: {},
        html: `<h1>${folder}</h1>\n<p>Deploy a team from this project's Settings tab.</p>\n`,
        headings: [],
        checkboxes: [],
        backlinks: [],
      },
    },
    boards: [
      {
        id: "backlog",
        name: "Backlog",
        columns: ["todo", "in-progress", "done"],
        laneField: "",
        lanes: [],
        cards: [],
        declaration: `[[tool.wikiview.board]]\nid = "backlog"\npath = "/backlog"\nname = "Backlog"\n`,
        problems: [],
      },
    ],
  };
}

function markSeen(nodes: Seed["wiki"][string]["tree"], path: string) {
  for (const n of nodes) {
    if (n.path === path) n.changed = false;
    markSeen(n.children, path);
  }
}
