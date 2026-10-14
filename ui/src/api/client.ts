import { createContext, useContext } from "react";
import { openPty, type PtyConnection, type PtyHandlers } from "./pty";

// Everything the UI asks of the server goes through this interface. Screens
// never call fetch or WebSocket themselves, so the transport can change and
// tests can hand components an in-memory client
// (wiki/product/architecture/overview.md#in-the-ui).

export type Workspace = { path: string };
export type Instance = { id: string };

export interface ForemanClient {
  workspace(): Promise<Workspace>;
  instances(): Promise<Instance[]>;
  openTerminal(instance: string, handlers: PtyHandlers): PtyConnection;
}

/** The client that talks to the Foreman server the page was loaded from. */
export function httpClient(origin: string = window.location.origin): ForemanClient {
  const getJSON = async <T>(path: string): Promise<T> => {
    const r = await fetch(origin + path);
    if (!r.ok) throw new Error(`${path}: HTTP ${r.status}`);
    return r.json() as Promise<T>;
  };
  const wsOrigin = origin.replace(/^http/, "ws");
  return {
    workspace: () => getJSON("/api/workspace"),
    instances: () => getJSON("/api/instances"),
    openTerminal: (instance, handlers) =>
      openPty(`${wsOrigin}/ws/pty/${encodeURIComponent(instance)}`, handlers),
  };
}

export const ClientContext = createContext<ForemanClient | null>(null);

export function useClient(): ForemanClient {
  const client = useContext(ClientContext);
  if (!client) throw new Error("useClient outside a ClientContext provider");
  return client;
}
