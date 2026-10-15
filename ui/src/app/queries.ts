import { type QueryClient, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { type Board, type Soul, useClient } from "../api/client";
import type { Event } from "../api/types/events";

// Server state, through TanStack Query: one hook per query, and server events
// invalidating what they touch, so every screen has one data path
// (wiki/product/architecture/tech-stack.md).

export const keys = {
  workspace: ["workspace"],
  projects: ["projects"],
  teams: ["teams"],
  settings: ["settings"],
  instances: ["instances"],
  deployments: ["deployments"],
  requests: ["requests"],
  harnesses: ["harnesses"],
  forge: ["forge"],
  maintenance: ["maintenance"],
  messages: ["messages"],
  chiefOfStaff: ["chief-of-staff"],
} as const;

export function useWorkspace() {
  const c = useClient();
  return useQuery({ queryKey: keys.workspace, queryFn: () => c.workspace.workspace() });
}

export function useProjects() {
  const c = useClient();
  return useQuery({ queryKey: keys.projects, queryFn: () => c.workspace.projects() });
}

export function useTeams() {
  const c = useClient();
  return useQuery({ queryKey: keys.teams, queryFn: () => c.workspace.teams() });
}

export function useSettings() {
  const c = useClient();
  return useQuery({ queryKey: keys.settings, queryFn: () => c.workspace.settings() });
}

export function useInstances() {
  const c = useClient();
  return useQuery({ queryKey: keys.instances, queryFn: () => c.agents.instances() });
}

export function useRequests() {
  const c = useClient();
  return useQuery({ queryKey: keys.requests, queryFn: () => c.requests.list() });
}

/** Which cached queries an event makes stale. */
export function staleKeys(e: Event): readonly (readonly string[])[] {
  switch (e.type) {
    case "config.changed": {
      const object = (e.data as { object?: string }).object;
      if (object === "team") return [keys.teams];
      if (object === "settings") return [keys.settings];
      return [keys.projects];
    }
    case "workspace.changed":
      return [keys.projects];
    case "instance.status":
      return [keys.instances];
    case "deployment.created":
    case "deployment.stopped":
      return [keys.instances, keys.deployments];
    case "request.created":
    case "request.resolved":
      return [keys.requests];
    case "team.message":
      return [keys.messages];
    case "wiki.changed":
      return [["wiki", String((e.data as { project?: string }).project ?? "")]];
    case "pipeline.changed":
      return [["pipeline"]];
    case "cos.changed":
      return [keys.chiefOfStaff];
    default:
      return [];
  }
}

/** Follows the server's events for as long as the app runs; listen also
 * hears each one (notifications). */
export function useLiveEvents(listen: (e: Event) => void = () => {}) {
  const c = useClient();
  const queries = useQueryClient();
  const listener = useRef(listen);
  listener.current = listen;
  useEffect(
    () =>
      c.events.subscribe({
        event: (e) => {
          invalidate(queries, staleKeys(e));
          listener.current(e);
        },
        // Events sent while away aren't replayed: refetch everything shown.
        reconnected: () => queries.invalidateQueries(),
      }),
    [c, queries],
  );
}

function invalidate(queries: QueryClient, stale: readonly (readonly string[])[]) {
  for (const key of stale) queries.invalidateQueries({ queryKey: key });
}

export function usePipeline(project: string) {
  const c = useClient();
  return useQuery({
    queryKey: ["pipeline", project],
    queryFn: () => c.pipeline.list(project),
    enabled: project !== "",
  });
}

/** An entry's title (a review names its wiki card). */
export function useEntryTitle(project: string, path: string) {
  const c = useClient();
  return useQuery({
    queryKey: ["entry", project, path],
    queryFn: () => c.wiki.entry(project, path),
    enabled: path !== "",
    select: (e) => e.title,
  });
}

/** Answers a request; the list refetches. */
export function useAnswer() {
  const c = useClient();
  const queries = useQueryClient();
  return useMutation({
    mutationFn: ({ id, answer }: { id: string; answer: string }) => c.requests.resolve(id, answer),
    onSettled: () => queries.invalidateQueries({ queryKey: keys.requests }),
  });
}

export function useMessages(deployment: string) {
  const c = useClient();
  return useQuery({
    queryKey: [...keys.messages, deployment],
    queryFn: () => c.messages.list(deployment),
    enabled: deployment !== "",
  });
}

const wikiKey = (project: string) => ["wiki", project] as const;

export function useTree(project: string) {
  const c = useClient();
  return useQuery({ queryKey: [...wikiKey(project), "tree"], queryFn: () => c.wiki.tree(project) });
}

/** An entry; reading it marks it seen, so the tree's dots refresh. */
export function useEntry(project: string, path: string) {
  const c = useClient();
  const queries = useQueryClient();
  return useQuery({
    queryKey: [...wikiKey(project), "entry", path],
    queryFn: async () => {
      const e = await c.wiki.entry(project, path);
      queries.invalidateQueries({ queryKey: [...wikiKey(project), "tree"] });
      return e;
    },
    enabled: path !== "",
  });
}

export function useBoards(project: string) {
  const c = useClient();
  return useQuery({
    queryKey: [...wikiKey(project), "boards"],
    queryFn: () => c.wiki.boards(project),
  });
}

/** Ticks a checkbox; on a conflict the box reverts (the entry refetches) and onError says why. */
export function useTick(project: string, onError: (e: Error) => void) {
  const c = useClient();
  const queries = useQueryClient();
  return useMutation({
    mutationFn: (v: { path: string; line: number; done: boolean; version: string }) =>
      c.wiki.setCheckbox(project, v.path, v.line, v.done, v.version),
    onError,
    onSettled: () => queries.invalidateQueries({ queryKey: wikiKey(project) }),
  });
}

/** Moves a card: shown at once, put back if the write fails. */
export function useMoveCard(project: string, onError: (e: Error) => void) {
  const c = useClient();
  const queries = useQueryClient();
  const key = [...wikiKey(project), "boards"];
  return useMutation({
    mutationFn: (v: {
      board: string;
      path: string;
      column: string;
      lane: string;
      version: string;
    }) => c.wiki.moveCard(project, v.board, v.path, v.column, v.lane, v.version),
    onMutate: async (v) => {
      await queries.cancelQueries({ queryKey: key });
      const before = queries.getQueryData<Board[]>(key);
      queries.setQueryData<Board[]>(key, (boards) =>
        boards?.map((b) =>
          b.id !== v.board
            ? b
            : {
                ...b,
                cards: b.cards.map((card) =>
                  card.path === v.path ? { ...card, column: v.column, lane: v.lane } : card,
                ),
              },
        ),
      );
      return { before };
    },
    onError: (e, _v, context) => {
      queries.setQueryData(key, context?.before);
      onError(e);
    },
    onSettled: () => queries.invalidateQueries({ queryKey: wikiKey(project) }),
  });
}

export function useDeployments() {
  const c = useClient();
  return useQuery({ queryKey: keys.deployments, queryFn: () => c.agents.deployments() });
}

/** Saves a soul in its team file; the teams refetch. */
export function useSaveSoul(team: string) {
  const c = useClient();
  const queries = useQueryClient();
  return useMutation({
    mutationFn: (v: { agent: string; soul: Soul; version: string }) =>
      c.workspace.saveSoul(team, v.agent, v.soul, v.version),
    onSettled: () => queries.invalidateQueries({ queryKey: keys.teams }),
  });
}

/** An instance's session prompt, or the agent's template when not deployed. */
export function usePrompt(team: string, agent: string, instance?: string) {
  const c = useClient();
  return useQuery({
    queryKey: ["prompt", team, agent, instance ?? ""],
    queryFn: () => (instance ? c.agents.prompt(instance) : c.workspace.promptTemplate(team, agent)),
  });
}
