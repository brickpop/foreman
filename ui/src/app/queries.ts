import { type QueryClient, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { useClient } from "../api/client";
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
  requests: ["requests"],
  harnesses: ["harnesses"],
  messages: ["messages"],
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
    case "deployment.created":
    case "deployment.stopped":
      return [keys.instances];
    case "request.created":
    case "request.resolved":
      return [keys.requests];
    case "team.message":
      return [keys.messages];
    default:
      return [];
  }
}

/** Follows the server's events for as long as the app runs. */
export function useLiveEvents() {
  const c = useClient();
  const queries = useQueryClient();
  useEffect(
    () =>
      c.events.subscribe({
        event: (e) => invalidate(queries, staleKeys(e)),
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
