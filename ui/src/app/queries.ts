import { type QueryClient, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import {
  type Board,
  type Draft,
  type ForemanClient,
  type Request,
  type Settings,
  type Soul,
  type Team,
  useClient,
} from "../api/client";
import type { Event } from "../api/types/events";
import { useToast } from "../components/Toasts";
import { type SerialSaver, serialSaver } from "./serialSave";

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
  setup: ["setup"],
  messages: ["messages"],
  actions: ["actions"],
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

export function useHarnesses() {
  const c = useClient();
  return useQuery({ queryKey: keys.harnesses, queryFn: () => c.harnesses.list() });
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
      // A team change also changes its members' prompts.
      if (object === "team") return [keys.teams, ["prompt"], keys.setup];
      if (object === "settings") return [keys.settings];
      return [keys.projects, keys.setup];
    }
    case "workspace.changed":
      return [keys.projects, keys.setup];
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
    case "team.action":
      return [keys.actions];
    case "wiki.changed":
      return [["wiki", String((e.data as { project?: string }).project ?? "")]];
    case "pipeline.changed":
      return [["pipeline"]];
    case "cos.changed":
      return [keys.chiefOfStaff];
    case "setup.changed":
      return [keys.setup, keys.maintenance];
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
          // A draft's countdown is pushed, not fetched: the event is the state.
          if (e.type === "instance.draft") {
            const d = e.data as Draft;
            queries.setQueryData<Drafts>(draftsKey, (all = {}) => {
              const { [d.id]: _, ...rest } = all;
              return d.waiting > 0 ? { ...rest, [d.id]: d } : rest;
            });
          }
          invalidate(queries, staleKeys(e));
          listener.current(e);
        },
        // Events sent while away aren't replayed: refetch everything shown.
        reconnected: () => {
          // Drafts are pushed each countdown step: start over from the next one.
          queries.setQueryData(draftsKey, {});
          queries.invalidateQueries();
        },
      }),
    [c, queries],
  );
}

/** Messages waiting on the CEO's draft, by instance, with their countdown.
 * Fed by instance.draft events only: nothing to fetch. */
type Drafts = Record<string, Draft>;
const draftsKey = ["drafts"] as const;

export function useDrafts(): Drafts {
  return useQuery<Drafts>({
    queryKey: draftsKey,
    queryFn: () => ({}),
    enabled: false,
    initialData: {},
  }).data;
}

