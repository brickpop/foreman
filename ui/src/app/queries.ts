import {
  type QueryClient,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import {
  type Board,
  type BranchPick,
  type Doing,
  type Draft,
  type ForemanClient,
  type Instance,
  type OpenTarget,
  type Preferences,
  type Proposal,
  type Request,
  type Settings,
  type Soul,
  type Team,
  useClient,
} from "../api/client";
import type { Event } from "../api/types/events";
import { useToast } from "../components/Toasts";
import { projectName } from "./model";
import { deploymentOn } from "./project";
import { type SerialSaver, serialSaver } from "./serialSave";
import { withDoing } from "./sessions";
import { applyAccent, applyTheme, asAccent, asTheme } from "./theme";

// Server state, through TanStack Query: one hook per query, and server events
// invalidating what they touch, so every screen has one data path
// (wiki/product/architecture/tech-stack.md).

export const keys = {
  home: ["home"],
  desktop: ["desktop"],
  projects: ["projects"],
  teams: ["teams"],
  settings: ["settings"],
  instances: ["instances"],
  deployments: ["deployments"],
  requests: ["requests"],
  harnesses: ["harnesses"],
  profiles: ["profiles"],
  preferences: ["preferences"],
  maintenance: ["maintenance"],
  setup: ["setup"],
  messages: ["messages"],
  activity: ["activity"],
  repos: ["repos"],
} as const;

export function useHomeFolder() {
  const c = useClient();
  return useQuery({ queryKey: keys.home, queryFn: () => c.home.folder() });
}

/** Whether Foreman's machine can open folders on its desktop: fixed while it
 * runs, so asked once. */
export function useCanOpen() {
  const c = useClient();
  return (
    useQuery({
      queryKey: keys.desktop,
      queryFn: () => c.desktop.canOpen(),
      staleTime: Number.POSITIVE_INFINITY,
    }).data?.canOpen ?? false
  );
}

/** Opens a folder on the desktop. A failure shows as a toast. */
export function useOpenFolder() {
  const c = useClient();
  const toast = useToast();
  return useMutation({
    mutationFn: (target: OpenTarget) => c.desktop.open(target),
    onError: (e: Error) => toast({ title: "Not opened", body: e.message, color: "var(--err)" }),
  });
}

/** A repo folder's branches, while one is picked to add (or to change its main). */
export function useBranches(path: string | undefined) {
  const c = useClient();
  return useQuery({
    queryKey: ["branches", path],
    queryFn: () => c.home.branches(path ?? ""),
    enabled: !!path,
  });
}

export function useProjects() {
  const c = useClient();
  return useQuery({ queryKey: keys.projects, queryFn: () => c.home.projects() });
}

/** A project's shown name by its id (projectName), from the projects read. */
export function useProjectName(): (id: string) => string {
  const projects = useProjects().data;
  return (id) => projectName(projects, id);
}

/** Where a project's repo is on disk, for showing paths: <workspace>/project
 * (in the home, projects/<id>, for one made before workspaces). */
export function useProjectRepo(project: string): string {
  return useProjects().data?.find((p) => p.id === project)?.path ?? "";
}

export function useTeams() {
  const c = useClient();
  return useQuery({ queryKey: keys.teams, queryFn: () => c.home.teams() });
}

export function useSettings() {
  const c = useClient();
  return useQuery({ queryKey: keys.settings, queryFn: () => c.home.settings() });
}

export function useHarnesses() {
  const c = useClient();
  return useQuery({ queryKey: keys.harnesses, queryFn: () => c.harnesses.list() });
}

/** The CEO's profiles, with their state. Kept outside the global repo, so no
 * event says they changed: what changes them here refetches them. */
export function useProfiles() {
  const c = useClient();
  return useQuery({ queryKey: keys.profiles, queryFn: () => c.profiles.list() });
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
      // Each is part of the agents' prompts: which runs are behind changes too.
      if (object === "team") return [keys.teams, ["prompt"], keys.setup, keys.instances];
      // The settings choose the engine, which the checklist checks, and hold
      // the CEO's instructions, part of every prompt, and the projects folder.
      if (object === "settings")
        return [keys.settings, keys.setup, ["prompt"], ["workspace"], keys.instances];
      return [keys.projects, keys.setup, keys.instances];
    }
    case "projects.changed":
      return [keys.projects, keys.setup, keys.repos, keys.instances];
    case "instance.status":
      return [keys.instances];
    case "deployment.created":
    case "deployment.stopped":
      return [keys.instances, keys.deployments];
    case "request.created":
    case "request.resolved":
      // A review's request opening or closing is its status changing.
      return [keys.requests, reviewsKey];
    case "review.withdrawn":
      // Its request resolves (when it had one), and its hand-offs move.
      return [keys.requests, reviewsKey, ["pipeline"]];
    case "team.message":
      return [keys.messages, keys.activity];
    case "team.action":
      return [keys.activity, keys.repos];
    case "wiki.changed":
      return [["wiki", String((e.data as { project?: string }).project ?? "")]];
    case "pipeline.changed":
      return [["pipeline"], keys.repos];
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
  // The unsubscribe is the cleanup.
  useEffect(() => {
    return c.events.subscribe({
      event: (e) => {
        // A draft's countdown is pushed, not fetched: the event is the state.
        if (e.type === "instance.draft") {
          const d = e.data as Draft;
          queries.setQueryData<Drafts>(draftsKey, (all = {}) => {
            const { [d.id]: _, ...rest } = all;
            return d.waiting > 0 ? { ...rest, [d.id]: d } : rest;
          });
        }
        // So is the Chief of Staff's proposal: null once it is gone.
        if (e.type === "cos.proposal") {
          queries.setQueryData(cosProposalKey, (e.data as Proposal | null) ?? null);
        }
        // So is an agent's activity line, at every step of its work.
        if (e.type === "instance.doing") {
          const change = e.data as { id: string; doing: Doing };
          queries.setQueryData<Instance[]>(keys.instances, (list) => withDoing(list, change));
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
    });
  }, [c, queries]);
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

/** The team the Chief of Staff proposes, on screen until it is created or
 * replaced, or the session ends. Fed by cos.proposal events, and by the
 * session's start (one already on screen). */
export const cosProposalKey = ["cos-proposal"] as const;

export function useCosProposal(): Proposal | null {
  return useQuery<Proposal | null>({
    queryKey: cosProposalKey,
    queryFn: () => null,
    enabled: false,
    initialData: null,
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

/** An entry's title (a merge request and a work item name their wiki card). */
export function useEntryTitle(project: string, path: string) {
  const c = useClient();
  return useQuery({
    queryKey: [...wikiKey(project), "title", path],
    queryFn: () => c.wiki.entry(project, path),
    enabled: path !== "",
    select: (e) => e.title,
  });
}

/** Answers a request; the list refetches. A failure shows as a toast (app-shell.md, Toasts). */
export function useAnswer() {
  const c = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  return useMutation({
    mutationFn: ({ request, answer }: { request: Request; answer: string }) =>
      // A blocked card's Restart restarts its agent, and the agent's new run
      // resolves the card (wiki/product/features/requests.md).
      request.kind === "blocked" && answer === "Restart" && request.instanceId
        ? c.agents.restart(request.instanceId)
        : c.requests.resolve(request.id, answer),
    onError: (e: Error) =>
      toast({ title: "Answer not sent", body: e.message, color: "var(--err)" }),
    onSettled: () => queries.invalidateQueries({ queryKey: keys.requests }),
  });
}

/** Dismisses a request, unanswered; the list refetches. A failure shows as a toast. */
export function useDismiss() {
  const c = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  return useMutation({
    mutationFn: (request: Request) => c.requests.dismiss(request.id),
    onError: (e: Error) => toast({ title: "Not dismissed", body: e.message, color: "var(--err)" }),
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

/** The scaffold files the project repo lacks: under the wiki's key, so its files appearing refresh it. */
export function useMissing(project: string) {
  const c = useClient();
  return useQuery({
    queryKey: [...wikiKey(project), "missing"],
    queryFn: () => c.home.missing(project),
  });
}

/** Creates the missing scaffold files; the wiki refetches. */
export function useScaffold(project: string) {
  const c = useClient();
  const queries = useQueryClient();
  return useMutation({
    mutationFn: () => c.home.scaffold(project),
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
      const saved = await c.home.saveSoul(team.id, agent, soul, version);
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
    queryFn: () => (instance ? c.agents.prompt(instance) : c.home.promptTemplate(team, agent)),
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
      const saved = await c.home.saveSettings(s, version);
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

/** The reviews (under ["pipeline"], so merge requests' events refresh them). */
const reviewsKey = ["pipeline", "reviews"] as const;

/** A project's open reviews and its last decided ones, newest first. ("project" keeps the key
 * apart from a review's, which is its id.) */
export function useReviews(project: string) {
  const c = useClient();
  return useQuery({
    queryKey: [...reviewsKey, "project", project],
    queryFn: () => c.reviews.list(project),
  });
}

/** A review's title, as its lead gave it; undefined until the reviews load, and for a review
 * decided before the project's last ones done. */
export function useReviewTitle(project: string, id: string) {
  return useReviews(project).data?.find((r) => r.id === id)?.title;
}

/** One review, with its comments where they are now. */
export function useReview(id: string) {
  const c = useClient();
  return useQuery({
    queryKey: [...reviewsKey, id],
    queryFn: () => c.reviews.get(id),
    enabled: id !== "",
  });
}

/** A review's change, repo by repo: against its target, or since the CEO's last look. */
export function useReviewDiff(id: string, since: boolean) {
  const c = useClient();
  return useQuery({
    queryKey: [...reviewsKey, id, "diff", since],
    queryFn: () => c.reviews.diff(id, since),
    enabled: id !== "",
  });
}

/** A change to a review's draft comments: only that review is refetched after it, whether it
 * worked or not. Its diff and the list of reviews don't change with a comment, and refetching
 * them made each comment wait on the slowest calls. The change is pending until that refetch is
 * in, so the comment shows as its box closes. */
export function useCommentChange<T>(
  id: string,
  change: (c: ForemanClient, input: T) => Promise<unknown>,
  onError: (e: Error) => void,
) {
  const c = useClient();
  const queries = useQueryClient();
  return useMutation({
    mutationFn: (input: T) => change(c, input),
    onError,
    onSettled: () => queries.invalidateQueries({ queryKey: [...reviewsKey, id], exact: true }),
  });
}

/** A decision on a review: the review, the pipeline and the requests are refetched after it,
 * whether it worked or not. */
export function useReviewChange<T, R = void>(
  change: (c: ForemanClient, input: T) => Promise<R>,
  onError: (e: Error) => void,
) {
  const c = useClient();
  const queries = useQueryClient();
  return useMutation({
    mutationFn: (input: T) => change(c, input),
    onError,
    onSettled: () => {
      queries.invalidateQueries({ queryKey: ["pipeline"] });
      queries.invalidateQueries({ queryKey: keys.requests });
    },
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

/** The setup checklist (Dashboard). Polled while anything is left: installing
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

/** The team's agents that would continue an earlier conversation on
 * project (Deploy's Start fresh). Under the deployments' key: a stop or a
 * deploy changes it. */
export function useContinuing(team?: string, project?: string) {
  const c = useClient();
  return useQuery({
    queryKey: [...keys.deployments, "continuing", team, project],
    queryFn: () => c.agents.continuing(team ?? "", project ?? ""),
    enabled: !!team && !!project,
  });
}

/** The stopped deployments: a project's older sessions. Under the
 * deployments' key, so a stop refreshes them. */
export function useStoppedDeployments() {
  const c = useClient();
  return useQuery({
    queryKey: [...keys.deployments, "stopped"],
    queryFn: () => c.agents.stoppedDeployments(),
  });
}

/** The project's deployment (deploymentOn): the running one, else the one
 * that stopped last. Its team is whose agents the project's branches and
 * cards name. */
export function useDeploymentOn(project: string) {
  const active = useDeployments().data ?? [];
  const stopped = useStoppedDeployments().data ?? [];
  return deploymentOn(project, active, stopped);
}

/** A deployment's team traffic a page at a time, newest first (a project's
 * Team section's Activity view). */
export function useActivity(deployment: string, enabled = true) {
  const c = useClient();
  return useInfiniteQuery({
    queryKey: [...keys.activity, deployment],
    queryFn: ({ pageParam }) => c.messages.activity(deployment, pageParam),
    initialPageParam: "",
    getNextPageParam: (last) => last.next || undefined,
    enabled,
  });
}

/** A project's repos and their branches (the Repos section). Under the
 * repos' key: a commit, a merge or a new repo refreshes everything shown. */
export function useRepos(project: string) {
  const c = useClient();
  return useQuery({ queryKey: [...keys.repos, project], queryFn: () => c.repos.list(project) });
}

/** Removes what agents no longer running left on the project (only what is
 * safe); its repos then show what is left. */
export function useCleanUp(project: string) {
  const c = useClient();
  const queries = useQueryClient();
  return useMutation({
    mutationFn: () => c.pipeline.cleanUp(project),
    onSettled: () => queries.invalidateQueries({ queryKey: keys.repos }),
  });
}

/** What the branch cleanup offers in the project's code repos, read while it is open. */
export function useCleanupBranches(project: string) {
  const c = useClient();
  return useQuery({
    queryKey: [...keys.repos, project, "branch-cleanup"],
    queryFn: () => c.pipeline.cleanupBranches(project),
  });
}

/** Deletes the CEO's pick of the project's branches; its repos then show
 * what is left. */
export function useDeleteBranches(project: string) {
  const c = useClient();
  const queries = useQueryClient();
  return useMutation({
    mutationFn: (branches: BranchPick[]) => c.pipeline.deleteBranches(project, branches),
    onSettled: () => queries.invalidateQueries({ queryKey: keys.repos }),
  });
}

export function useRepoTree(project: string, repo: string, ref: string, path: string) {
  const c = useClient();
  return useQuery({
    queryKey: [...keys.repos, project, repo, ref, "tree", path],
    queryFn: () => c.repos.tree(project, repo, ref, path),
  });
}

export function useRepoFile(project: string, repo: string, ref: string, path: string) {
  const c = useClient();
  return useQuery({
    queryKey: [...keys.repos, project, repo, ref, "file", path],
    queryFn: () => c.repos.file(project, repo, ref, path),
  });
}

/** A ref's history, or with all every branch's, a page at a time, newest first. */
export function useRepoLog(project: string, repo: string, ref: string, all = false) {
  const c = useClient();
  return useInfiniteQuery({
    queryKey: [...keys.repos, project, repo, all ? "" : ref, "log"],
    queryFn: ({ pageParam }) => c.repos.log(project, repo, ref, pageParam, all),
    initialPageParam: 0,
    getNextPageParam: (last) => last.next || undefined,
  });
}

/** The CEO's preferences, applied as they arrive: on load, and when this tab
 * comes back into focus after another tab changed them. */
export function usePreferences() {
  const c = useClient();
  const prefs = useQuery({ queryKey: keys.preferences, queryFn: () => c.preferences.get() });
  useEffect(() => {
    if (!prefs.data) return;
    applyTheme(asTheme(prefs.data.theme));
    applyAccent(asAccent(prefs.data.accent));
  }, [prefs.data]);
  return prefs;
}

/** Saves some of the preferences (the theme, the accent, the terminal font):
 * shown at once (usePreferences applies them, open terminals follow), and
 * put back if refused. */
export function useSavePreferences() {
  const c = useClient();
  const queries = useQueryClient();
  return useMutation({
    mutationFn: (p: Partial<Preferences>) => c.preferences.save(p),
    onMutate: (p) => {
      const before = queries.getQueryData<Preferences>(keys.preferences);
      if (before) queries.setQueryData(keys.preferences, { ...before, ...p });
      return before;
    },
    onError: (_e, _p, before) => queries.setQueryData(keys.preferences, before),
    onSettled: () => queries.invalidateQueries({ queryKey: keys.preferences }),
  });
}
