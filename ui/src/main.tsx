import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { ClientContext, type ForemanClient, httpClient } from "./api/client";
import { demoClient } from "./api/demo/demo";
import { seed } from "./api/demo/seed";
import { router } from "./app/router";
import { applyTheme, savedTheme } from "./app/theme";
import "./styles.css";

// The composition root: the only place that picks concrete implementations.
// One line per area: the real client, or the demo client where the screens
// aren't wired to the backend yet (wiki/backlog/epics/full-ui-demo.md). Until
// then every area is demo, so the demo world stays coherent; only the terminal
// is real: every demo agent shows the server's demo session (just dev).
const real = httpClient();
const demo = demoClient(seed, {
  terminal: (_id, handlers) => real.agents.openTerminal("demo", handlers),
});
const client: ForemanClient = {
  workspace: demo.workspace,
  agents: demo.agents,
  harnesses: demo.harnesses,
  maintenance: demo.maintenance,
  forge: demo.forge,
  requests: demo.requests,
  messages: demo.messages,
  pipeline: demo.pipeline,
  wiki: demo.wiki,
  chiefOfStaff: demo.chiefOfStaff,
  events: demo.events,
};

applyTheme(savedTheme());

const root = document.getElementById("root");
if (!root) throw new Error("index.html has no #root");
createRoot(root).render(
  <StrictMode>
    <ClientContext.Provider value={client}>
      <QueryClientProvider client={new QueryClient()}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ClientContext.Provider>
  </StrictMode>,
);