/** The messages waiting on the CEO's draft in one agent's terminal. */
export function useDraft(instance: string): Draft | undefined {
  return useDrafts()[instance];
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

const wikiKey = (project: string) => ["wiki", project] as const;

/** An entry's title (a review names its wiki card). */
export function useEntryTitle(project: string, path: string) {
  const c = useClient();
  return useQuery({
    queryKey: [...wikiKey(project), "title", path],
    queryFn: () => c.wiki.entry(project, path),
    enabled: path !== "",
    select: (e) => e.title,
  });
}

/** Answers a request; the list refetches. A failure, and a resolved Chief of
 * Staff review, show as toasts (app-shell.md, Toasts). */
export function useAnswer() {
  const c = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  return useMutation({
    mutationFn: ({ request, answer }: { request: Request; answer: string }) =>
      c.requests.resolve(request.id, answer),
    onSuccess: (_r, { request, answer }) => {
      if (request.source !== "cos" || request.kind !== "review") return;
      const merged = answer === "Merge to main";
      toast({
        title: request.projectId ? "project.toml" : "Team file",
        body: merged ? "Config change merged to main." : "Config change rejected.",
        color: merged ? "var(--ok)" : "var(--tx4)",
      });
    },
    onError: (e: Error) =>
      toast({ title: "Answer not sent", body: e.message, color: "var(--err)" }),
    onSettled: () => queries.invalidateQueries({ queryKey: keys.requests }),
  });
}

export function useActions(deployment: string) {
  const c = useClient();
  return useQuery({
    queryKey: [...keys.actions, deployment],
    queryFn: () => c.messages.actions(deployment),
    enabled: deployment !== "",
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

/** The scaffold files the project repo lacks: under the wiki's key, so its files appearing refresh it. */
export function useMissing(project: string) {
  const c = useClient();
  return useQuery({
    queryKey: [...wikiKey(project), "missing"],
    queryFn: () => c.workspace.missing(project),
  });
}

/** Creates the missing scaffold files; the wiki refetches. */
export function useScaffold(project: string) {
  const c = useClient();
  const queries = useQueryClient();
  return useMutation({
    mutationFn: () => c.workspace.scaffold(project),
    onSettled: () => queries.invalidateQueries({ queryKey: wikiKey(project) }),
  });
}

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
                  // An empty lane leaves the card's lane as it is, as the server does.
                  card.path === v.path
                    ? { ...card, column: v.column, lane: v.lane || card.lane }
                    : card,
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
export function useSaveSoul(team: Team, agent: string) {
  const c = useClient();
  const queries = useQueryClient();
  const saver = useRef(
    serialSaver<Soul>(async (soul, version) => {
      const saved = await c.workspace.saveSoul(team.id, agent, soul, version);
      return { value: saved.config.agents[agent], version: saved.version };
    }),
  ).current;
  // biome-ignore lint/correctness/useExhaustiveDependencies: what the server serves, by version
  useEffect(() => {
    saver.seen({ value: team.config.agents[agent], version: team.version });
  }, [team.version]);
  return {
    /** Saves the soul; quick saves in a row queue up. */
    save: (soul: Soul) =>
      saver.change(() => soul).finally(() => queries.invalidateQueries({ queryKey: keys.teams })),
    /** The version of the last save made here: a newer one came from elsewhere. */
    lastSaved: saver.lastSaved,
    /** Takes a fresh read now (Keep mine, after a conflict). */
    seen: (fresh: Team) =>
      saver.seen({ value: fresh.config.agents[agent], version: fresh.version }),
  };
}

/** An instance's session prompt, or the agent's template when not deployed. */
export function usePrompt(team: string, agent: string, instance?: string) {
  const c = useClient();
  return useQuery({
    queryKey: ["prompt", team, agent, instance ?? ""],
    queryFn: () => (instance ? c.agents.prompt(instance) : c.workspace.promptTemplate(team, agent)),
  });
}

// One queue for settings.toml across the app: every Settings section saves
// through it, so quick changes in different sections don't refuse each other.
const settingsSavers = new WeakMap<ForemanClient, SerialSaver<Settings>>();

/** Saves a change to the settings (serialSave.ts); they refetch after each save. */
export function useSaveSettings() {
  const c = useClient();
  const queries = useQueryClient();
  const file = useSettings();
  let saver = settingsSavers.get(c);
  if (!saver) {
    saver = serialSaver<Settings>(async (s, version) => {
      const saved = await c.workspace.saveSettings(s, version);
      return { value: saved.settings, version: saved.version };
    });
    settingsSavers.set(c, saver);
  }
  const current = saver;
  // biome-ignore lint/correctness/useExhaustiveDependencies: what the server serves, by version
  useEffect(() => {
    if (file.data) current.seen({ value: file.data.settings, version: file.data.version });
  }, [file.data?.version]);
  return (change: (s: Settings) => Settings) =>
    current.change(change).finally(() => queries.invalidateQueries({ queryKey: keys.settings }));
}

/** One merge request, wherever it is (under ["pipeline"], so pipeline events refresh it). */
export function useMergeRequest(id: string) {
  const c = useClient();
  return useQuery({
    queryKey: ["pipeline", "mr", id],
    queryFn: () => c.pipeline.mergeRequest(id),
    enabled: id !== "",
  });
}

/** What a merge request changes. */
export function useDiff(mergeRequest: string) {
  const c = useClient();
  return useQuery({
    queryKey: ["pipeline", "diff", mergeRequest],
    queryFn: () => c.pipeline.diff(mergeRequest),
    enabled: mergeRequest !== "",
  });
}

/** The engine running now and its images (Settings). */
export function useMaintenance() {
  const c = useClient();
  return useQuery({
    queryKey: keys.maintenance,
    queryFn: () => c.maintenance.status(),
    // A build reports no events: follow it while it runs.
    refetchInterval: (q) =>
      q.state.data?.images.some((i) => i.state === "building") ? 2000 : false,
  });
}

/** The setup checklist (Home). Polled while anything is left: installing
 * Podman or logging in happens outside Foreman, and a machine's creation or
 * an image build reports its output only when asked. */
export function useSetup() {
  const c = useClient();
  return useQuery({
    queryKey: keys.setup,
    queryFn: () => c.setup.steps(),
    refetchInterval: (q) => {
      const steps = q.state.data ?? [];
      if (steps.some((s) => s.state === "working")) return 1000;
      return steps.some((s) => s.state !== "done") ? 3000 : false;
    },
  });
}
