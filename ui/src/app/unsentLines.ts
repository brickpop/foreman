import { useSyncExternalStore } from "react";

// What the CEO typed into each agent's Line input and hasn't sent, by
// instance: it survives switching agents, until the page reloads (which asks
// first while any is kept). The one copy: the Line input shows it from here.

const unsent = new Map<string, string>();
const listeners = new Set<() => void>();

export function setUnsentLine(instance: string, text: string): void {
  if (text) unsent.set(instance, text);
  else unsent.delete(instance);
  for (const listener of listeners) listener();
}

export const unsentLine = (instance: string): string => unsent.get(instance) ?? "";

/** Whether any of these instances has a line a reload would lose (gone
 * agents' lines can't be sent anymore: they don't count). */
export const hasUnsentLine = (instances: string[]): boolean =>
  instances.some((id) => unsent.has(id));

const subscribe = (listener: () => void) => {
  listeners.add(listener);
  return () => listeners.delete(listener);
};

export function useUnsentLine(instance: string): string {
  return useSyncExternalStore(subscribe, () => unsentLine(instance));
}
