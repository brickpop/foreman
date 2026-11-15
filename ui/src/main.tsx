import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { ClientContext, httpClient } from "./api/client";
import { router } from "./app/router";
import { applyAccent, applyTheme, savedAccent, savedTheme } from "./app/theme";
import "./styles.css";

// The composition root: the only place that picks concrete implementations.
const client = httpClient();
const queries = new QueryClient();

applyTheme(savedTheme());
applyAccent(savedAccent());

const root = document.getElementById("root");
if (!root) throw new Error("index.html has no #root");
createRoot(root).render(
  <StrictMode>
    <ClientContext.Provider value={client}>
      <QueryClientProvider client={queries}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ClientContext.Provider>
  </StrictMode>,
);
